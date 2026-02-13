package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// DeviceInspection represents a device inspection record (STK/MTK)
type DeviceInspection struct {
	ID              string     `db:"id" json:"id"`
	ItemID          string     `db:"item_id" json:"item_id"`
	InspectionType  string     `db:"inspection_type" json:"inspection_type"`
	InspectionDate  time.Time  `db:"inspection_date" json:"inspection_date"`
	NextDueDate     *time.Time `db:"next_due_date" json:"next_due_date,omitempty"`
	Result          string     `db:"result" json:"result"`
	PerformedBy     string     `db:"performed_by" json:"performed_by"`
	ReportReference *string    `db:"report_reference" json:"report_reference,omitempty"`
	Notes           *string    `db:"notes" json:"notes,omitempty"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt       *time.Time `db:"deleted_at" json:"-"`
	CreatedBy       *string    `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy       *string    `db:"updated_by" json:"updated_by,omitempty"`
}

// InspectionRepository handles device inspection persistence
type InspectionRepository struct {
	db *database.DB
}

// NewInspectionRepository creates a new inspection repository
func NewInspectionRepository(db *database.DB) *InspectionRepository {
	return &InspectionRepository{db: db}
}

// Create creates a new device inspection
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *InspectionRepository) Create(ctx context.Context, insp *DeviceInspection) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if insp.ID == "" {
		insp.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO device_inspections (
				id, tenant_id, item_id, inspection_type, inspection_date, next_due_date,
				result, performed_by, report_reference, notes, created_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			insp.ID, tenantID, insp.ItemID, insp.InspectionType, insp.InspectionDate,
			insp.NextDueDate, insp.Result, insp.PerformedBy, insp.ReportReference,
			insp.Notes, insp.CreatedBy,
		).Scan(&insp.CreatedAt, &insp.UpdatedAt)
	})
}

// GetByID gets an inspection by ID
// TENANT-ISOLATED: Queries via RLS
func (r *InspectionRepository) GetByID(ctx context.Context, id string) (*DeviceInspection, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var insp DeviceInspection
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, inspection_type, inspection_date, next_due_date, result,
			       performed_by, report_reference, notes, created_at, updated_at, created_by, updated_by
			FROM device_inspections WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &insp, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("inspection")
	}
	if err != nil {
		return nil, err
	}

	return &insp, nil
}

// ListByItem lists inspections for a device
// TENANT-ISOLATED: Returns only inspections via RLS
func (r *InspectionRepository) ListByItem(ctx context.Context, itemID string) ([]*DeviceInspection, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var inspections []*DeviceInspection
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, inspection_type, inspection_date, next_due_date, result,
			       performed_by, report_reference, notes, created_at, updated_at, created_by, updated_by
			FROM device_inspections WHERE item_id = $1 AND deleted_at IS NULL
			ORDER BY inspection_date DESC
		`
		return r.db.SelectContext(ctx, &inspections, query, itemID)
	})

	if err != nil {
		return nil, err
	}

	return inspections, nil
}

// Update updates an inspection
// TENANT-ISOLATED: Updates via RLS
func (r *InspectionRepository) Update(ctx context.Context, insp *DeviceInspection) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE device_inspections SET
				inspection_type = $2, inspection_date = $3, next_due_date = $4, result = $5,
				performed_by = $6, report_reference = $7, notes = $8, updated_by = $9,
				updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			insp.ID, insp.InspectionType, insp.InspectionDate, insp.NextDueDate,
			insp.Result, insp.PerformedBy, insp.ReportReference, insp.Notes, insp.UpdatedBy,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("inspection")
		}

		return nil
	})
}

// Delete soft-deletes an inspection
// TENANT-ISOLATED: Deletes via RLS
func (r *InspectionRepository) Delete(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE device_inspections SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("inspection")
		}

		return nil
	})
}

// GetOverdueInspections returns inspections with next_due_date in the past
// TENANT-ISOLATED: Returns only inspections via RLS
func (r *InspectionRepository) GetOverdueInspections(ctx context.Context) ([]*DeviceInspection, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var inspections []*DeviceInspection
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, inspection_type, inspection_date, next_due_date, result,
			       performed_by, report_reference, notes, created_at, updated_at, created_by, updated_by
			FROM device_inspections
			WHERE deleted_at IS NULL AND next_due_date IS NOT NULL AND next_due_date < NOW()
			ORDER BY next_due_date
		`
		return r.db.SelectContext(ctx, &inspections, query)
	})

	if err != nil {
		return nil, err
	}

	return inspections, nil
}
