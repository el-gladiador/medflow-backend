// Package actor provides a universal pattern for identifying and tracking
// the user/system performing actions across services.
//
// This package is used for:
// - Audit logging (who performed an action)
// - Cross-service user identification
// - User cache population in non-user services
package actor

import (
	"context"
	"fmt"
)

// Actor represents the entity performing an action in the system.
// This is the universal pattern for cross-service user identification.
type Actor struct {
	// ID is the unique identifier of the actor (user ID)
	ID string `json:"id"`

	// FirstName is the actor's first name
	FirstName string `json:"first_name"`

	// LastName is the actor's last name
	LastName string `json:"last_name"`

	// Email is the actor's email address
	Email string `json:"email"`

	// TenantID is the tenant the actor belongs to
	TenantID string `json:"tenant_id"`

	// RoleName is the actor's role (optional, for display purposes)
	RoleName string `json:"role_name,omitempty"`
}

// FullName returns the actor's full name (first + last)
func (a *Actor) FullName() string {
	if a == nil {
		return ""
	}
	return a.FirstName + " " + a.LastName
}

// String returns a string representation of the actor for logging
func (a *Actor) String() string {
	if a == nil {
		return "system"
	}
	return fmt.Sprintf("%s (%s)", a.FullName(), a.Email)
}

// contextKey is the type for context keys to avoid collisions
type contextKey string

const actorContextKey contextKey = "actor"

// FromContext retrieves the Actor from the context.
// Returns nil if no actor is present (e.g., system operations).
func FromContext(ctx context.Context) *Actor {
	if ctx == nil {
		return nil
	}
	actor, ok := ctx.Value(actorContextKey).(*Actor)
	if !ok {
		return nil
	}
	return actor
}

// WithActor returns a new context with the Actor attached.
func WithActor(ctx context.Context, a *Actor) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, actorContextKey, a)
}

// MustFromContext retrieves the Actor from the context.
// Panics if no actor is present. Use only when actor is guaranteed to exist.
func MustFromContext(ctx context.Context) *Actor {
	actor := FromContext(ctx)
	if actor == nil {
		panic("actor not found in context")
	}
	return actor
}

// SystemActor returns an Actor representing the system itself.
// Use this for background jobs, scheduled tasks, and system-initiated operations.
func SystemActor() *Actor {
	return &Actor{
		ID:        "00000000-0000-0000-0000-000000000000",
		FirstName: "System",
		LastName:  "",
		Email:     "system@medflow.local",
	}
}

// IsSystem returns true if the actor represents the system.
func (a *Actor) IsSystem() bool {
	if a == nil {
		return true
	}
	return a.ID == "00000000-0000-0000-0000-000000000000"
}

// UserCache represents the cached user data stored in non-user services
// for event-synced user information.
type UserCache struct {
	UserID    string `json:"user_id" db:"user_id"`
	FirstName string `json:"first_name" db:"first_name"`
	LastName  string `json:"last_name" db:"last_name"`
	Email     string `json:"email" db:"email"`
	RoleName  string `json:"role_name" db:"role_name"`
	TenantID  string `json:"tenant_id" db:"tenant_id"`
}

// ToActor converts a UserCache entry to an Actor.
func (uc *UserCache) ToActor() *Actor {
	if uc == nil {
		return nil
	}
	return &Actor{
		ID:        uc.UserID,
		FirstName: uc.FirstName,
		LastName:  uc.LastName,
		Email:     uc.Email,
		TenantID:  uc.TenantID,
		RoleName:  uc.RoleName,
	}
}

// FullName returns the cached user's full name.
func (uc *UserCache) FullName() string {
	if uc == nil {
		return ""
	}
	return uc.FirstName + " " + uc.LastName
}
