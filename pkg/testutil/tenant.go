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
	ID         string
	Name       string
	Slug       string
	SchemaName string
}

// TenantManager manages test tenant schemas
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

// CreateTenant creates a new isolated tenant schema for testing.
// Each test can have its own tenant to ensure complete isolation.
//
// Usage:
//
//	tm := testutil.NewTenantManager(db)
//	tenant := tm.CreateTenant(ctx, "test-clinic")
//	ctx = testutil.WithTestTenant(ctx, tenant)
//
//	// Now all repository operations will use this tenant's schema
//	user, err := userRepo.GetByID(ctx, userID)
func (tm *TenantManager) CreateTenant(ctx context.Context, name string) (*TestTenant, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	id := uuid.New().String()
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	schemaName := fmt.Sprintf("tenant_%s", strings.ReplaceAll(slug, "-", "_"))

	// Create schema
	_, err := tm.db.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName))
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant schema: %w", err)
	}

	// Register tenant in public.tenants
	_, err = tm.db.ExecContext(ctx, `
		INSERT INTO public.tenants (id, name, slug, schema_name, subscription_status)
		VALUES ($1, $2, $3, $4, 'active')
		ON CONFLICT (slug) DO NOTHING
	`, id, name, slug, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to register tenant: %w", err)
	}

	t := TestTenant{
		ID:         id,
		Name:       name,
		Slug:       slug,
		SchemaName: schemaName,
	}

	tm.tenants = append(tm.tenants, t)
	return &t, nil
}

// CreateTenantWithMigrations creates a tenant and applies the given migrations
func (tm *TenantManager) CreateTenantWithMigrations(ctx context.Context, name string, migrations []string) (*TestTenant, error) {
	t, err := tm.CreateTenant(ctx, name)
	if err != nil {
		return nil, err
	}

	// Set search_path and apply migrations
	for _, migration := range migrations {
		_, err = tm.db.ExecContext(ctx, fmt.Sprintf("SET search_path TO %s, public", t.SchemaName))
		if err != nil {
			return nil, fmt.Errorf("failed to set search_path: %w", err)
		}

		_, err = tm.db.ExecContext(ctx, migration)
		if err != nil {
			return nil, fmt.Errorf("failed to apply migration: %w", err)
		}
	}

	// Reset search_path
	_, err = tm.db.ExecContext(ctx, "SET search_path TO public")
	if err != nil {
		return nil, fmt.Errorf("failed to reset search_path: %w", err)
	}

	return t, nil
}

// DropTenant removes a tenant schema completely
func (tm *TenantManager) DropTenant(ctx context.Context, t *TestTenant) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Drop schema with CASCADE (removes all objects)
	_, err := tm.db.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", t.SchemaName))
	if err != nil {
		return fmt.Errorf("failed to drop tenant schema: %w", err)
	}

	// Remove from tenants table
	_, err = tm.db.ExecContext(ctx, "DELETE FROM public.tenants WHERE id = $1", t.ID)
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

// Cleanup drops all tenant schemas created by this manager.
// Call this in TestMain or test cleanup.
func (tm *TenantManager) Cleanup(ctx context.Context) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	var lastErr error
	for _, t := range tm.tenants {
		_, err := tm.db.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", t.SchemaName))
		if err != nil {
			lastErr = err
		}
		_, err = tm.db.ExecContext(ctx, "DELETE FROM public.tenants WHERE id = $1", t.ID)
		if err != nil {
			lastErr = err
		}
	}

	tm.tenants = make([]TestTenant, 0)
	return lastErr
}

// WithTestTenant creates a context with tenant information for testing.
// This is the primary way to set up tenant context in tests.
func WithTestTenant(ctx context.Context, t *TestTenant) context.Context {
	return tenant.WithTenantContext(ctx, t.ID, t.Slug, t.SchemaName)
}

// WithTestTenantValues creates a context with custom tenant values.
// Useful for testing error cases or edge conditions.
func WithTestTenantValues(ctx context.Context, id, slug, schema string) context.Context {
	return tenant.WithTenantContext(ctx, id, slug, schema)
}

// TestTenantContext creates a context with a fake tenant for simple unit tests
// that don't need actual database isolation.
func TestTenantContext() context.Context {
	return tenant.WithTenantContext(
		context.Background(),
		"test-tenant-id",
		"test-tenant",
		"tenant_test",
	)
}

// UserMigrations returns the standard user service migrations for tests
func UserMigrations() []string {
	return []string{
		// Roles table
		`CREATE TABLE IF NOT EXISTS roles (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(50) UNIQUE NOT NULL,
			display_name VARCHAR(100) NOT NULL,
			display_name_de VARCHAR(100),
			description TEXT,
			level INT NOT NULL DEFAULT 0,
			is_manager BOOLEAN DEFAULT FALSE,
			can_receive_delegation BOOLEAN DEFAULT FALSE,
			permissions JSONB DEFAULT '[]',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			first_name VARCHAR(100),
			last_name VARCHAR(100),
			avatar_url TEXT,
			status VARCHAR(20) DEFAULT 'active',
			created_by UUID,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_login_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ
		)`,

		// User-roles junction
		`CREATE TABLE IF NOT EXISTS user_roles (
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (user_id, role_id)
		)`,

		// Permissions lookup
		`CREATE TABLE IF NOT EXISTS permissions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(100) UNIQUE NOT NULL,
			description TEXT,
			category VARCHAR(50),
			is_admin_only BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Permission overrides
		`CREATE TABLE IF NOT EXISTS permission_overrides (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
			granted BOOLEAN NOT NULL,
			granted_by UUID,
			granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			reason TEXT,
			expires_at TIMESTAMPTZ,
			UNIQUE(user_id, permission_id)
		)`,

		// Access giver scope
		`CREATE TABLE IF NOT EXISTS access_giver_scope (
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
			PRIMARY KEY (user_id, role_id)
		)`,

		// Audit logs
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			actor_id UUID,
			actor_name VARCHAR(255),
			action VARCHAR(100) NOT NULL,
			target_user_id UUID,
			target_user_name VARCHAR(255),
			resource_type VARCHAR(100),
			resource_id UUID,
			details JSONB,
			ip_address INET,
			user_agent TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Seed default roles
		`INSERT INTO roles (name, display_name, display_name_de, level, is_manager, permissions) VALUES
			('admin', 'Administrator', 'Administrator', 100, true, '["*"]'),
			('manager', 'Manager', 'Manager', 80, true, '["users:read", "users:write", "inventory:read", "inventory:write"]'),
			('staff', 'Staff', 'Mitarbeiter', 50, false, '["inventory:read", "inventory:write"]'),
			('viewer', 'Viewer', 'Betrachter', 10, false, '["inventory:read"]')
		ON CONFLICT (name) DO NOTHING`,
	}
}

// InventoryMigrations returns the standard inventory service migrations for tests
func InventoryMigrations() []string {
	return []string{
		// Storage rooms
		`CREATE TABLE IF NOT EXISTS storage_rooms (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			description TEXT,
			location VARCHAR(255),
			temperature_min DECIMAL(5,2),
			temperature_max DECIMAL(5,2),
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		)`,

		// Inventory items
		`CREATE TABLE IF NOT EXISTS inventory_items (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			sku VARCHAR(100) UNIQUE,
			category VARCHAR(100),
			description TEXT,
			unit VARCHAR(50),
			min_quantity INT DEFAULT 0,
			max_quantity INT,
			reorder_point INT,
			storage_room_id UUID REFERENCES storage_rooms(id),
			is_active BOOLEAN DEFAULT TRUE,
			created_by UUID,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		)`,

		// Inventory batches
		`CREATE TABLE IF NOT EXISTS inventory_batches (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			item_id UUID REFERENCES inventory_items(id) ON DELETE CASCADE,
			batch_number VARCHAR(100),
			quantity INT NOT NULL DEFAULT 0,
			unit_price DECIMAL(10,2),
			expiry_date DATE,
			received_date DATE,
			supplier VARCHAR(255),
			notes TEXT,
			created_by UUID,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Inventory transactions
		`CREATE TABLE IF NOT EXISTS inventory_transactions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			item_id UUID REFERENCES inventory_items(id),
			batch_id UUID REFERENCES inventory_batches(id),
			type VARCHAR(50) NOT NULL,
			quantity INT NOT NULL,
			previous_quantity INT,
			new_quantity INT,
			reason TEXT,
			performed_by UUID,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	}
}

// TimeTrackingMigrations returns the time tracking migrations for tests
// These should be applied AFTER StaffMigrations (depends on employees table)
func TimeTrackingMigrations() []string {
	return []string{
		// Time entries table (matches migrations/staff/tenant/000003)
		`CREATE TABLE IF NOT EXISTS time_entries (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
			entry_date DATE NOT NULL,
			clock_in TIMESTAMPTZ NOT NULL,
			clock_out TIMESTAMPTZ,
			total_work_minutes INTEGER NOT NULL DEFAULT 0,
			total_break_minutes INTEGER NOT NULL DEFAULT 0,
			notes TEXT,
			is_manual_entry BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ,
			created_by UUID,
			updated_by UUID
		)`,

		// Time breaks table (matches migrations/staff/tenant/000003)
		`CREATE TABLE IF NOT EXISTS time_breaks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			time_entry_id UUID NOT NULL REFERENCES time_entries(id) ON DELETE CASCADE,
			start_time TIMESTAMPTZ NOT NULL,
			end_time TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Time corrections table (matches migrations/staff/tenant/000003)
		`CREATE TABLE IF NOT EXISTS time_corrections (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
			time_entry_id UUID REFERENCES time_entries(id) ON DELETE SET NULL,
			correction_date DATE NOT NULL,
			original_clock_in TIMESTAMPTZ,
			original_clock_out TIMESTAMPTZ,
			corrected_clock_in TIMESTAMPTZ,
			corrected_clock_out TIMESTAMPTZ,
			reason TEXT NOT NULL,
			corrected_by UUID NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		)`,

		// Shift templates table
		`CREATE TABLE IF NOT EXISTS shift_templates (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(100) NOT NULL,
			description TEXT,
			start_time TIME NOT NULL,
			end_time TIME NOT NULL,
			break_duration_minutes INT DEFAULT 30,
			shift_type VARCHAR(20) NOT NULL DEFAULT 'regular',
			color VARCHAR(7) DEFAULT '#22c55e',
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ,
			created_by UUID
		)`,

		// Shift assignments table
		`CREATE TABLE IF NOT EXISTS shift_assignments (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
			shift_template_id UUID REFERENCES shift_templates(id) ON DELETE SET NULL,
			shift_date DATE NOT NULL,
			start_time TIME NOT NULL,
			end_time TIME NOT NULL,
			break_duration_minutes INT DEFAULT 30,
			shift_type VARCHAR(20) NOT NULL DEFAULT 'regular',
			status VARCHAR(20) NOT NULL DEFAULT 'scheduled',
			has_conflict BOOLEAN DEFAULT FALSE,
			conflict_reason TEXT,
			notes TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ,
			created_by UUID,
			updated_by UUID
		)`,

		// Absences table
		`CREATE TABLE IF NOT EXISTS absences (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
			start_date DATE NOT NULL,
			end_date DATE NOT NULL,
			absence_type VARCHAR(30) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			reviewed_by UUID,
			reviewed_at TIMESTAMPTZ,
			rejection_reason TEXT,
			vacation_days_used DECIMAL(4,1),
			employee_note TEXT,
			manager_note TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ,
			created_by UUID,
			updated_by UUID
		)`,

		// Vacation balances table
		`CREATE TABLE IF NOT EXISTS vacation_balances (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
			year INT NOT NULL,
			annual_entitlement DECIMAL(4,1) NOT NULL,
			carryover_from_previous DECIMAL(4,1) DEFAULT 0,
			additional_granted DECIMAL(4,1) DEFAULT 0,
			taken DECIMAL(4,1) DEFAULT 0,
			planned DECIMAL(4,1) DEFAULT 0,
			pending DECIMAL(4,1) DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT vacation_balances_unique UNIQUE (employee_id, year)
		)`,

		// ArbZG compliance log table
		`CREATE TABLE IF NOT EXISTS arbzg_compliance_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
			time_entry_id UUID REFERENCES time_entries(id) ON DELETE SET NULL,
			violation_date DATE NOT NULL,
			violation_type VARCHAR(50) NOT NULL,
			severity VARCHAR(20) NOT NULL DEFAULT 'warning',
			description TEXT NOT NULL,
			details JSONB,
			acknowledged_by UUID,
			acknowledged_at TIMESTAMPTZ,
			resolution_note TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Indexes (matches migrations/staff/tenant/000003)
		`CREATE INDEX IF NOT EXISTS idx_time_entries_employee_date ON time_entries(employee_id, entry_date) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_time_entries_date ON time_entries(entry_date) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_time_entries_employee ON time_entries(employee_id) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_time_entries_active ON time_entries(employee_id, clock_out) WHERE deleted_at IS NULL AND clock_out IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_time_breaks_entry ON time_breaks(time_entry_id)`,
		`CREATE INDEX IF NOT EXISTS idx_time_breaks_active ON time_breaks(time_entry_id, end_time) WHERE end_time IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_time_corrections_employee ON time_corrections(employee_id, correction_date) WHERE deleted_at IS NULL`,
	}
}

// StaffMigrations returns the standard staff service migrations for tests
// Matches the actual schema from migrations/staff/tenant/000001_create_tenant_schema.up.sql
func StaffMigrations() []string {
	return []string{
		// Employees table
		`CREATE TABLE IF NOT EXISTS employees (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID,
			first_name VARCHAR(100) NOT NULL,
			last_name VARCHAR(100) NOT NULL,
			date_of_birth DATE,
			gender VARCHAR(20),
			nationality VARCHAR(100),
			email VARCHAR(255),
			phone VARCHAR(50),
			mobile VARCHAR(50),
			employee_number VARCHAR(50) UNIQUE,
			job_title VARCHAR(255),
			department VARCHAR(100),
			employment_type VARCHAR(50) NOT NULL DEFAULT 'full_time',
			hire_date DATE NOT NULL,
			termination_date DATE,
			probation_end_date DATE,
			status VARCHAR(50) NOT NULL DEFAULT 'active',
			avatar_url TEXT,
			notes TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ,
			created_by UUID,
			updated_by UUID
		)`,

		// Employee addresses
		`CREATE TABLE IF NOT EXISTS employee_addresses (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
			address_type VARCHAR(50) NOT NULL DEFAULT 'home',
			street VARCHAR(255) NOT NULL,
			house_number VARCHAR(20),
			address_line2 VARCHAR(255),
			postal_code VARCHAR(20) NOT NULL,
			city VARCHAR(100) NOT NULL,
			state VARCHAR(100),
			country VARCHAR(100) NOT NULL DEFAULT 'Germany',
			is_primary BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Employee contacts (emergency contacts)
		`CREATE TABLE IF NOT EXISTS employee_contacts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
			contact_type VARCHAR(50) NOT NULL DEFAULT 'emergency',
			name VARCHAR(255) NOT NULL,
			relationship VARCHAR(100),
			phone VARCHAR(50) NOT NULL,
			email VARCHAR(255),
			is_primary BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Employee financials
		`CREATE TABLE IF NOT EXISTS employee_financials (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
			iban VARCHAR(34),
			bic VARCHAR(11),
			bank_name VARCHAR(255),
			account_holder VARCHAR(255),
			tax_id VARCHAR(20),
			tax_class VARCHAR(10),
			church_tax BOOLEAN DEFAULT FALSE,
			child_allowance DECIMAL(5,2) DEFAULT 0,
			salary_type VARCHAR(50) DEFAULT 'monthly',
			base_salary_cents INTEGER,
			currency VARCHAR(3) DEFAULT 'EUR',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT employee_financials_unique UNIQUE (employee_id)
		)`,

		// Employee documents
		`CREATE TABLE IF NOT EXISTS employee_documents (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
			document_type VARCHAR(50) NOT NULL,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			file_path TEXT NOT NULL,
			file_size_bytes INTEGER,
			mime_type VARCHAR(100),
			issue_date DATE,
			expiry_date DATE,
			status VARCHAR(50) NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ,
			uploaded_by UUID
		)`,

		// Employee files (for uploads)
		`CREATE TABLE IF NOT EXISTS employee_files (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			file_type VARCHAR(100),
			file_path TEXT NOT NULL,
			file_size INTEGER,
			mime_type VARCHAR(100),
			category VARCHAR(100),
			uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			uploaded_by UUID
		)`,
	}
}
