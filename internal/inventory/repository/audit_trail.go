package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// AuditEntry represents a GoBD-compliant audit trail entry.
// Audit entries are append-only — they are never updated or deleted.
type AuditEntry struct {
	ID              string    `db:"id" json:"id"`
	EntityType      string    `db:"entity_type" json:"entity_type"`
	EntityID        string    `db:"entity_id" json:"entity_id"`
	Action          string    `db:"action" json:"action"`
	FieldChanges    *string   `db:"field_changes" json:"field_changes,omitempty"`
	Metadata        *string   `db:"metadata" json:"metadata,omitempty"`
	PerformedBy     *string   `db:"performed_by" json:"performed_by,omitempty"`
	PerformedByName *string   `db:"performed_by_name" json:"performed_by_name,omitempty"`
	IPAddress       *string   `db:"ip_address" json:"ip_address,omitempty"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
}

// AuditTrailRepository handles GoBD-compliant audit trail persistence.
// All operations are append-only: no UPDATE or DELETE is permitted.
type AuditTrailRepository struct {
	db *database.DB
}

// NewAuditTrailRepository creates a new audit trail repository
func NewAuditTrailRepository(db *database.DB) *AuditTrailRepository {
	return &AuditTrailRepository{db: db}
}

// Create creates a new audit trail entry (append-only, no update/delete)
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *AuditTrailRepository) Create(ctx context.Context, entry *AuditEntry) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO audit_trail (
				id, tenant_id, entity_type, entity_id, action, field_changes,
				metadata, performed_by, performed_by_name, ip_address
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at
		`

		return r.db.QueryRowxContext(ctx, query,
			entry.ID, tenantID, entry.EntityType, entry.EntityID, entry.Action,
			entry.FieldChanges, entry.Metadata, entry.PerformedBy,
			entry.PerformedByName, entry.IPAddress,
		).Scan(&entry.CreatedAt)
	})
}

// ListByEntity lists audit entries for a specific entity with pagination
// TENANT-ISOLATED: Returns only entries via RLS
func (r *AuditTrailRepository) ListByEntity(ctx context.Context, entityType, entityID string, page, perPage int) ([]*AuditEntry, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var entries []*AuditEntry

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		countQuery := `SELECT COUNT(*) FROM audit_trail WHERE entity_type = $1 AND entity_id = $2`
		if err := r.db.GetContext(ctx, &total, countQuery, entityType, entityID); err != nil {
			return err
		}

		offset := (page - 1) * perPage
		query := `
			SELECT id, entity_type, entity_id, action, field_changes, metadata,
			       performed_by, performed_by_name, ip_address, created_at
			FROM audit_trail
			WHERE entity_type = $1 AND entity_id = $2
			ORDER BY created_at DESC
			LIMIT $3 OFFSET $4
		`

		return r.db.SelectContext(ctx, &entries, query, entityType, entityID, perPage, offset)
	})

	if err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

// ListByTenant lists audit entries for the tenant with optional filters
// TENANT-ISOLATED: Returns only entries via RLS
func (r *AuditTrailRepository) ListByTenant(ctx context.Context, entityType string, from, to *time.Time, page, perPage int) ([]*AuditEntry, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var entries []*AuditEntry

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		args := []interface{}{}
		argIdx := 1

		countQuery := `SELECT COUNT(*) FROM audit_trail WHERE 1=1`
		query := `
			SELECT id, entity_type, entity_id, action, field_changes, metadata,
			       performed_by, performed_by_name, ip_address, created_at
			FROM audit_trail WHERE 1=1
		`

		if entityType != "" {
			countQuery += fmt.Sprintf(` AND entity_type = $%d`, argIdx)
			query += fmt.Sprintf(` AND entity_type = $%d`, argIdx)
			args = append(args, entityType)
			argIdx++
		}

		if from != nil {
			countQuery += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
			query += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
			args = append(args, *from)
			argIdx++
		}

		if to != nil {
			countQuery += fmt.Sprintf(` AND created_at <= $%d`, argIdx)
			query += fmt.Sprintf(` AND created_at <= $%d`, argIdx)
			args = append(args, *to)
			argIdx++
		}

		if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
			return err
		}

		query += ` ORDER BY created_at DESC`

		offset := (page - 1) * perPage
		query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
		args = append(args, perPage, offset)

		return r.db.SelectContext(ctx, &entries, query, args...)
	})

	if err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

// ExportGoBD exports all audit entries in a date range for GoBD compliance
// TENANT-ISOLATED: Returns only entries via RLS
func (r *AuditTrailRepository) ExportGoBD(ctx context.Context, from, to *time.Time) ([]*AuditEntry, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var entries []*AuditEntry

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		args := []interface{}{}
		argIdx := 1

		query := `
			SELECT id, entity_type, entity_id, action, field_changes, metadata,
			       performed_by, performed_by_name, ip_address, created_at
			FROM audit_trail WHERE 1=1
		`

		if from != nil {
			query += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
			args = append(args, *from)
			argIdx++
		}

		if to != nil {
			query += fmt.Sprintf(` AND created_at <= $%d`, argIdx)
			args = append(args, *to)
			argIdx++
		}

		query += ` ORDER BY created_at ASC`

		return r.db.SelectContext(ctx, &entries, query, args...)
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}
