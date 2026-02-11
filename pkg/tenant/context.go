package tenant

import (
	"context"
	"errors"
)

// contextKey is a private type for context keys to prevent collisions
type contextKey string

const (
	tenantIDKey   contextKey = "tenant_id"
	tenantSlugKey contextKey = "tenant_slug"
)

var (
	// ErrNoTenantInContext is returned when tenant context is missing
	ErrNoTenantInContext = errors.New("no tenant in context")
)

// WithTenantContext adds tenant information to the context.
// Used by middleware after extracting tenant from JWT.
func WithTenantContext(ctx context.Context, id, slug string) context.Context {
	ctx = context.WithValue(ctx, tenantIDKey, id)
	ctx = context.WithValue(ctx, tenantSlugKey, slug)
	return ctx
}

// WithTenantID adds only tenant ID to context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// WithTenantSlug adds only tenant slug to context
func WithTenantSlug(ctx context.Context, tenantSlug string) context.Context {
	return context.WithValue(ctx, tenantSlugKey, tenantSlug)
}

// TenantID extracts tenant ID from context.
// This is the primary function for RLS-based multi-tenancy — used by all repositories
// to set app.current_tenant for RLS policy evaluation.
func TenantID(ctx context.Context) (string, error) {
	id, ok := ctx.Value(tenantIDKey).(string)
	if !ok || id == "" {
		return "", ErrNoTenantInContext
	}
	return id, nil
}

// TenantSlug extracts tenant slug from context
// Returns ErrNoTenantInContext if tenant slug is not found
func TenantSlug(ctx context.Context) (string, error) {
	slug, ok := ctx.Value(tenantSlugKey).(string)
	if !ok || slug == "" {
		return "", ErrNoTenantInContext
	}
	return slug, nil
}

// MustTenantID extracts tenant ID from context and panics if not found
// Use only in cases where missing tenant is a programming error
func MustTenantID(ctx context.Context) string {
	id, err := TenantID(ctx)
	if err != nil {
		panic("tenant ID not found in context")
	}
	return id
}
