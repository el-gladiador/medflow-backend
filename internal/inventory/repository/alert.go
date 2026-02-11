package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// InventoryAlert represents an inventory alert
// Actual DB schema: item_id, batch_id, alert_type, severity, message, status, acknowledged_at, acknowledged_by, resolved_at, resolved_by
type InventoryAlert struct {
	ID             string     `db:"id" json:"id"`
	ItemID         string     `db:"item_id" json:"item_id"`
	BatchID        *string    `db:"batch_id" json:"batch_id,omitempty"`
	AlertType      string     `db:"alert_type" json:"alert_type"`
	Severity       string     `db:"severity" json:"severity"`
	Message        string     `db:"message" json:"message"`
	Status         string     `db:"status" json:"status"` // 'open', 'acknowledged', 'resolved'
	AcknowledgedAt *time.Time `db:"acknowledged_at" json:"acknowledged_at,omitempty"`
	AcknowledgedBy *string    `db:"acknowledged_by" json:"acknowledged_by,omitempty"`
	ResolvedAt     *time.Time `db:"resolved_at" json:"resolved_at,omitempty"`
	ResolvedBy     *string    `db:"resolved_by" json:"resolved_by,omitempty"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	// Computed fields for API compatibility (not in DB)
	ItemName        string     `db:"-" json:"item_name,omitempty"`
	BatchNumber     *string    `db:"-" json:"batch_number,omitempty"`
	ExpiryDate      *time.Time `db:"-" json:"expiry_date,omitempty"`
	DaysUntilExpiry *int       `db:"-" json:"days_until_expiry,omitempty"`
	CurrentStock    *int       `db:"-" json:"current_stock,omitempty"`
	MinStock        *int       `db:"-" json:"min_stock,omitempty"`
}

// AlertRepository handles alert persistence
type AlertRepository struct {
	db *database.DB
}

// NewAlertRepository creates a new alert repository
func NewAlertRepository(db *database.DB) *AlertRepository {
	return &AlertRepository{db: db}
}

// Create creates a new alert
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *AlertRepository) Create(ctx context.Context, alert *InventoryAlert) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if alert.ID == "" {
		alert.ID = uuid.New().String()
	}

	if alert.Status == "" {
		alert.Status = "open"
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO inventory_alerts (id, tenant_id, item_id, batch_id, alert_type, severity, message, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at
		`

		return r.db.QueryRowxContext(ctx, query,
			alert.ID, tenantID, alert.ItemID, alert.BatchID, alert.AlertType,
			alert.Severity, alert.Message, alert.Status,
		).Scan(&alert.CreatedAt)
	})
}

// GetByID gets an alert by ID
// TENANT-ISOLATED: Queries via RLS
func (r *AlertRepository) GetByID(ctx context.Context, id string) (*InventoryAlert, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var alert InventoryAlert
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, batch_id, alert_type, severity, message, status,
			       acknowledged_at, acknowledged_by, resolved_at, resolved_by, created_at
			FROM inventory_alerts WHERE id = $1
		`
		return r.db.GetContext(ctx, &alert, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("alert")
	}
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

// List lists alerts with filtering
// TENANT-ISOLATED: Returns only alerts via RLS
func (r *AlertRepository) List(ctx context.Context, acknowledged *bool, alertType string, page, perPage int) ([]*InventoryAlert, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var alerts []*InventoryAlert

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		args := []interface{}{}
		argIndex := 1

		countQuery := `SELECT COUNT(*) FROM inventory_alerts WHERE 1=1`
		query := `SELECT id, item_id, batch_id, alert_type, severity, message, status,
		          acknowledged_at, acknowledged_by, resolved_at, resolved_by, created_at
		          FROM inventory_alerts WHERE 1=1`

		if acknowledged != nil {
			if *acknowledged {
				countQuery += fmt.Sprintf(` AND status = $%d`, argIndex)
				query += fmt.Sprintf(` AND status = $%d`, argIndex)
				args = append(args, "acknowledged")
			} else {
				countQuery += fmt.Sprintf(` AND status = $%d`, argIndex)
				query += fmt.Sprintf(` AND status = $%d`, argIndex)
				args = append(args, "open")
			}
			argIndex++
		}

		if alertType != "" {
			countQuery += fmt.Sprintf(` AND alert_type = $%d`, argIndex)
			query += fmt.Sprintf(` AND alert_type = $%d`, argIndex)
			args = append(args, alertType)
			argIndex++
		}

		if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
			return err
		}

		query += ` ORDER BY CASE severity WHEN 'critical' THEN 0 ELSE 1 END, created_at DESC`

		offset := (page - 1) * perPage
		query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argIndex, argIndex+1)
		args = append(args, perPage, offset)

		return r.db.SelectContext(ctx, &alerts, query, args...)
	})

	if err != nil {
		return nil, 0, err
	}

	return alerts, total, nil
}

// Acknowledge acknowledges an alert
// TENANT-ISOLATED: Updates via RLS
func (r *AlertRepository) Acknowledge(ctx context.Context, id, userID string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE inventory_alerts
			SET status = 'acknowledged', acknowledged_by = $2, acknowledged_at = NOW()
			WHERE id = $1
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

// DeleteOld deletes old resolved alerts
// TENANT-ISOLATED: Deletes via RLS
func (r *AlertRepository) DeleteOld(ctx context.Context, olderThan time.Duration) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `DELETE FROM inventory_alerts WHERE status = 'resolved' AND resolved_at < $1`
		_, err := r.db.ExecContext(ctx, query, time.Now().Add(-olderThan))
		return err
	})
}

// GetUnacknowledgedCount gets count of open alerts
// TENANT-ISOLATED: Queries via RLS
func (r *AlertRepository) GetUnacknowledgedCount(ctx context.Context) (int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return 0, err
	}

	var count int64
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `SELECT COUNT(*) FROM inventory_alerts WHERE status = 'open'`
		return r.db.GetContext(ctx, &count, query)
	})

	if err != nil {
		return 0, err
	}
	return count, nil
}
