package repository

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/medflow/medflow-backend/pkg/database"
)

// UserTenantLookup represents a user-to-tenant mapping for fast login resolution
type UserTenantLookup struct {
	Email        string    `db:"email"`
	UserID       string    `db:"user_id"`
	TenantID     string    `db:"tenant_id"`
	TenantSlug   string    `db:"tenant_slug"`
	TenantSchema string    `db:"tenant_schema"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// UserTenantLookupRepository handles user-tenant lookup persistence
type UserTenantLookupRepository struct {
	db *database.DB
}

// NewUserTenantLookupRepository creates a new user-tenant lookup repository
func NewUserTenantLookupRepository(db *database.DB) *UserTenantLookupRepository {
	return &UserTenantLookupRepository{db: db}
}

// GetByEmail retrieves tenant information for a user by email
// This is the primary lookup used during login for O(1) tenant resolution
func (r *UserTenantLookupRepository) GetByEmail(ctx context.Context, email string) (*UserTenantLookup, error) {
	var lookup UserTenantLookup
	query := `
		SELECT email, user_id, tenant_id, tenant_slug, tenant_schema, created_at, updated_at
		FROM public.user_tenant_lookup
		WHERE email = $1
	`

	if err := r.db.GetContext(ctx, &lookup, query, email); err != nil {
		return nil, err
	}

	return &lookup, nil
}

// GetByUserID retrieves all tenant mappings for a user ID
// Useful for reverse lookups when deleting a user
func (r *UserTenantLookupRepository) GetByUserID(ctx context.Context, userID string) ([]*UserTenantLookup, error) {
	var lookups []*UserTenantLookup
	query := `
		SELECT email, user_id, tenant_id, tenant_slug, tenant_schema, created_at, updated_at
		FROM public.user_tenant_lookup
		WHERE user_id = $1
	`

	if err := r.db.SelectContext(ctx, &lookups, query, userID); err != nil {
		return nil, err
	}

	return lookups, nil
}

// Upsert inserts or updates a user-tenant mapping
// Used when a user is created or when their email changes
func (r *UserTenantLookupRepository) Upsert(ctx context.Context, lookup *UserTenantLookup) error {
	query := `
		INSERT INTO public.user_tenant_lookup (email, user_id, tenant_id, tenant_slug, tenant_schema)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			tenant_id = EXCLUDED.tenant_id,
			tenant_slug = EXCLUDED.tenant_slug,
			tenant_schema = EXCLUDED.tenant_schema,
			updated_at = NOW()
	`

	_, err := r.db.ExecContext(ctx, query,
		lookup.Email,
		lookup.UserID,
		lookup.TenantID,
		lookup.TenantSlug,
		lookup.TenantSchema,
	)

	return err
}

// DeleteByEmail removes a user-tenant mapping by email
// Used when a user is deleted
func (r *UserTenantLookupRepository) DeleteByEmail(ctx context.Context, email string) error {
	query := `DELETE FROM public.user_tenant_lookup WHERE email = $1`
	_, err := r.db.ExecContext(ctx, query, email)
	return err
}

// DeleteByUserID removes all user-tenant mappings for a user ID
// Used as a fallback cleanup when deleting a user
func (r *UserTenantLookupRepository) DeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM public.user_tenant_lookup WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

// UpdateEmail updates the email for an existing lookup entry
// Used when a user's email changes
func (r *UserTenantLookupRepository) UpdateEmail(ctx context.Context, oldEmail, newEmail, userID string) error {
	// Use a transaction to ensure atomicity
	return r.db.Transaction(ctx, func(tx *sqlx.Tx) error {
		// First, get the existing entry's tenant info
		var lookup UserTenantLookup
		err := tx.GetContext(ctx, &lookup, `
			SELECT email, user_id, tenant_id, tenant_slug, tenant_schema, created_at, updated_at
			FROM public.user_tenant_lookup
			WHERE email = $1
		`, oldEmail)
		if err != nil {
			return err
		}

		// Delete old email entry
		_, err = tx.ExecContext(ctx, `DELETE FROM public.user_tenant_lookup WHERE email = $1`, oldEmail)
		if err != nil {
			return err
		}

		// Insert new email entry with the same tenant info
		_, err = tx.ExecContext(ctx, `
			INSERT INTO public.user_tenant_lookup (email, user_id, tenant_id, tenant_slug, tenant_schema)
			VALUES ($1, $2, $3, $4, $5)
		`, newEmail, lookup.UserID, lookup.TenantID, lookup.TenantSlug, lookup.TenantSchema)
		return err
	})
}

// Exists checks if an email exists in the lookup table
func (r *UserTenantLookupRepository) Exists(ctx context.Context, email string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM public.user_tenant_lookup WHERE email = $1`
	if err := r.db.GetContext(ctx, &count, query, email); err != nil {
		return false, err
	}
	return count > 0, nil
}
