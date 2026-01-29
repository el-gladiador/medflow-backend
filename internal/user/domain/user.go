package domain

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID                  string              `json:"id" db:"id"`
	Email               string              `json:"email" db:"email"`
	PasswordHash        string              `json:"-" db:"password_hash"`
	Name                string              `json:"name" db:"name"`
	Avatar              *string             `json:"avatar" db:"avatar"`
	RoleID              string              `json:"-" db:"role_id"`
	Role                *Role               `json:"role,omitempty" db:"-"`
	IsActive            bool                `json:"is_active" db:"is_active"`
	IsAccessGiver       bool                `json:"is_access_giver" db:"is_access_giver"`
	CreatedBy           *string             `json:"created_by,omitempty" db:"created_by"`
	CreatedAt           time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at" db:"updated_at"`
	LastLoginAt         *time.Time          `json:"last_login_at,omitempty" db:"last_login_at"`
	DeletedAt           *time.Time          `json:"-" db:"deleted_at"`
	PermissionOverrides []PermissionOverride `json:"permission_overrides,omitempty" db:"-"`
	AccessGiverScope    []string            `json:"access_giver_scope,omitempty" db:"-"`
}

// Role represents a role with permissions
type Role struct {
	ID                   string       `json:"id" db:"id"`
	Name                 string       `json:"name" db:"name"`
	DisplayName          string       `json:"display_name" db:"display_name"`
	DisplayNameDE        string       `json:"display_name_de" db:"display_name_de"`
	Description          *string      `json:"description,omitempty" db:"description"`
	Level                int          `json:"level" db:"level"`
	IsManager            bool         `json:"is_manager" db:"is_manager"`
	CanReceiveDelegation bool         `json:"can_receive_delegation" db:"can_receive_delegation"`
	CreatedAt            time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time    `json:"updated_at" db:"updated_at"`
	Permissions          []Permission `json:"permissions,omitempty" db:"-"`
}

// Permission represents a permission
type Permission struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description,omitempty" db:"description"`
	Category    string    `json:"category" db:"category"`
	IsAdminOnly bool      `json:"is_admin_only" db:"is_admin_only"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// PermissionOverride represents a per-user permission grant/revoke
type PermissionOverride struct {
	ID           string     `json:"id" db:"id"`
	UserID       string     `json:"user_id" db:"user_id"`
	PermissionID string     `json:"permission_id" db:"permission_id"`
	Permission   string     `json:"permission" db:"permission"` // Permission name (from JOIN)
	Granted      bool       `json:"granted" db:"granted"`
	GrantedBy    string     `json:"granted_by" db:"granted_by"`
	GrantedAt    time.Time  `json:"granted_at" db:"granted_at"`
	Reason       *string    `json:"reason,omitempty" db:"reason"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty" db:"expires_at"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID             string                 `json:"id" db:"id"`
	ActorID        *string                `json:"actor_id" db:"actor_id"`
	ActorName      string                 `json:"actor_name" db:"actor_name"`
	Action         string                 `json:"action" db:"action"`
	TargetUserID   *string                `json:"target_user_id,omitempty" db:"target_user_id"`
	TargetUserName *string                `json:"target_user_name,omitempty" db:"target_user_name"`
	ResourceType   *string                `json:"resource_type,omitempty" db:"resource_type"`
	ResourceID     *string                `json:"resource_id,omitempty" db:"resource_id"`
	Details        map[string]interface{} `json:"details,omitempty" db:"details"`
	IPAddress      *string                `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent      *string                `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
}

// UserWithPermissions includes computed effective permissions
type UserWithPermissions struct {
	*User
	Permissions          []string `json:"permissions"`
	EffectivePermissions []string `json:"effective_permissions"`
}

// GetEffectivePermissions computes effective permissions from role + overrides
func (u *User) GetEffectivePermissions() []string {
	if u.Role == nil {
		return []string{}
	}

	// Start with role permissions
	permSet := make(map[string]bool)
	for _, p := range u.Role.Permissions {
		permSet[p.Name] = true
	}

	// Apply overrides
	for _, override := range u.PermissionOverrides {
		if override.Granted {
			permSet[override.Permission] = true
		} else {
			delete(permSet, override.Permission)
		}
	}

	// Convert to slice
	permissions := make([]string, 0, len(permSet))
	for p := range permSet {
		permissions = append(permissions, p)
	}

	return permissions
}
