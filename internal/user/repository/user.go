package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/tenant"
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
// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
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
	})
}

// GetByID gets a user by ID
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var user domain.User
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, email, password_hash, name, avatar, role_id, is_active, is_access_giver,
			       created_by, created_at, updated_at, last_login_at, deleted_at
			FROM users
			WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &user, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("user")
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetByEmail gets a user by email
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var user domain.User
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, email, password_hash, name, avatar, role_id, is_active, is_access_giver,
			       created_by, created_at, updated_at, last_login_at, deleted_at
			FROM users
			WHERE email = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &user, query, email)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("user")
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetWithRole gets a user with their role information
// TENANT-ISOLATED: Queries only the tenant's schema for user and related data
func (r *UserRepository) GetWithRole(ctx context.Context, id string) (*domain.User, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	user, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetByID failed: %w", err)
	}

	// Execute role and permission queries with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Get role
		var role domain.Role
		roleQuery := `
			SELECT id, name, display_name, display_name_de, description,
			       level::text::int as level, is_manager, can_receive_delegation, created_at, updated_at
			FROM roles
			WHERE id = $1
		`
		if err := r.db.GetContext(ctx, &role, roleQuery, user.RoleID); err != nil {
			return fmt.Errorf("role query failed for role_id %s: %w", user.RoleID, err)
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
			return fmt.Errorf("permissions query failed: %w", err)
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
			return fmt.Errorf("overrides query failed: %w", err)
		}

		// Get access giver scope
		scopeQuery := `
			SELECT r.name
			FROM access_giver_scope ags
			JOIN roles r ON r.id = ags.role_id
			WHERE ags.user_id = $1
		`
		if err := r.db.SelectContext(ctx, &user.AccessGiverScope, scopeQuery, id); err != nil {
			return fmt.Errorf("scope query failed: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return user, nil
}

// List lists all users with pagination
// TENANT-ISOLATED: Returns only users from the tenant's schema
func (r *UserRepository) List(ctx context.Context, page, perPage int) ([]*domain.User, int64, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var users []*domain.User

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Count total
		countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
		if err := r.db.GetContext(ctx, &total, countQuery); err != nil {
			return err
		}

		// Get paginated users with role from user_roles junction table
		// Using actual schema: first_name, last_name, avatar_url, status
		offset := (page - 1) * perPage
		query := `
			SELECT u.id, u.email, u.first_name, u.last_name, u.avatar_url, u.status,
			       u.created_at, u.updated_at, u.last_login_at,
			       r.id as role_id, r.name as role_name, r.display_name as role_display_name,
			       r.permissions as role_permissions
			FROM users u
			LEFT JOIN user_roles ur ON ur.user_id = u.id
			LEFT JOIN roles r ON r.id = ur.role_id
			WHERE u.deleted_at IS NULL
			ORDER BY u.created_at DESC
			LIMIT $1 OFFSET $2
		`

		rows, err := r.db.QueryxContext(ctx, query, perPage, offset)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var firstName, lastName string
			var avatarURL sql.NullString
			var status string
			var roleID sql.NullString
			var roleName sql.NullString
			var roleDisplayName sql.NullString
			var rolePermissions []byte
			var user domain.User

			if err := rows.Scan(
				&user.ID, &user.Email, &firstName, &lastName, &avatarURL, &status,
				&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
				&roleID, &roleName, &roleDisplayName, &rolePermissions,
			); err != nil {
				return err
			}

			// Map schema fields to domain model
			user.Name = fmt.Sprintf("%s %s", firstName, lastName)
			if avatarURL.Valid {
				avatar := avatarURL.String
				user.Avatar = &avatar
			}
			user.IsActive = (status == "active")

			// Build role from query results
			if roleID.Valid && roleName.Valid {
				user.Role = &domain.Role{
					ID:          roleID.String,
					Name:        roleName.String,
					DisplayName: roleDisplayName.String,
					IsManager:   (roleName.String == "admin" || roleName.String == "manager"),
				}

				// Parse permissions from JSONB
				if rolePermissions != nil {
					var permNames []string
					if err := json.Unmarshal(rolePermissions, &permNames); err == nil {
						for _, permName := range permNames {
							user.Role.Permissions = append(user.Role.Permissions, domain.Permission{
								Name: permName,
							})
						}
					}
				}
			}

			users = append(users, &user)
		}

		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// Update updates a user
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
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
	})
}

// UpdatePassword updates a user's password
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *UserRepository) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `UPDATE users SET password_hash = $2 WHERE id = $1 AND deleted_at IS NULL`
		_, err := r.db.ExecContext(ctx, query, id, passwordHash)
		return err
	})
}

// UpdateLastLogin updates the last login timestamp
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *UserRepository) UpdateLastLogin(ctx context.Context, id string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `UPDATE users SET last_login_at = NOW() WHERE id = $1`
		_, err := r.db.ExecContext(ctx, query, id)
		return err
	})
}

// SoftDelete soft deletes a user
// TENANT-ISOLATED: Soft deletes only in the tenant's schema
func (r *UserRepository) SoftDelete(ctx context.Context, id string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
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
	})
}

// AddPermissionOverride adds a permission override
// TENANT-ISOLATED: Inserts/updates only in the tenant's schema
func (r *UserRepository) AddPermissionOverride(ctx context.Context, override *domain.PermissionOverride) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if override.ID == "" {
		override.ID = uuid.New().String()
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
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
	})
}

// RemovePermissionOverride removes a permission override
// TENANT-ISOLATED: Deletes only from the tenant's schema
func (r *UserRepository) RemovePermissionOverride(ctx context.Context, userID, permission string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			DELETE FROM permission_overrides
			WHERE user_id = $1 AND permission_id = (SELECT id FROM permissions WHERE name = $2)
		`
		_, err := r.db.ExecContext(ctx, query, userID, permission)
		return err
	})
}

// SetAccessGiverScope sets the access giver scope for a user
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *UserRepository) SetAccessGiverScope(ctx context.Context, userID string, roleNames []string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
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
	})
}

// ClearAccessGiverScope clears the access giver scope
// TENANT-ISOLATED: Deletes only from the tenant's schema
func (r *UserRepository) ClearAccessGiverScope(ctx context.Context, userID string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		_, err := r.db.ExecContext(ctx, `DELETE FROM access_giver_scope WHERE user_id = $1`, userID)
		return err
	})
}

// TenantInfo holds tenant metadata
type TenantInfo struct {
	ID         string
	Slug       string
	SchemaName string
}

// GetUserWithRoleFromJunction fetches user with their role from user_roles junction table
// This is used after FindUserAcrossTenants since that doesn't know the role yet
func (r *UserRepository) GetUserWithRoleFromJunction(ctx context.Context, userID string) (*domain.User, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var user domain.User
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Get user basic info
		userQuery := `
			SELECT id, email, first_name, last_name, avatar_url, status, created_at, updated_at, last_login_at
			FROM users
			WHERE id = $1 AND deleted_at IS NULL
		`
		var firstName, lastName string
		var avatarURL sql.NullString
		var status string

		if err := r.db.QueryRowContext(ctx, userQuery, userID).Scan(
			&user.ID, &user.Email, &firstName, &lastName, &avatarURL, &status,
			&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
		); err != nil {
			return err
		}

		user.Name = fmt.Sprintf("%s %s", firstName, lastName)
		if avatarURL.Valid {
			avatar := avatarURL.String
			user.Avatar = &avatar
		}
		user.IsActive = (status == "active")

		// Get user's first role (for multi-role support, we'd need a different approach)
		// Note: Actual schema has permissions as JSONB array on roles
		roleQuery := `
			SELECT r.id, r.name, r.display_name, r.description, r.permissions,
			       r.created_at, r.updated_at
			FROM roles r
			JOIN user_roles ur ON ur.role_id = r.id
			WHERE ur.user_id = $1
			LIMIT 1
		`
		var roleData struct {
			ID          string         `db:"id"`
			Name        string         `db:"name"`
			DisplayName string         `db:"display_name"`
			Description sql.NullString `db:"description"`
			Permissions []byte         `db:"permissions"`
			CreatedAt   time.Time      `db:"created_at"`
			UpdatedAt   time.Time      `db:"updated_at"`
		}
		if err := r.db.GetContext(ctx, &roleData, roleQuery, userID); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("user has no role assigned")
			}
			return err
		}

		// Map to domain.Role
		user.Role = &domain.Role{
			ID:            roleData.ID,
			Name:          roleData.Name,
			DisplayName:   roleData.DisplayName,
			DisplayNameDE: roleData.DisplayName, // Use same for now
			IsManager:     (roleData.Name == "admin" || roleData.Name == "manager"),
			CreatedAt:     roleData.CreatedAt,
			UpdatedAt:     roleData.UpdatedAt,
		}
		if roleData.Description.Valid {
			descStr := roleData.Description.String
			user.Role.Description = &descStr
		}

		// Parse permissions from JSONB array of strings
		var permNames []string
		if err := json.Unmarshal(roleData.Permissions, &permNames); err != nil {
			return fmt.Errorf("failed to parse permissions: %w", err)
		}

		// Convert permission names to Permission objects
		for _, permName := range permNames {
			user.Role.Permissions = append(user.Role.Permissions, domain.Permission{
				Name: permName,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// FindUserAcrossTenants searches all active tenant schemas for a user by email.
// This is used ONLY during login to determine which tenant owns the email.
// Returns the user and the tenant information.
//
// Performance: O(N) where N is number of active tenants.
// With 100 tenants and email index, typically < 100ms worst case.
//
// Security: Only searches active/trial tenants. Suspended tenants are excluded.
func (r *UserRepository) FindUserAcrossTenants(ctx context.Context, email string) (*domain.User, *TenantInfo, error) {
	// Step 1: Get all active tenants from auth DB
	var tenants []TenantInfo
	tenantsQuery := `
		SELECT id, slug, schema_name
		FROM public.tenants
		WHERE deleted_at IS NULL
		  AND subscription_status IN ('active', 'trial')
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, tenantsQuery)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query tenants: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t TenantInfo
		if err := rows.Scan(&t.ID, &t.Slug, &t.SchemaName); err != nil {
			continue
		}
		tenants = append(tenants, t)
	}

	if len(tenants) == 0 {
		return nil, nil, errors.NotFound("no active tenants found")
	}

	// Step 2: Search each tenant schema for the email
	for _, tenant := range tenants {
		// Query the tenant's schema for this email
		// Note: Actual schema uses first_name/last_name, avatar_url, status
		userQuery := fmt.Sprintf(`
			SELECT id, email, password_hash, first_name, last_name, avatar_url, status,
			       created_at, updated_at, last_login_at
			FROM %s.users
			WHERE email = $1 AND deleted_at IS NULL AND status = 'active'
			LIMIT 1
		`, tenant.SchemaName)

		var user domain.User
		var firstName, lastName string
		var avatarURL sql.NullString
		var status string

		err := r.db.QueryRowContext(ctx, userQuery, email).Scan(
			&user.ID,
			&user.Email,
			&user.PasswordHash,
			&firstName,
			&lastName,
			&avatarURL,
			&status,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.LastLoginAt,
		)

		if err == nil {
			// Found the user in this tenant - map schema fields to domain model
			user.Name = fmt.Sprintf("%s %s", firstName, lastName)
			if avatarURL.Valid {
				avatar := avatarURL.String
				user.Avatar = &avatar
			}
			user.IsActive = (status == "active")
			return &user, &tenant, nil
		}

		if err != sql.ErrNoRows {
			// Actual error (not just "not found")
			// Log but continue searching other tenants
			continue
		}
	}

	// User not found in any tenant
	return nil, nil, errors.InvalidCredentials()
}
