package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
)

// UserRepository handles user persistence
type UserRepository struct {
	db *database.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	query := `
		INSERT INTO users (id, email, password_hash, name, avatar, role_id, is_active, is_access_giver, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`

	return r.db.QueryRowxContext(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.Name,
		user.Avatar,
		user.RoleID,
		user.IsActive,
		user.IsAccessGiver,
		user.CreatedBy,
	).Scan(&user.CreatedAt, &user.UpdatedAt)
}

// GetByID gets a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	query := `
		SELECT id, email, password_hash, name, avatar, role_id, is_active, is_access_giver,
		       created_by, created_at, updated_at, last_login_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	if err := r.db.GetContext(ctx, &user, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("user")
		}
		return nil, err
	}

	return &user, nil
}

// GetByEmail gets a user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	query := `
		SELECT id, email, password_hash, name, avatar, role_id, is_active, is_access_giver,
		       created_by, created_at, updated_at, last_login_at, deleted_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`

	if err := r.db.GetContext(ctx, &user, query, email); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("user")
		}
		return nil, err
	}

	return &user, nil
}

// GetWithRole gets a user with their role information
func (r *UserRepository) GetWithRole(ctx context.Context, id string) (*domain.User, error) {
	user, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetByID failed: %w", err)
	}

	// Get role
	var role domain.Role
	roleQuery := `
		SELECT id, name, display_name, display_name_de, description,
		       level::text::int as level, is_manager, can_receive_delegation, created_at, updated_at
		FROM roles
		WHERE id = $1
	`
	if err := r.db.GetContext(ctx, &role, roleQuery, user.RoleID); err != nil {
		return nil, fmt.Errorf("role query failed for role_id %s: %w", user.RoleID, err)
	}
	user.Role = &role

	// Get role permissions
	permQuery := `
		SELECT p.id, p.name, p.description, p.category, p.is_admin_only, p.created_at
		FROM permissions p
		JOIN role_permissions rp ON rp.permission_id = p.id
		WHERE rp.role_id = $1
	`
	if err := r.db.SelectContext(ctx, &user.Role.Permissions, permQuery, role.ID); err != nil {
		return nil, fmt.Errorf("permissions query failed: %w", err)
	}

	// Get permission overrides
	overrideQuery := `
		SELECT po.id, po.user_id, po.permission_id, p.name as permission, po.granted,
		       po.granted_by, po.granted_at, po.reason, po.expires_at
		FROM permission_overrides po
		JOIN permissions p ON p.id = po.permission_id
		WHERE po.user_id = $1 AND (po.expires_at IS NULL OR po.expires_at > NOW())
	`
	if err := r.db.SelectContext(ctx, &user.PermissionOverrides, overrideQuery, id); err != nil {
		return nil, fmt.Errorf("overrides query failed: %w", err)
	}

	// Get access giver scope
	scopeQuery := `
		SELECT r.name
		FROM access_giver_scope ags
		JOIN roles r ON r.id = ags.role_id
		WHERE ags.user_id = $1
	`
	if err := r.db.SelectContext(ctx, &user.AccessGiverScope, scopeQuery, id); err != nil {
		return nil, fmt.Errorf("scope query failed: %w", err)
	}

	return user, nil
}

// List lists all users with pagination
func (r *UserRepository) List(ctx context.Context, page, perPage int) ([]*domain.User, int64, error) {
	var total int64
	countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	if err := r.db.GetContext(ctx, &total, countQuery); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	query := `
		SELECT u.id, u.email, u.name, u.avatar, u.role_id, u.is_active, u.is_access_giver,
		       u.created_by, u.created_at, u.updated_at, u.last_login_at,
		       r.name as "role.name", r.display_name as "role.display_name",
		       r.display_name_de as "role.display_name_de", r.is_manager as "role.is_manager"
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.deleted_at IS NULL
		ORDER BY u.created_at DESC
		LIMIT $1 OFFSET $2
	`

	var users []*domain.User
	rows, err := r.db.QueryxContext(ctx, query, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var user domain.User
		user.Role = &domain.Role{}
		if err := rows.Scan(
			&user.ID, &user.Email, &user.Name, &user.Avatar, &user.RoleID,
			&user.IsActive, &user.IsAccessGiver, &user.CreatedBy, &user.CreatedAt,
			&user.UpdatedAt, &user.LastLoginAt,
			&user.Role.Name, &user.Role.DisplayName, &user.Role.DisplayNameDE, &user.Role.IsManager,
		); err != nil {
			return nil, 0, err
		}
		users = append(users, &user)
	}

	return users, total, nil
}

// Update updates a user
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET email = $2, name = $3, avatar = $4, role_id = $5, is_active = $6, is_access_giver = $7
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Email,
		user.Name,
		user.Avatar,
		user.RoleID,
		user.IsActive,
		user.IsAccessGiver,
	)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("user")
	}

	return nil
}

// UpdatePassword updates a user's password
func (r *UserRepository) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	query := `UPDATE users SET password_hash = $2 WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, id, passwordHash)
	return err
}

// UpdateLastLogin updates the last login timestamp
func (r *UserRepository) UpdateLastLogin(ctx context.Context, id string) error {
	query := `UPDATE users SET last_login_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// SoftDelete soft deletes a user
func (r *UserRepository) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE users SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("user")
	}

	return nil
}

// AddPermissionOverride adds a permission override
func (r *UserRepository) AddPermissionOverride(ctx context.Context, override *domain.PermissionOverride) error {
	if override.ID == "" {
		override.ID = uuid.New().String()
	}

	query := `
		INSERT INTO permission_overrides (id, user_id, permission_id, granted, granted_by, granted_at, reason, expires_at)
		VALUES ($1, $2, (SELECT id FROM permissions WHERE name = $3), $4, $5, NOW(), $6, $7)
		ON CONFLICT (user_id, permission_id)
		DO UPDATE SET granted = $4, granted_by = $5, granted_at = NOW(), reason = $6, expires_at = $7
	`

	_, err := r.db.ExecContext(ctx, query,
		override.ID,
		override.UserID,
		override.Permission,
		override.Granted,
		override.GrantedBy,
		override.Reason,
		override.ExpiresAt,
	)

	return err
}

// RemovePermissionOverride removes a permission override
func (r *UserRepository) RemovePermissionOverride(ctx context.Context, userID, permission string) error {
	query := `
		DELETE FROM permission_overrides
		WHERE user_id = $1 AND permission_id = (SELECT id FROM permissions WHERE name = $2)
	`
	_, err := r.db.ExecContext(ctx, query, userID, permission)
	return err
}

// SetAccessGiverScope sets the access giver scope for a user
func (r *UserRepository) SetAccessGiverScope(ctx context.Context, userID string, roleNames []string) error {
	// Clear existing scope
	_, err := r.db.ExecContext(ctx, `DELETE FROM access_giver_scope WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}

	// Add new scope
	for _, roleName := range roleNames {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO access_giver_scope (user_id, role_id)
			SELECT $1, id FROM roles WHERE name = $2
		`, userID, roleName)
		if err != nil {
			return err
		}
	}

	return nil
}

// ClearAccessGiverScope clears the access giver scope
func (r *UserRepository) ClearAccessGiverScope(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM access_giver_scope WHERE user_id = $1`, userID)
	return err
}
