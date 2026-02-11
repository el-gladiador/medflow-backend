package repository

import (
	"context"

	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// CachedUser represents cached user data (matches user_cache table in tenant schema)
type CachedUser struct {
	UserID    string  `db:"user_id" json:"user_id"`
	FirstName string  `db:"first_name" json:"first_name"`
	LastName  string  `db:"last_name" json:"last_name"`
	Email     *string `db:"email" json:"email,omitempty"`
	RoleName  *string `db:"role_name" json:"role_name,omitempty"`
	TenantID  string  `db:"tenant_id" json:"tenant_id"`
}

// FullName returns the user's full name
func (u *CachedUser) FullName() string {
	return u.FirstName + " " + u.LastName
}

// UserCacheRepository handles user cache persistence
type UserCacheRepository struct {
	db *database.DB
}

// NewUserCacheRepository creates a new user cache repository
func NewUserCacheRepository(db *database.DB) *UserCacheRepository {
	return &UserCacheRepository{db: db}
}

// Set creates or updates a cached user
// TENANT-ISOLATED: Uses tenant ID from context for RLS
func (r *UserCacheRepository) Set(ctx context.Context, user *CachedUser) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO user_cache (user_id, tenant_id, first_name, last_name, email, role_name, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())
			ON CONFLICT (user_id)
			DO UPDATE SET first_name = $3, last_name = $4, email = $5, role_name = $6, updated_at = NOW()
		`

		_, err := r.db.ExecContext(ctx, query, user.UserID, tenantID, user.FirstName, user.LastName, user.Email, user.RoleName)
		return err
	})
}

// Get gets a cached user by ID
// TENANT-ISOLATED: Uses tenant ID from context for RLS
func (r *UserCacheRepository) Get(ctx context.Context, userID string) (*CachedUser, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var user CachedUser
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `SELECT user_id, first_name, last_name, email, role_name, tenant_id FROM user_cache WHERE user_id = $1`
		return r.db.GetContext(ctx, &user, query, userID)
	})

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// Delete deletes a cached user
// TENANT-ISOLATED: Uses tenant ID from context for RLS
func (r *UserCacheRepository) Delete(ctx context.Context, userID string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `DELETE FROM user_cache WHERE user_id = $1`
		_, err := r.db.ExecContext(ctx, query, userID)
		return err
	})
}
