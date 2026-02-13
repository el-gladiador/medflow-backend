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

// SterilizationBatch represents a sterilization batch record (KRINKO compliance)
type SterilizationBatch struct {
	ID                 string     `db:"id" json:"id"`
	BatchNumber        string     `db:"batch_number" json:"batch_number"`
	SterilizerID       *string    `db:"sterilizer_id" json:"sterilizer_id,omitempty"`
	SterilizerName     string     `db:"sterilizer_name" json:"sterilizer_name"`
	ProgramNumber      *string    `db:"program_number" json:"program_number,omitempty"`
	CycleDate          time.Time  `db:"cycle_date" json:"cycle_date"`
	TemperatureCelsius *float64   `db:"temperature_celsius" json:"temperature_celsius,omitempty"`
	PressureBar        *float64   `db:"pressure_bar" json:"pressure_bar,omitempty"`
	HoldTimeMinutes    *int       `db:"hold_time_minutes" json:"hold_time_minutes,omitempty"`
	BiResult           *string    `db:"bi_result" json:"bi_result,omitempty"`
	CiResult           *string    `db:"ci_result" json:"ci_result,omitempty"`
	BowieDickResult    *string    `db:"bowie_dick_result" json:"bowie_dick_result,omitempty"`
	OverallResult      string     `db:"overall_result" json:"overall_result"`
	ReleasedBy         *string    `db:"released_by" json:"released_by,omitempty"`
	ReleasedByName     *string    `db:"released_by_name" json:"released_by_name,omitempty"`
	ReleaseDate        *time.Time `db:"release_date" json:"release_date,omitempty"`
	CreatedAt          time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt          *time.Time `db:"deleted_at" json:"-"`
}

// ReprocessingCycle represents a reprocessing cycle for an instrument (KRINKO compliance)
type ReprocessingCycle struct {
	ID                   string     `db:"id" json:"id"`
	ItemID               string     `db:"item_id" json:"item_id"`
	SterilizationBatchID *string    `db:"sterilization_batch_id" json:"sterilization_batch_id,omitempty"`
	CycleNumber          int        `db:"cycle_number" json:"cycle_number"`
	CycleDate            time.Time  `db:"cycle_date" json:"cycle_date"`
	CleaningMethod       *string    `db:"cleaning_method" json:"cleaning_method,omitempty"`
	DisinfectionMethod   *string    `db:"disinfection_method" json:"disinfection_method,omitempty"`
	SterilizationMethod  *string    `db:"sterilization_method" json:"sterilization_method,omitempty"`
	BiIndicatorResult    *string    `db:"bi_indicator_result" json:"bi_indicator_result,omitempty"`
	CiIndicatorResult    *string    `db:"ci_indicator_result" json:"ci_indicator_result,omitempty"`
	ReleasedBy           *string    `db:"released_by" json:"released_by,omitempty"`
	ReleasedByName       *string    `db:"released_by_name" json:"released_by_name,omitempty"`
	ReleaseDate          *time.Time `db:"release_date" json:"release_date,omitempty"`
	CreatedAt            time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt            *time.Time `db:"deleted_at" json:"-"`
}

// ReprocessingRepository handles sterilization batch and reprocessing cycle persistence
type ReprocessingRepository struct {
	db *database.DB
}

// NewReprocessingRepository creates a new reprocessing repository
func NewReprocessingRepository(db *database.DB) *ReprocessingRepository {
	return &ReprocessingRepository{db: db}
}

// --- Sterilization Batches ---

// CreateBatch creates a new sterilization batch
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *ReprocessingRepository) CreateBatch(ctx context.Context, batch *SterilizationBatch) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if batch.ID == "" {
		batch.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO sterilization_batches (
				id, tenant_id, batch_number, sterilizer_id, sterilizer_name, program_number,
				cycle_date, temperature_celsius, pressure_bar, hold_time_minutes,
				bi_result, ci_result, bowie_dick_result, overall_result,
				released_by, released_by_name, release_date
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			batch.ID, tenantID, batch.BatchNumber, batch.SterilizerID, batch.SterilizerName,
			batch.ProgramNumber, batch.CycleDate, batch.TemperatureCelsius, batch.PressureBar,
			batch.HoldTimeMinutes, batch.BiResult, batch.CiResult, batch.BowieDickResult,
			batch.OverallResult, batch.ReleasedBy, batch.ReleasedByName, batch.ReleaseDate,
		).Scan(&batch.CreatedAt, &batch.UpdatedAt)
	})
}

// GetBatch gets a sterilization batch by ID
// TENANT-ISOLATED: Queries via RLS
func (r *ReprocessingRepository) GetBatch(ctx context.Context, id string) (*SterilizationBatch, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var batch SterilizationBatch
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, batch_number, sterilizer_id, sterilizer_name, program_number,
			       cycle_date, temperature_celsius, pressure_bar, hold_time_minutes,
			       bi_result, ci_result, bowie_dick_result, overall_result,
			       released_by, released_by_name, release_date, created_at, updated_at
			FROM sterilization_batches WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &batch, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("sterilization_batch")
	}
	if err != nil {
		return nil, err
	}

	return &batch, nil
}

// ListBatches lists sterilization batches with pagination
// TENANT-ISOLATED: Returns only batches via RLS
func (r *ReprocessingRepository) ListBatches(ctx context.Context, page, perPage int) ([]*SterilizationBatch, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var batches []*SterilizationBatch

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		countQuery := `SELECT COUNT(*) FROM sterilization_batches WHERE deleted_at IS NULL`
		if err := r.db.GetContext(ctx, &total, countQuery); err != nil {
			return err
		}

		query := `
			SELECT id, batch_number, sterilizer_id, sterilizer_name, program_number,
			       cycle_date, temperature_celsius, pressure_bar, hold_time_minutes,
			       bi_result, ci_result, bowie_dick_result, overall_result,
			       released_by, released_by_name, release_date, created_at, updated_at
			FROM sterilization_batches WHERE deleted_at IS NULL
			ORDER BY cycle_date DESC
			LIMIT $1 OFFSET $2
		`
		offset := (page - 1) * perPage
		return r.db.SelectContext(ctx, &batches, query, perPage, offset)
	})

	if err != nil {
		return nil, 0, err
	}

	return batches, total, nil
}

// UpdateBatch updates a sterilization batch
// TENANT-ISOLATED: Updates via RLS
func (r *ReprocessingRepository) UpdateBatch(ctx context.Context, batch *SterilizationBatch) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE sterilization_batches SET
				batch_number = $2, sterilizer_id = $3, sterilizer_name = $4, program_number = $5,
				cycle_date = $6, temperature_celsius = $7, pressure_bar = $8, hold_time_minutes = $9,
				bi_result = $10, ci_result = $11, bowie_dick_result = $12, overall_result = $13,
				released_by = $14, released_by_name = $15, release_date = $16, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			batch.ID, batch.BatchNumber, batch.SterilizerID, batch.SterilizerName,
			batch.ProgramNumber, batch.CycleDate, batch.TemperatureCelsius, batch.PressureBar,
			batch.HoldTimeMinutes, batch.BiResult, batch.CiResult, batch.BowieDickResult,
			batch.OverallResult, batch.ReleasedBy, batch.ReleasedByName, batch.ReleaseDate,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("sterilization_batch")
		}

		return nil
	})
}

// DeleteBatch soft-deletes a sterilization batch
// TENANT-ISOLATED: Deletes via RLS
func (r *ReprocessingRepository) DeleteBatch(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE sterilization_batches SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("sterilization_batch")
		}

		return nil
	})
}

// --- Reprocessing Cycles ---

// CreateCycle creates a new reprocessing cycle with auto-incremented cycle number
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *ReprocessingRepository) CreateCycle(ctx context.Context, cycle *ReprocessingCycle) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if cycle.ID == "" {
		cycle.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		// Auto-increment cycle number for this item
		var maxCycle int
		countQuery := `SELECT COALESCE(MAX(cycle_number), 0) FROM reprocessing_cycles WHERE item_id = $1 AND deleted_at IS NULL`
		if err := r.db.GetContext(ctx, &maxCycle, countQuery, cycle.ItemID); err != nil {
			return err
		}
		cycle.CycleNumber = maxCycle + 1

		query := `
			INSERT INTO reprocessing_cycles (
				id, tenant_id, item_id, sterilization_batch_id, cycle_number, cycle_date,
				cleaning_method, disinfection_method, sterilization_method,
				bi_indicator_result, ci_indicator_result,
				released_by, released_by_name, release_date
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			cycle.ID, tenantID, cycle.ItemID, cycle.SterilizationBatchID, cycle.CycleNumber,
			cycle.CycleDate, cycle.CleaningMethod, cycle.DisinfectionMethod,
			cycle.SterilizationMethod, cycle.BiIndicatorResult, cycle.CiIndicatorResult,
			cycle.ReleasedBy, cycle.ReleasedByName, cycle.ReleaseDate,
		).Scan(&cycle.CreatedAt, &cycle.UpdatedAt)
	})
}

// GetCycle gets a reprocessing cycle by ID
// TENANT-ISOLATED: Queries via RLS
func (r *ReprocessingRepository) GetCycle(ctx context.Context, id string) (*ReprocessingCycle, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var cycle ReprocessingCycle
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, sterilization_batch_id, cycle_number, cycle_date,
			       cleaning_method, disinfection_method, sterilization_method,
			       bi_indicator_result, ci_indicator_result,
			       released_by, released_by_name, release_date, created_at, updated_at
			FROM reprocessing_cycles WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &cycle, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("reprocessing_cycle")
	}
	if err != nil {
		return nil, err
	}

	return &cycle, nil
}

// ListCyclesByItem lists reprocessing cycles for an item
// TENANT-ISOLATED: Returns only cycles via RLS
func (r *ReprocessingRepository) ListCyclesByItem(ctx context.Context, itemID string) ([]*ReprocessingCycle, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var cycles []*ReprocessingCycle
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, sterilization_batch_id, cycle_number, cycle_date,
			       cleaning_method, disinfection_method, sterilization_method,
			       bi_indicator_result, ci_indicator_result,
			       released_by, released_by_name, release_date, created_at, updated_at
			FROM reprocessing_cycles WHERE item_id = $1 AND deleted_at IS NULL
			ORDER BY cycle_number DESC
		`
		return r.db.SelectContext(ctx, &cycles, query, itemID)
	})

	if err != nil {
		return nil, err
	}

	return cycles, nil
}

// UpdateCycle updates a reprocessing cycle
// TENANT-ISOLATED: Updates via RLS
func (r *ReprocessingRepository) UpdateCycle(ctx context.Context, cycle *ReprocessingCycle) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE reprocessing_cycles SET
				sterilization_batch_id = $2, cycle_date = $3,
				cleaning_method = $4, disinfection_method = $5, sterilization_method = $6,
				bi_indicator_result = $7, ci_indicator_result = $8,
				released_by = $9, released_by_name = $10, release_date = $11, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			cycle.ID, cycle.SterilizationBatchID, cycle.CycleDate,
			cycle.CleaningMethod, cycle.DisinfectionMethod, cycle.SterilizationMethod,
			cycle.BiIndicatorResult, cycle.CiIndicatorResult,
			cycle.ReleasedBy, cycle.ReleasedByName, cycle.ReleaseDate,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("reprocessing_cycle")
		}

		return nil
	})
}

// DeleteCycle soft-deletes a reprocessing cycle
// TENANT-ISOLATED: Deletes via RLS
func (r *ReprocessingRepository) DeleteCycle(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE reprocessing_cycles SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("reprocessing_cycle")
		}

		return nil
	})
}
