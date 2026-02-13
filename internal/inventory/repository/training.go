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

// DeviceTraining represents a device training record (Einweisung)
type DeviceTraining struct {
	ID                   string     `db:"id" json:"id"`
	ItemID               string     `db:"item_id" json:"item_id"`
	TrainingDate         time.Time  `db:"training_date" json:"training_date"`
	TrainerName          string     `db:"trainer_name" json:"trainer_name"`
	TrainerQualification *string    `db:"trainer_qualification" json:"trainer_qualification,omitempty"`
	AttendeeNames        string     `db:"attendee_names" json:"attendee_names"`
	Topic                *string    `db:"topic" json:"topic,omitempty"`
	Notes                *string    `db:"notes" json:"notes,omitempty"`
	CreatedAt            time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt            *time.Time `db:"deleted_at" json:"-"`
	CreatedBy            *string    `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy            *string    `db:"updated_by" json:"updated_by,omitempty"`
}

// TrainingRepository handles device training persistence
type TrainingRepository struct {
	db *database.DB
}

// NewTrainingRepository creates a new training repository
func NewTrainingRepository(db *database.DB) *TrainingRepository {
	return &TrainingRepository{db: db}
}

// Create creates a new device training
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *TrainingRepository) Create(ctx context.Context, tr *DeviceTraining) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if tr.ID == "" {
		tr.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO device_trainings (
				id, tenant_id, item_id, training_date, trainer_name, trainer_qualification,
				attendee_names, topic, notes, created_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			tr.ID, tenantID, tr.ItemID, tr.TrainingDate, tr.TrainerName,
			tr.TrainerQualification, tr.AttendeeNames, tr.Topic, tr.Notes, tr.CreatedBy,
		).Scan(&tr.CreatedAt, &tr.UpdatedAt)
	})
}

// GetByID gets a training by ID
// TENANT-ISOLATED: Queries via RLS
func (r *TrainingRepository) GetByID(ctx context.Context, id string) (*DeviceTraining, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var tr DeviceTraining
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, training_date, trainer_name, trainer_qualification,
			       attendee_names, topic, notes, created_at, updated_at, created_by, updated_by
			FROM device_trainings WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &tr, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("training")
	}
	if err != nil {
		return nil, err
	}

	return &tr, nil
}

// ListByItem lists trainings for a device
// TENANT-ISOLATED: Returns only trainings via RLS
func (r *TrainingRepository) ListByItem(ctx context.Context, itemID string) ([]*DeviceTraining, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var trainings []*DeviceTraining
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, training_date, trainer_name, trainer_qualification,
			       attendee_names, topic, notes, created_at, updated_at, created_by, updated_by
			FROM device_trainings WHERE item_id = $1 AND deleted_at IS NULL
			ORDER BY training_date DESC
		`
		return r.db.SelectContext(ctx, &trainings, query, itemID)
	})

	if err != nil {
		return nil, err
	}

	return trainings, nil
}

// Update updates a training
// TENANT-ISOLATED: Updates via RLS
func (r *TrainingRepository) Update(ctx context.Context, tr *DeviceTraining) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE device_trainings SET
				training_date = $2, trainer_name = $3, trainer_qualification = $4,
				attendee_names = $5, topic = $6, notes = $7, updated_by = $8,
				updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			tr.ID, tr.TrainingDate, tr.TrainerName, tr.TrainerQualification,
			tr.AttendeeNames, tr.Topic, tr.Notes, tr.UpdatedBy,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("training")
		}

		return nil
	})
}

// Delete soft-deletes a training
// TENANT-ISOLATED: Deletes via RLS
func (r *TrainingRepository) Delete(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE device_trainings SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("training")
		}

		return nil
	})
}
