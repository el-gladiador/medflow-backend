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

// RetentionPolicy represents a data retention policy for regulatory compliance
type RetentionPolicy struct {
	ID             string     `db:"id" json:"id"`
	EntityType     string     `db:"entity_type" json:"entity_type"`
	RetentionYears int        `db:"retention_years" json:"retention_years"`
	LegalBasis     *string    `db:"legal_basis" json:"legal_basis,omitempty"`
	Description    *string    `db:"description" json:"description,omitempty"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at" json:"-"`
}

// RetentionRepository handles retention policy persistence
type RetentionRepository struct {
	db *database.DB
}

// NewRetentionRepository creates a new retention repository
func NewRetentionRepository(db *database.DB) *RetentionRepository {
	return &RetentionRepository{db: db}
}

// Create creates a new retention policy
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *RetentionRepository) Create(ctx context.Context, policy *RetentionPolicy) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO retention_policies (
				id, tenant_id, entity_type, retention_years, legal_basis, description
			) VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			policy.ID, tenantID, policy.EntityType, policy.RetentionYears,
			policy.LegalBasis, policy.Description,
		).Scan(&policy.CreatedAt, &policy.UpdatedAt)
	})
}

// GetByEntityType gets the retention policy for a specific entity type
// TENANT-ISOLATED: Queries via RLS
func (r *RetentionRepository) GetByEntityType(ctx context.Context, entityType string) (*RetentionPolicy, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var policy RetentionPolicy
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, entity_type, retention_years, legal_basis, description,
			       created_at, updated_at
			FROM retention_policies WHERE entity_type = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &policy, query, entityType)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("retention_policy")
	}
	if err != nil {
		return nil, err
	}

	return &policy, nil
}

// List lists all retention policies
// TENANT-ISOLATED: Returns only policies via RLS
func (r *RetentionRepository) List(ctx context.Context) ([]*RetentionPolicy, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var policies []*RetentionPolicy
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, entity_type, retention_years, legal_basis, description,
			       created_at, updated_at
			FROM retention_policies WHERE deleted_at IS NULL
			ORDER BY entity_type ASC
		`
		return r.db.SelectContext(ctx, &policies, query)
	})

	if err != nil {
		return nil, err
	}

	return policies, nil
}

// Update updates a retention policy
// TENANT-ISOLATED: Updates via RLS
func (r *RetentionRepository) Update(ctx context.Context, policy *RetentionPolicy) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE retention_policies SET
				entity_type = $2, retention_years = $3, legal_basis = $4,
				description = $5, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			policy.ID, policy.EntityType, policy.RetentionYears,
			policy.LegalBasis, policy.Description,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("retention_policy")
		}

		return nil
	})
}

// Delete soft-deletes a retention policy
// TENANT-ISOLATED: Deletes via RLS
func (r *RetentionRepository) Delete(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE retention_policies SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("retention_policy")
		}

		return nil
	})
}

// ValidateDeletion checks if a record is still within the retention period
// TENANT-ISOLATED: Queries retention policy via RLS
// Returns nil if deletion is allowed, error if record must be retained
func (r *RetentionRepository) ValidateDeletion(ctx context.Context, entityType string, recordDate time.Time) error {
	policy, err := r.GetByEntityType(ctx, entityType)
	if err != nil {
		// If no policy exists, allow deletion
		var appErr *errors.AppError
		if errors.As(err, &appErr) && appErr.Code == "NOT_FOUND" {
			return nil
		}
		return err
	}

	retentionEnd := recordDate.AddDate(policy.RetentionYears, 0, 0)
	if time.Now().Before(retentionEnd) {
		legalBasis := ""
		if policy.LegalBasis != nil {
			legalBasis = *policy.LegalBasis
		}
		return fmt.Errorf(
			"record cannot be deleted: retention period of %d years for %q has not expired (until %s, legal basis: %s)",
			policy.RetentionYears, entityType, retentionEnd.Format("2006-01-02"), legalBasis,
		)
	}

	return nil
}
