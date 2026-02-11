package database

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type txKey struct{}

// WithTenantRLS executes a function with RLS-based tenant isolation.
// This is the KEY isolation mechanism for RLS-based pooled multi-tenancy.
//
// Usage in repositories:
//
//	tenantID, err := tenant.TenantID(ctx)
//	if err != nil { return err }
//	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
//	    return r.db.GetContext(ctx, &emp, "SELECT * FROM employees WHERE id = $1", id)
//	})
//
// How it works:
//  1. Starts a transaction
//  2. Sets "SET LOCAL search_path TO <service_schema>, public" (from db.searchPath)
//  3. Sets "SET LOCAL app.current_tenant = '<tenant-uuid>'"
//  4. RLS policies filter rows automatically: USING (tenant_id = current_setting('app.current_tenant')::uuid)
//  5. Commits transaction (auto-cleanup of session variables)
//
// Why this is secure:
//   - SET LOCAL is scoped to transaction (automatic cleanup)
//   - Even with connection pooling (PgBouncer), next request gets clean state
//   - RLS policies are enforced by PostgreSQL engine — app code can't bypass them
//   - WITH CHECK prevents inserting rows for wrong tenant
func (db *DB) WithTenantRLS(ctx context.Context, tenantID string, fn func(context.Context) error) error {
	return db.Transaction(ctx, func(tx *sqlx.Tx) error {
		// Set search_path for the service schema
		searchPath := db.searchPath
		if searchPath == "" {
			searchPath = "public"
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL search_path TO %s", searchPath)); err != nil {
			return fmt.Errorf("failed to set search_path to %s: %w", searchPath, err)
		}

		// Set tenant context for RLS policies
		// This is what RLS policies check: current_setting('app.current_tenant')::uuid
		// NOTE: SET LOCAL doesn't support parameterized queries ($1), must use fmt.Sprintf.
		// This is safe because tenantID is a UUID validated upstream (not user input).
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tenantID)); err != nil {
			return fmt.Errorf("failed to set app.current_tenant to %s: %w", tenantID, err)
		}

		// Store transaction in context so DB methods can use it
		txCtx := context.WithValue(ctx, txKey{}, tx)

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
