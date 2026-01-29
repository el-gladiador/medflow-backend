package repository

import (
	"context"

	"github.com/medflow/medflow-backend/pkg/database"
)

// CachedUser represents cached user data from user-service
type CachedUser struct {
	UserID   string  `db:"user_id" json:"user_id"`
	Name     string  `db:"name" json:"name"`
	Email    *string `db:"email" json:"email,omitempty"`
	RoleName *string `db:"role_name" json:"role_name,omitempty"`
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
func (r *UserCacheRepository) Set(ctx context.Context, user *CachedUser) error {
	query := `
		INSERT INTO user_cache (user_id, name, email, role_name, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id)
		DO UPDATE SET name = $2, email = $3, role_name = $4, updated_at = NOW()
	`

	_, err := r.db.ExecContext(ctx, query, user.UserID, user.Name, user.Email, user.RoleName)
	return err
}

// Get gets a cached user by ID
func (r *UserCacheRepository) Get(ctx context.Context, userID string) (*CachedUser, error) {
	var user CachedUser
	query := `SELECT user_id, name, email, role_name FROM user_cache WHERE user_id = $1`

	if err := r.db.GetContext(ctx, &user, query, userID); err != nil {
		return nil, err
	}

	return &user, nil
}

// Delete deletes a cached user
func (r *UserCacheRepository) Delete(ctx context.Context, userID string) error {
	query := `DELETE FROM user_cache WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}
