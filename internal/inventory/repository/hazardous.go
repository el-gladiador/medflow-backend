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

// HazardousSubstanceDetail represents hazardous substance data for an inventory item (GefStoffV §6)
type HazardousSubstanceDetail struct {
	ID                  string     `db:"id" json:"id"`
	ItemID              string     `db:"item_id" json:"item_id"`
	GHSPictogramCodes   *string    `db:"ghs_pictogram_codes" json:"ghs_pictogram_codes,omitempty"`
	HStatements         *string    `db:"h_statements" json:"h_statements,omitempty"`
	PStatements         *string    `db:"p_statements" json:"p_statements,omitempty"`
	SignalWord          *string    `db:"signal_word" json:"signal_word,omitempty"`
	UsageArea           *string    `db:"usage_area" json:"usage_area,omitempty"`
	StorageInstructions *string    `db:"storage_instructions" json:"storage_instructions,omitempty"`
	EmergencyProcedures *string    `db:"emergency_procedures" json:"emergency_procedures,omitempty"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt           *time.Time `db:"deleted_at" json:"-"`
	CreatedBy           *string    `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy           *string    `db:"updated_by" json:"updated_by,omitempty"`
}

// HazardousRepository handles hazardous substance detail persistence
type HazardousRepository struct {
	db *database.DB
}

// NewHazardousRepository creates a new hazardous repository
func NewHazardousRepository(db *database.DB) *HazardousRepository {
	return &HazardousRepository{db: db}
}

// GetByItemID gets hazardous details for an item
func (r *HazardousRepository) GetByItemID(ctx context.Context, itemID string) (*HazardousSubstanceDetail, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var detail HazardousSubstanceDetail

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, ghs_pictogram_codes, h_statements, p_statements, signal_word,
			       usage_area, storage_instructions, emergency_procedures,
			       created_at, updated_at, created_by, updated_by
			FROM hazardous_substance_details
			WHERE item_id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &detail, query, itemID)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("hazardous_details")
	}
	if err != nil {
		return nil, err
	}

	return &detail, nil
}

// Upsert creates or updates hazardous details for an item
func (r *HazardousRepository) Upsert(ctx context.Context, detail *HazardousSubstanceDetail) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if detail.ID == "" {
		detail.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO hazardous_substance_details (
				id, tenant_id, item_id, ghs_pictogram_codes, h_statements, p_statements,
				signal_word, usage_area, storage_instructions, emergency_procedures,
				created_by, updated_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (tenant_id, item_id) WHERE deleted_at IS NULL
			DO UPDATE SET
				ghs_pictogram_codes = EXCLUDED.ghs_pictogram_codes,
				h_statements = EXCLUDED.h_statements,
				p_statements = EXCLUDED.p_statements,
				signal_word = EXCLUDED.signal_word,
				usage_area = EXCLUDED.usage_area,
				storage_instructions = EXCLUDED.storage_instructions,
				emergency_procedures = EXCLUDED.emergency_procedures,
				updated_by = EXCLUDED.updated_by,
				updated_at = NOW()
			RETURNING id, created_at, updated_at
		`

		return r.db.QueryRowxContext(ctx, query,
			detail.ID, tenantID, detail.ItemID, detail.GHSPictogramCodes,
			detail.HStatements, detail.PStatements, detail.SignalWord,
			detail.UsageArea, detail.StorageInstructions, detail.EmergencyProcedures,
			detail.CreatedBy, detail.UpdatedBy,
		).Scan(&detail.ID, &detail.CreatedAt, &detail.UpdatedAt)
	})
}

// Delete soft-deletes hazardous details for an item
func (r *HazardousRepository) Delete(ctx context.Context, itemID string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE hazardous_substance_details SET deleted_at = NOW() WHERE item_id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, itemID)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("hazardous_details")
		}

		return nil
	})
}

// ListAll lists all hazardous substance details (for Gefahrstoffverzeichnis export)
func (r *HazardousRepository) ListAll(ctx context.Context) ([]*HazardousSubstanceDetail, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var details []*HazardousSubstanceDetail

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, ghs_pictogram_codes, h_statements, p_statements, signal_word,
			       usage_area, storage_instructions, emergency_procedures,
			       created_at, updated_at, created_by, updated_by
			FROM hazardous_substance_details
			WHERE deleted_at IS NULL
			ORDER BY created_at
		`
		return r.db.SelectContext(ctx, &details, query)
	})

	if err != nil {
		return nil, err
	}

	return details, nil
}
