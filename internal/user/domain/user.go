package domain

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID           string  `json:"id" db:"id"`
	Email        string  `json:"email" db:"email"`
	PasswordHash string  `json:"-" db:"password_hash"`
	FirstName    string  `json:"first_name" db:"first_name"`
	LastName     string  `json:"last_name" db:"last_name"`
	AvatarURL    *string `json:"avatar_url" db:"avatar_url"`

	// Status (maps to VARCHAR status column: active, inactive, suspended, pending)
	Status string `json:"status" db:"status"`

	// Role (from user_roles junction table)
	Role *Role `json:"role,omitempty" db:"-"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	DeletedAt   *time.Time `json:"-" db:"deleted_at"`

	// Permission overrides (loaded separately)
	PermissionOverrides []PermissionOverride `json:"permission_overrides,omitempty" db:"-"`
	AccessGiverScope    []string             `json:"access_giver_scope,omitempty" db:"-"`
}

// FullName returns the user's full name (first + last)
func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}

// IsActive returns true if user status is "active"
func (u *User) IsActive() bool {
	return u.Status == "active"
}

// Role represents a role with permissions
type Role struct {
	ID                   string    `json:"id" db:"id"`
	Name                 string    `json:"name" db:"name"`
	DisplayName          string    `json:"display_name" db:"display_name"`
	DisplayNameDE        string    `json:"display_name_de" db:"display_name_de"`
	Description          *string   `json:"description,omitempty" db:"description"`
	IsSystem             bool      `json:"is_system" db:"is_system"`
	IsDefault            bool      `json:"is_default" db:"is_default"`
	IsManager            bool      `json:"is_manager" db:"is_manager"`
	CanReceiveDelegation bool      `json:"can_receive_delegation" db:"can_receive_delegation"`
	Level                int       `json:"level" db:"level"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
	// PermissionStrings holds the raw JSONB array of permission strings from the database
	PermissionStrings []string `json:"permission_strings,omitempty" db:"-"`
	// Permissions holds parsed Permission objects (deprecated, for backwards compatibility)
	Permissions []Permission `json:"permissions,omitempty" db:"-"`
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
	ResourceType   *string                `json:"resource_type,omitempty" db:"resource_type"`
	ResourceID     *string                `json:"resource_id,omitempty" db:"resource_id"`
	TargetUserID   *string                `json:"target_user_id,omitempty" db:"target_user_id"`
	TargetUserName *string                `json:"target_user_name,omitempty" db:"target_user_name"`
	OldValues      map[string]interface{} `json:"old_values,omitempty" db:"old_values"`
	NewValues      map[string]interface{} `json:"new_values,omitempty" db:"new_values"`
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

	// Start with role permissions (prefer PermissionStrings, fall back to Permissions)
	permSet := make(map[string]bool)
	if len(u.Role.PermissionStrings) > 0 {
		for _, p := range u.Role.PermissionStrings {
			permSet[p] = true
		}
	} else {
		for _, p := range u.Role.Permissions {
			permSet[p.Name] = true
		}
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
