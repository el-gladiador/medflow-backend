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

// SafetyOfficer represents a designated safety officer (Sicherheitsbeauftragter)
type SafetyOfficer struct {
	ID              string     `db:"id" json:"id"`
	UserID          string     `db:"user_id" json:"user_id"`
	UserName        string     `db:"user_name" json:"user_name"`
	Qualification   *string    `db:"qualification" json:"qualification,omitempty"`
	DesignationDate time.Time  `db:"designation_date" json:"designation_date"`
	IsActive        bool       `db:"is_active" json:"is_active"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt       *time.Time `db:"deleted_at" json:"-"`
}

// SafetyOfficerRepository handles safety officer persistence
type SafetyOfficerRepository struct {
	db *database.DB
}

// NewSafetyOfficerRepository creates a new safety officer repository
func NewSafetyOfficerRepository(db *database.DB) *SafetyOfficerRepository {
	return &SafetyOfficerRepository{db: db}
}

// Create creates a new safety officer record
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *SafetyOfficerRepository) Create(ctx context.Context, officer *SafetyOfficer) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if officer.ID == "" {
		officer.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO safety_officers (
				id, tenant_id, user_id, user_name, qualification, designation_date, is_active
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING created_at, updated_at
		`

		return r.db.QueryRowxContext(ctx, query,
			officer.ID, tenantID, officer.UserID, officer.UserName,
			officer.Qualification, officer.DesignationDate, officer.IsActive,
		).Scan(&officer.CreatedAt, &officer.UpdatedAt)
	})
}

// GetByID gets a safety officer by ID
// TENANT-ISOLATED: Queries via RLS
func (r *SafetyOfficerRepository) GetByID(ctx context.Context, id string) (*SafetyOfficer, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var officer SafetyOfficer

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, user_id, user_name, qualification, designation_date, is_active,
			       created_at, updated_at
			FROM safety_officers
			WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &officer, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("safety_officer")
	}
	if err != nil {
		return nil, err
	}

	return &officer, nil
}

// List lists all active safety officers
// TENANT-ISOLATED: Returns only records via RLS
func (r *SafetyOfficerRepository) List(ctx context.Context) ([]*SafetyOfficer, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var officers []*SafetyOfficer

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, user_id, user_name, qualification, designation_date, is_active,
			       created_at, updated_at
			FROM safety_officers
			WHERE is_active = TRUE AND deleted_at IS NULL
			ORDER BY user_name
		`
		return r.db.SelectContext(ctx, &officers, query)
	})

	if err != nil {
		return nil, err
	}

	return officers, nil
}

// Update updates a safety officer record
// TENANT-ISOLATED: Updates via RLS
func (r *SafetyOfficerRepository) Update(ctx context.Context, officer *SafetyOfficer) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE safety_officers SET
				user_id = $2, user_name = $3, qualification = $4,
				designation_date = $5, is_active = $6, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`

		result, err := r.db.ExecContext(ctx, query,
			officer.ID, officer.UserID, officer.UserName, officer.Qualification,
			officer.DesignationDate, officer.IsActive,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("safety_officer")
		}

		return nil
	})
}

// Delete soft deletes a safety officer
// TENANT-ISOLATED: Updates via RLS
func (r *SafetyOfficerRepository) Delete(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE safety_officers SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`

		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("safety_officer")
		}

		return nil
	})
}
