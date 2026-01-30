package database

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type txKey struct{}

// WithTenantSchema executes a function with search_path set to the tenant's schema.
// This is the KEY isolation mechanism for schema-per-tenant architecture.
//
// Usage in repositories:
//
//	err := r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
//	    query := `SELECT * FROM employees WHERE id = $1`
//	    return r.db.GetContext(ctx, &emp, query, id)
//	})
//
// How it works:
//  1. Starts a transaction
//  2. Sets "SET LOCAL search_path TO tenant_xxx, public"
//  3. Stores tx in context
//  4. Executes user function
//  5. Commits transaction (auto-cleanup of search_path)
//
// Why this is secure:
// - SET LOCAL is scoped to transaction (automatic cleanup)
// - Even with connection pooling, next request gets fresh search_path
// - PostgreSQL engine enforces schema isolation
// - If table doesn't exist in schema, query fails (prevents cross-tenant access)
func (db *DB) WithTenantSchema(ctx context.Context, schemaName string, fn func(context.Context) error) error {
	return db.Transaction(ctx, func(tx *sqlx.Tx) error {
		// Set search_path for this transaction
		// SET LOCAL ensures it's only valid for this transaction
		// Including 'public' allows access to shared functions (e.g., update_updated_at)
		query := fmt.Sprintf("SET LOCAL search_path TO %s, public", schemaName)

		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to set search_path to %s: %w", schemaName, err)
		}

		// Store transaction in context so DB methods can use it
		txCtx := context.WithValue(ctx, txKey{}, tx)

		// Execute user function with tenant-scoped connection
		// All queries inside fn will execute in the tenant's schema
		return fn(txCtx)
	})
}

// getTx extracts transaction from context if present
func (db *DB) getTx(ctx context.Context) *sqlx.Tx {
	if tx, ok := ctx.Value(txKey{}).(*sqlx.Tx); ok {
		return tx
	}
	return nil
}

// WithTenantSchemaReadOnly executes a read-only function with search_path set to tenant's schema.
// This is an optimization for read-only queries that don't need transaction overhead.
//
// IMPORTANT: Only use this for SELECT queries where you don't need ACID guarantees.
// For any writes (INSERT/UPDATE/DELETE), use WithTenantSchema.
func (db *DB) WithTenantSchemaReadOnly(ctx context.Context, schemaName string, fn func(context.Context) error) error {
	// For read-only, we still use a transaction to ensure search_path is scoped
	// PostgreSQL transactions for reads are lightweight
	return db.WithTenantSchema(ctx, schemaName, fn)
}
