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

// BioRiskAssessment represents a biological agents risk assessment (BioStoffV)
type BioRiskAssessment struct {
	ID                       string     `db:"id" json:"id"`
	ItemID                   string     `db:"item_id" json:"item_id"`
	RiskGroup                int        `db:"risk_group" json:"risk_group"`
	AssessmentDate           time.Time  `db:"assessment_date" json:"assessment_date"`
	AssessorName             string     `db:"assessor_name" json:"assessor_name"`
	ExposureRoutes           *string    `db:"exposure_routes" json:"exposure_routes,omitempty"`
	ProtectiveMeasures       *string    `db:"protective_measures" json:"protective_measures,omitempty"`
	OperatingInstructionsRef *string    `db:"operating_instructions_ref" json:"operating_instructions_ref,omitempty"`
	ValidUntil               *time.Time `db:"valid_until" json:"valid_until,omitempty"`
	CreatedAt                time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt                time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt                *time.Time `db:"deleted_at" json:"-"`
}

// BioTraining represents a biological safety training record
type BioTraining struct {
	ID            string     `db:"id" json:"id"`
	TrainingType  string     `db:"training_type" json:"training_type"`
	TrainingDate  time.Time  `db:"training_date" json:"training_date"`
	TrainerName   string     `db:"trainer_name" json:"trainer_name"`
	AttendeeNames string     `db:"attendee_names" json:"attendee_names"`
	Topic         *string    `db:"topic" json:"topic,omitempty"`
	NextDueDate   *time.Time `db:"next_due_date" json:"next_due_date,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt     *time.Time `db:"deleted_at" json:"-"`
}

// BioSafetyRepository handles biological safety data persistence
type BioSafetyRepository struct {
	db *database.DB
}

// NewBioSafetyRepository creates a new biosafety repository
func NewBioSafetyRepository(db *database.DB) *BioSafetyRepository {
	return &BioSafetyRepository{db: db}
}

// --- Risk Assessments ---

// CreateAssessment creates a new bio risk assessment
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *BioSafetyRepository) CreateAssessment(ctx context.Context, assessment *BioRiskAssessment) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if assessment.ID == "" {
		assessment.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO bio_risk_assessments (
				id, tenant_id, item_id, risk_group, assessment_date, assessor_name,
				exposure_routes, protective_measures, operating_instructions_ref, valid_until
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			assessment.ID, tenantID, assessment.ItemID, assessment.RiskGroup,
			assessment.AssessmentDate, assessment.AssessorName, assessment.ExposureRoutes,
			assessment.ProtectiveMeasures, assessment.OperatingInstructionsRef, assessment.ValidUntil,
		).Scan(&assessment.CreatedAt, &assessment.UpdatedAt)
	})
}

// GetAssessment gets a risk assessment by ID
// TENANT-ISOLATED: Queries via RLS
func (r *BioSafetyRepository) GetAssessment(ctx context.Context, id string) (*BioRiskAssessment, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var assessment BioRiskAssessment
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, risk_group, assessment_date, assessor_name,
			       exposure_routes, protective_measures, operating_instructions_ref,
			       valid_until, created_at, updated_at
			FROM bio_risk_assessments WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &assessment, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("bio_risk_assessment")
	}
	if err != nil {
		return nil, err
	}

	return &assessment, nil
}

// ListAssessmentsByItem lists risk assessments for an item
// TENANT-ISOLATED: Returns only assessments via RLS
func (r *BioSafetyRepository) ListAssessmentsByItem(ctx context.Context, itemID string) ([]*BioRiskAssessment, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var assessments []*BioRiskAssessment
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, risk_group, assessment_date, assessor_name,
			       exposure_routes, protective_measures, operating_instructions_ref,
			       valid_until, created_at, updated_at
			FROM bio_risk_assessments WHERE item_id = $1 AND deleted_at IS NULL
			ORDER BY assessment_date DESC
		`
		return r.db.SelectContext(ctx, &assessments, query, itemID)
	})

	if err != nil {
		return nil, err
	}

	return assessments, nil
}

// UpdateAssessment updates a risk assessment
// TENANT-ISOLATED: Updates via RLS
func (r *BioSafetyRepository) UpdateAssessment(ctx context.Context, assessment *BioRiskAssessment) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE bio_risk_assessments SET
				risk_group = $2, assessment_date = $3, assessor_name = $4,
				exposure_routes = $5, protective_measures = $6,
				operating_instructions_ref = $7, valid_until = $8, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			assessment.ID, assessment.RiskGroup, assessment.AssessmentDate,
			assessment.AssessorName, assessment.ExposureRoutes, assessment.ProtectiveMeasures,
			assessment.OperatingInstructionsRef, assessment.ValidUntil,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("bio_risk_assessment")
		}

		return nil
	})
}

// DeleteAssessment soft-deletes a risk assessment
// TENANT-ISOLATED: Deletes via RLS
func (r *BioSafetyRepository) DeleteAssessment(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE bio_risk_assessments SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("bio_risk_assessment")
		}

		return nil
	})
}

// --- Bio Trainings ---

// CreateTraining creates a new bio training record
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *BioSafetyRepository) CreateTraining(ctx context.Context, training *BioTraining) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if training.ID == "" {
		training.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO bio_trainings (
				id, tenant_id, training_type, training_date, trainer_name,
				attendee_names, topic, next_due_date
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			training.ID, tenantID, training.TrainingType, training.TrainingDate,
			training.TrainerName, training.AttendeeNames, training.Topic, training.NextDueDate,
		).Scan(&training.CreatedAt, &training.UpdatedAt)
	})
}

// GetTraining gets a bio training by ID
// TENANT-ISOLATED: Queries via RLS
func (r *BioSafetyRepository) GetTraining(ctx context.Context, id string) (*BioTraining, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var training BioTraining
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, training_type, training_date, trainer_name, attendee_names,
			       topic, next_due_date, created_at, updated_at
			FROM bio_trainings WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &training, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("bio_training")
	}
	if err != nil {
		return nil, err
	}

	return &training, nil
}

// ListTrainings lists all bio trainings
// TENANT-ISOLATED: Returns only trainings via RLS
func (r *BioSafetyRepository) ListTrainings(ctx context.Context) ([]*BioTraining, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var trainings []*BioTraining
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, training_type, training_date, trainer_name, attendee_names,
			       topic, next_due_date, created_at, updated_at
			FROM bio_trainings WHERE deleted_at IS NULL
			ORDER BY training_date DESC
		`
		return r.db.SelectContext(ctx, &trainings, query)
	})

	if err != nil {
		return nil, err
	}

	return trainings, nil
}

// UpdateTraining updates a bio training record
// TENANT-ISOLATED: Updates via RLS
func (r *BioSafetyRepository) UpdateTraining(ctx context.Context, training *BioTraining) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE bio_trainings SET
				training_type = $2, training_date = $3, trainer_name = $4,
				attendee_names = $5, topic = $6, next_due_date = $7, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			training.ID, training.TrainingType, training.TrainingDate,
			training.TrainerName, training.AttendeeNames, training.Topic, training.NextDueDate,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("bio_training")
		}

		return nil
	})
}

// DeleteTraining soft-deletes a bio training record
// TENANT-ISOLATED: Deletes via RLS
func (r *BioSafetyRepository) DeleteTraining(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE bio_trainings SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("bio_training")
		}

		return nil
	})
}
