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

// DeviceIncident represents a device incident record (Vorkommnis)
type DeviceIncident struct {
	ID               string     `db:"id" json:"id"`
	ItemID           string     `db:"item_id" json:"item_id"`
	IncidentDate     time.Time  `db:"incident_date" json:"incident_date"`
	IncidentType     string     `db:"incident_type" json:"incident_type"`
	Description      string     `db:"description" json:"description"`
	Consequences     *string    `db:"consequences" json:"consequences,omitempty"`
	CorrectiveAction *string    `db:"corrective_action" json:"corrective_action,omitempty"`
	ReportedTo       *string    `db:"reported_to" json:"reported_to,omitempty"`
	ReportDate       *time.Time `db:"report_date" json:"report_date,omitempty"`
	ReportReference  *string    `db:"report_reference" json:"report_reference,omitempty"`
	Notes            *string    `db:"notes" json:"notes,omitempty"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt        *time.Time `db:"deleted_at" json:"-"`
	CreatedBy        *string    `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy        *string    `db:"updated_by" json:"updated_by,omitempty"`
}

// IncidentRepository handles device incident persistence
type IncidentRepository struct {
	db *database.DB
}

// NewIncidentRepository creates a new incident repository
func NewIncidentRepository(db *database.DB) *IncidentRepository {
	return &IncidentRepository{db: db}
}

// Create creates a new device incident
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *IncidentRepository) Create(ctx context.Context, inc *DeviceIncident) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if inc.ID == "" {
		inc.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO device_incidents (
				id, tenant_id, item_id, incident_date, incident_type, description,
				consequences, corrective_action, reported_to, report_date,
				report_reference, notes, created_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			inc.ID, tenantID, inc.ItemID, inc.IncidentDate, inc.IncidentType,
			inc.Description, inc.Consequences, inc.CorrectiveAction,
			inc.ReportedTo, inc.ReportDate, inc.ReportReference, inc.Notes, inc.CreatedBy,
		).Scan(&inc.CreatedAt, &inc.UpdatedAt)
	})
}

// GetByID gets an incident by ID
// TENANT-ISOLATED: Queries via RLS
func (r *IncidentRepository) GetByID(ctx context.Context, id string) (*DeviceIncident, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var inc DeviceIncident
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, incident_date, incident_type, description, consequences,
			       corrective_action, reported_to, report_date, report_reference, notes,
			       created_at, updated_at, created_by, updated_by
			FROM device_incidents WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &inc, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("incident")
	}
	if err != nil {
		return nil, err
	}

	return &inc, nil
}

// ListByItem lists incidents for a device
// TENANT-ISOLATED: Returns only incidents via RLS
func (r *IncidentRepository) ListByItem(ctx context.Context, itemID string) ([]*DeviceIncident, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var incidents []*DeviceIncident
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, incident_date, incident_type, description, consequences,
			       corrective_action, reported_to, report_date, report_reference, notes,
			       created_at, updated_at, created_by, updated_by
			FROM device_incidents WHERE item_id = $1 AND deleted_at IS NULL
			ORDER BY incident_date DESC
		`
		return r.db.SelectContext(ctx, &incidents, query, itemID)
	})

	if err != nil {
		return nil, err
	}

	return incidents, nil
}

// Update updates an incident
// TENANT-ISOLATED: Updates via RLS
func (r *IncidentRepository) Update(ctx context.Context, inc *DeviceIncident) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE device_incidents SET
				incident_date = $2, incident_type = $3, description = $4, consequences = $5,
				corrective_action = $6, reported_to = $7, report_date = $8,
				report_reference = $9, notes = $10, updated_by = $11,
				updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			inc.ID, inc.IncidentDate, inc.IncidentType, inc.Description,
			inc.Consequences, inc.CorrectiveAction, inc.ReportedTo,
			inc.ReportDate, inc.ReportReference, inc.Notes, inc.UpdatedBy,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("incident")
		}

		return nil
	})
}

// Delete soft-deletes an incident
// TENANT-ISOLATED: Deletes via RLS
func (r *IncidentRepository) Delete(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE device_incidents SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("incident")
		}

		return nil
	})
}
