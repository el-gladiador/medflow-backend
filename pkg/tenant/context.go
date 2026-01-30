package tenant

import (
	"context"
	"errors"
)

// contextKey is a private type for context keys to prevent collisions
type contextKey string

const (
	tenantIDKey     contextKey = "tenant_id"
	tenantSlugKey   contextKey = "tenant_slug"
	tenantSchemaKey contextKey = "tenant_schema"
)

var (
	// ErrNoTenantInContext is returned when tenant context is missing
	ErrNoTenantInContext = errors.New("no tenant in context")
)

// WithTenantContext adds all tenant information to the context
// This should be called by middleware after extracting tenant from JWT
func WithTenantContext(ctx context.Context, id, slug, schema string) context.Context {
	ctx = context.WithValue(ctx, tenantIDKey, id)
	ctx = context.WithValue(ctx, tenantSlugKey, slug)
	ctx = context.WithValue(ctx, tenantSchemaKey, schema)
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

// WithTenantSchema adds only tenant schema to context
func WithTenantSchema(ctx context.Context, tenantSchema string) context.Context {
	return context.WithValue(ctx, tenantSchemaKey, tenantSchema)
}

// TenantID extracts tenant ID from context
// Returns ErrNoTenantInContext if tenant ID is not found
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

// TenantSchema extracts tenant schema name from context
// Returns ErrNoTenantInContext if tenant schema is not found
// This is the most important function - used by repositories to set search_path
func TenantSchema(ctx context.Context) (string, error) {
	schema, ok := ctx.Value(tenantSchemaKey).(string)
	if !ok || schema == "" {
		return "", ErrNoTenantInContext
	}
	return schema, nil
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

// MustTenantSchema extracts tenant schema from context and panics if not found
// Use only in cases where missing tenant is a programming error
func MustTenantSchema(ctx context.Context) string {
	schema, err := TenantSchema(ctx)
	if err != nil {
		panic("tenant schema not found in context")
	}
	return schema
}
