package repository

import (
	"context"
	"database/sql"

	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
)

// RoleRepository handles role persistence
type RoleRepository struct {
	db *database.DB
}

// NewRoleRepository creates a new role repository
func NewRoleRepository(db *database.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

// GetByID gets a role by ID
func (r *RoleRepository) GetByID(ctx context.Context, id string) (*domain.Role, error) {
	var role domain.Role
	query := `
		SELECT id, name, display_name, display_name_de, description,
		       level::text::int as level, is_manager, can_receive_delegation, created_at, updated_at
		FROM roles
		WHERE id = $1
	`

	if err := r.db.GetContext(ctx, &role, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("role")
		}
		return nil, err
	}

	// Get permissions
	permQuery := `
		SELECT p.id, p.name, p.description, p.category, p.is_admin_only, p.created_at
		FROM permissions p
		JOIN role_permissions rp ON rp.permission_id = p.id
		WHERE rp.role_id = $1
	`
	if err := r.db.SelectContext(ctx, &role.Permissions, permQuery, id); err != nil {
		return nil, err
	}

	return &role, nil
}

// GetByName gets a role by name
func (r *RoleRepository) GetByName(ctx context.Context, name string) (*domain.Role, error) {
	var role domain.Role
	query := `
		SELECT id, name, display_name, display_name_de, description,
		       level::text::int as level, is_manager, can_receive_delegation, created_at, updated_at
		FROM roles
		WHERE name = $1
	`

	if err := r.db.GetContext(ctx, &role, query, name); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("role")
		}
		return nil, err
	}

	// Get permissions
	permQuery := `
		SELECT p.id, p.name, p.description, p.category, p.is_admin_only, p.created_at
		FROM permissions p
		JOIN role_permissions rp ON rp.permission_id = p.id
		WHERE rp.role_id = $1
	`
	if err := r.db.SelectContext(ctx, &role.Permissions, permQuery, role.ID); err != nil {
		return nil, err
	}

	return &role, nil
}

// List lists all roles
func (r *RoleRepository) List(ctx context.Context) ([]*domain.Role, error) {
	query := `
		SELECT id, name, display_name, display_name_de, description,
		       level::text::int as level, is_manager, can_receive_delegation, created_at, updated_at
		FROM roles
		ORDER BY level DESC
	`

	var roles []*domain.Role
	if err := r.db.SelectContext(ctx, &roles, query); err != nil {
		return nil, err
	}

	// Get permissions for each role
	for _, role := range roles {
		permQuery := `
			SELECT p.id, p.name, p.description, p.category, p.is_admin_only, p.created_at
			FROM permissions p
			JOIN role_permissions rp ON rp.permission_id = p.id
			WHERE rp.role_id = $1
		`
		if err := r.db.SelectContext(ctx, &role.Permissions, permQuery, role.ID); err != nil {
			return nil, err
		}
	}

	return roles, nil
}

// GetPermissionByName gets a permission by name
func (r *RoleRepository) GetPermissionByName(ctx context.Context, name string) (*domain.Permission, error) {
	var perm domain.Permission
	query := `
		SELECT id, name, description, category, is_admin_only, created_at
		FROM permissions
		WHERE name = $1
	`

	if err := r.db.GetContext(ctx, &perm, query, name); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("permission")
		}
		return nil, err
	}

	return &perm, nil
}

// ListPermissions lists all permissions
func (r *RoleRepository) ListPermissions(ctx context.Context) ([]*domain.Permission, error) {
	query := `
		SELECT id, name, description, category, is_admin_only, created_at
		FROM permissions
		ORDER BY category, name
	`

	var permissions []*domain.Permission
	if err := r.db.SelectContext(ctx, &permissions, query); err != nil {
		return nil, err
	}

	return permissions, nil
}
