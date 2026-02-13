package testutil

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// TestTenant represents a tenant created for testing
type TestTenant struct {
	ID   string
	Name string
	Slug string
}

// TenantManager manages test tenants for RLS-based isolation
type TenantManager struct {
	db      *sqlx.DB
	tenants []TestTenant
	mu      sync.Mutex
}

// NewTenantManager creates a new tenant manager for tests
func NewTenantManager(db *sqlx.DB) *TenantManager {
	return &TenantManager{
		db:      db,
		tenants: make([]TestTenant, 0),
	}
}

// CreateTenant creates a new tenant for testing.
// With RLS, this just inserts a row into public.tenants (no schema creation needed).
func (tm *TenantManager) CreateTenant(ctx context.Context, name string) (*TestTenant, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	id := uuid.New().String()
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	// Register tenant in public.tenants
	_, err := tm.db.ExecContext(ctx, `
		INSERT INTO public.tenants (id, name, slug, subscription_status)
		VALUES ($1, $2, $3, 'active')
		ON CONFLICT (slug) DO NOTHING
	`, id, name, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to register tenant: %w", err)
	}

	t := TestTenant{
		ID:   id,
		Name: name,
		Slug: slug,
	}

	tm.tenants = append(tm.tenants, t)
	return &t, nil
}

// CreateTenantWithMigrations creates a tenant and applies the given migrations.
// With RLS, migrations are applied once at suite setup, so this just creates the tenant
// and optionally runs any extra migration SQL (for test-specific setup).
func (tm *TenantManager) CreateTenantWithMigrations(ctx context.Context, name string, migrations []string) (*TestTenant, error) {
	t, err := tm.CreateTenant(ctx, name)
	if err != nil {
		return nil, err
	}

	// Run any additional migration statements if provided
	// These are now expected to be tenant-specific INSERT statements, not DDL
	for _, migration := range migrations {
		_, err = tm.db.ExecContext(ctx, migration)
		if err != nil {
			return nil, fmt.Errorf("failed to apply migration: %w", err)
		}
	}

	return t, nil
}

// DropTenant removes all data for a tenant.
// With RLS, this deletes rows from all tables where tenant_id matches.
func (tm *TenantManager) DropTenant(ctx context.Context, t *TestTenant) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Delete all tenant data across schemas
	schemas := []struct {
		schema string
		tables []string
	}{
		{"inventory", []string{"temperature_readings", "device_incidents", "device_trainings", "device_inspections", "item_documents", "hazardous_substance_details", "inventory_alerts", "stock_adjustments", "inventory_batches", "inventory_items", "storage_shelves", "storage_cabinets", "storage_rooms", "user_cache"}},
		{"staff", []string{"document_processing_audit", "time_correction_requests", "compliance_alerts", "compliance_violations", "compliance_settings", "arbzg_compliance_log", "time_corrections", "time_breaks", "time_entries", "vacation_balances", "absences", "shift_assignments", "shift_templates", "employee_files", "employee_documents", "employee_social_insurance", "employee_financials", "employee_contacts", "employee_addresses", "employees", "user_cache"}},
		{"users", []string{"audit_logs", "user_roles", "sessions", "roles", "users"}},
	}

	for _, s := range schemas {
		for _, table := range s.tables {
			_, err := tm.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s.%s WHERE tenant_id = $1", s.schema, table), t.ID)
			if err != nil {
				// Ignore errors for tables that don't exist in the test DB
				continue
			}
		}
	}

	// Remove from public tables
	_, _ = tm.db.ExecContext(ctx, "DELETE FROM public.user_tenant_lookup WHERE tenant_id = $1", t.ID)
	_, _ = tm.db.ExecContext(ctx, "DELETE FROM public.tenant_audit_log WHERE tenant_id = $1", t.ID)
	_, err := tm.db.ExecContext(ctx, "DELETE FROM public.tenants WHERE id = $1", t.ID)
	if err != nil {
		return fmt.Errorf("failed to delete tenant record: %w", err)
	}

	// Remove from tracked tenants
	for i, tracked := range tm.tenants {
		if tracked.ID == t.ID {
			tm.tenants = append(tm.tenants[:i], tm.tenants[i+1:]...)
			break
		}
	}

	return nil
}

// Cleanup drops all tenants created by this manager.
func (tm *TenantManager) Cleanup(ctx context.Context) error {
	tm.mu.Lock()
	tenantsToClean := make([]TestTenant, len(tm.tenants))
	copy(tenantsToClean, tm.tenants)
	tm.mu.Unlock()

	var lastErr error
	for _, t := range tenantsToClean {
		if err := tm.DropTenant(ctx, &t); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// WithTestTenant creates a context with tenant information for testing.
func WithTestTenant(ctx context.Context, t *TestTenant) context.Context {
	return tenant.WithTenantContext(ctx, t.ID, t.Slug)
}

// WithTestTenantValues creates a context with custom tenant values.
func WithTestTenantValues(ctx context.Context, id, slug string) context.Context {
	return tenant.WithTenantContext(ctx, id, slug)
}

// TestTenantContext creates a context with a fake tenant for simple unit tests.
func TestTenantContext() context.Context {
	return tenant.WithTenantContext(
		context.Background(),
		"test-tenant-id",
		"test-tenant",
	)
}

// UserMigrations returns seed data migrations for user service tests.
// With RLS, table DDL is created once at suite setup; these just seed roles.
func UserMigrations() []string {
	return []string{}
}

// InventoryMigrations returns seed data migrations for inventory service tests.
func InventoryMigrations() []string {
	return []string{}
}

// TimeTrackingMigrations returns seed data migrations for time tracking tests.
func TimeTrackingMigrations() []string {
	return []string{}
}

// StaffMigrations returns seed data migrations for staff service tests.
func StaffMigrations() []string {
	return []string{}
}
