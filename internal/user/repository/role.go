package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/tenant"
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
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *RoleRepository) GetByID(ctx context.Context, id string) (*domain.Role, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var role domain.Role
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Actual schema: id, name, display_name, description, is_system, is_default, permissions (JSONB)
		query := `
			SELECT id, name, display_name, description, permissions, created_at, updated_at
			FROM roles
			WHERE id = $1 AND deleted_at IS NULL
		`

		var permissions []byte
		if err := r.db.QueryRowContext(ctx, query, id).Scan(
			&role.ID, &role.Name, &role.DisplayName, &role.Description,
			&permissions, &role.CreatedAt, &role.UpdatedAt,
		); err != nil {
			return err
		}

		// Set computed fields
		role.IsManager = (role.Name == "admin" || role.Name == "manager")
		role.DisplayNameDE = role.DisplayName // Use same for now

		// Parse permissions from JSONB array
		if permissions != nil {
			var permNames []string
			if err := json.Unmarshal(permissions, &permNames); err != nil {
				return fmt.Errorf("failed to parse permissions: %w", err)
			}
			for _, permName := range permNames {
				role.Permissions = append(role.Permissions, domain.Permission{
					Name: permName,
				})
			}
		}

		return nil
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("role")
	}
	if err != nil {
		return nil, err
	}

	return &role, nil
}

// GetByName gets a role by name
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *RoleRepository) GetByName(ctx context.Context, name string) (*domain.Role, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var role domain.Role
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Actual schema: id, name, display_name, description, is_system, is_default, permissions (JSONB)
		query := `
			SELECT id, name, display_name, description, permissions, created_at, updated_at
			FROM roles
			WHERE name = $1 AND deleted_at IS NULL
		`

		var permissions []byte
		if err := r.db.QueryRowContext(ctx, query, name).Scan(
			&role.ID, &role.Name, &role.DisplayName, &role.Description,
			&permissions, &role.CreatedAt, &role.UpdatedAt,
		); err != nil {
			return err
		}

		// Set computed fields
		role.IsManager = (role.Name == "admin" || role.Name == "manager")
		role.DisplayNameDE = role.DisplayName

		// Parse permissions from JSONB array
		if permissions != nil {
			var permNames []string
			if err := json.Unmarshal(permissions, &permNames); err != nil {
				return fmt.Errorf("failed to parse permissions: %w", err)
			}
			for _, permName := range permNames {
				role.Permissions = append(role.Permissions, domain.Permission{
					Name: permName,
				})
			}
		}

		return nil
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("role")
	}
	if err != nil {
		return nil, err
	}

	return &role, nil
}

// List lists all roles
// TENANT-ISOLATED: Returns only roles from the tenant's schema
func (r *RoleRepository) List(ctx context.Context) ([]*domain.Role, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var roles []*domain.Role
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Actual schema: id, name, display_name, description, is_system, is_default, permissions (JSONB)
		query := `
			SELECT id, name, display_name, description, permissions, created_at, updated_at
			FROM roles
			WHERE deleted_at IS NULL
			ORDER BY name
		`

		rows, err := r.db.QueryxContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var role domain.Role
			var permissions []byte
			if err := rows.Scan(
				&role.ID, &role.Name, &role.DisplayName, &role.Description,
				&permissions, &role.CreatedAt, &role.UpdatedAt,
			); err != nil {
				return err
			}

			// Set computed fields
			role.IsManager = (role.Name == "admin" || role.Name == "manager")
			role.DisplayNameDE = role.DisplayName

			// Parse permissions from JSONB array
			if permissions != nil {
				var permNames []string
				if err := json.Unmarshal(permissions, &permNames); err == nil {
					for _, permName := range permNames {
						role.Permissions = append(role.Permissions, domain.Permission{
							Name: permName,
						})
					}
				}
			}

			roles = append(roles, &role)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return roles, nil
}

// GetPermissionByName gets a permission by name
// Note: Permissions are now stored as JSONB arrays in roles table
// This method searches all roles for the permission
func (r *RoleRepository) GetPermissionByName(ctx context.Context, name string) (*domain.Permission, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var perm *domain.Permission
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Search roles for this permission
		query := `
			SELECT permissions FROM roles WHERE deleted_at IS NULL
		`
		rows, err := r.db.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var permissions []byte
			if err := rows.Scan(&permissions); err != nil {
				continue
			}
			var permNames []string
			if err := json.Unmarshal(permissions, &permNames); err != nil {
				continue
			}
			for _, permName := range permNames {
				if permName == name {
					perm = &domain.Permission{Name: name}
					return nil
				}
			}
		}
		return sql.ErrNoRows
	})

	if err == sql.ErrNoRows || perm == nil {
		return nil, errors.NotFound("permission")
	}
	if err != nil {
		return nil, err
	}

	return perm, nil
}

// ListPermissions lists all unique permissions from all roles
// Note: Permissions are now stored as JSONB arrays in roles table
func (r *RoleRepository) ListPermissions(ctx context.Context) ([]*domain.Permission, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var permissions []*domain.Permission
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Collect all unique permissions from all roles
		query := `
			SELECT permissions FROM roles WHERE deleted_at IS NULL
		`
		rows, err := r.db.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()

		permSet := make(map[string]bool)
		for rows.Next() {
			var permsJSON []byte
			if err := rows.Scan(&permsJSON); err != nil {
				continue
			}
			var permNames []string
			if err := json.Unmarshal(permsJSON, &permNames); err != nil {
				continue
			}
			for _, permName := range permNames {
				permSet[permName] = true
			}
		}

		for permName := range permSet {
			permissions = append(permissions, &domain.Permission{Name: permName})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return permissions, nil
}
