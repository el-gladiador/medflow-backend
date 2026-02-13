package repository

import (
	"context"

	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// ExistsByTypeAndEntity checks if an open or acknowledged alert already exists
// for the given alert type and entity (item + optional batch).
// Used for deduplication in alert scanners.
// TENANT-ISOLATED: Queries via RLS
func (r *AlertRepository) ExistsByTypeAndEntity(ctx context.Context, alertType, itemID string, batchID *string) (bool, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return false, err
	}

	var exists bool

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT EXISTS(
				SELECT 1 FROM inventory_alerts
				WHERE alert_type = $1
				AND item_id = $2
				AND ($3::TEXT IS NULL AND batch_id IS NULL OR batch_id = $3)
				AND status IN ('open', 'acknowledged')
			)
		`
		return r.db.GetContext(ctx, &exists, query, alertType, itemID, batchID)
	})

	if err != nil {
		return false, err
	}

	return exists, nil
}

// Resolve marks an alert as resolved
// TENANT-ISOLATED: Updates via RLS
func (r *AlertRepository) Resolve(ctx context.Context, id, userID string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE inventory_alerts
			SET status = 'resolved', resolved_by = $2, resolved_at = NOW()
			WHERE id = $1 AND status IN ('open', 'acknowledged')
		`

		result, err := r.db.ExecContext(ctx, query, id, userID)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("alert")
		}

		return nil
	})
}

// BulkResolve resolves all open/acknowledged alerts matching the given type and item
// TENANT-ISOLATED: Updates via RLS
func (r *AlertRepository) BulkResolve(ctx context.Context, alertType, itemID string, userID string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE inventory_alerts
			SET status = 'resolved', resolved_by = $3, resolved_at = NOW()
			WHERE alert_type = $1 AND item_id = $2 AND status IN ('open', 'acknowledged')
		`

		_, err := r.db.ExecContext(ctx, query, alertType, itemID, userID)
		return err
	})
}

// ListActive returns all active (non-resolved) alerts ordered by severity and creation time
// TENANT-ISOLATED: Returns only alerts via RLS
func (r *AlertRepository) ListActive(ctx context.Context) ([]*InventoryAlert, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var alerts []*InventoryAlert

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, batch_id, alert_type, severity, message, status,
			       acknowledged_at, acknowledged_by, resolved_at, resolved_by, created_at
			FROM inventory_alerts
			WHERE status IN ('open', 'acknowledged')
			ORDER BY CASE severity WHEN 'critical' THEN 0 ELSE 1 END, created_at DESC
		`
		return r.db.SelectContext(ctx, &alerts, query)
	})

	if err != nil {
		return nil, err
	}

	return alerts, nil
}
