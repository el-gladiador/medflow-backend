// Package testutil provides testing utilities for MedFlow backend services.
// It includes testcontainers for PostgreSQL, tenant context helpers,
// mock factories, and common test fixtures.
package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer wraps a testcontainers PostgreSQL instance
type PostgresContainer struct {
	*postgres.PostgresContainer
	DSN        string
	AppRoleDSN string // DSN for medflow_app (non-superuser, RLS enforced)
}

// PostgresContainerConfig configures the test PostgreSQL container
type PostgresContainerConfig struct {
	Database string
	Username string
	Password string
	Image    string // Optional: defaults to postgres:15-alpine
}

// DefaultPostgresConfig returns sensible defaults for test containers
func DefaultPostgresConfig() PostgresContainerConfig {
	return PostgresContainerConfig{
		Database: "medflow_test",
		Username: "test",
		Password: "test",
		Image:    "postgres:15-alpine",
	}
}

// NewPostgresContainer creates a new PostgreSQL test container.
// The container is automatically configured for testing with RLS-based multi-tenancy.
//
// Usage:
//
//	func TestMain(m *testing.M) {
//	    ctx := context.Background()
//	    container, err := testutil.NewPostgresContainer(ctx, testutil.DefaultPostgresConfig())
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer container.Terminate(ctx)
//
//	    // Run tests
//	    code := m.Run()
//	    os.Exit(code)
//	}
func NewPostgresContainer(ctx context.Context, cfg PostgresContainerConfig) (*PostgresContainer, error) {
	if cfg.Image == "" {
		cfg.Image = "postgres:15-alpine"
	}
	if cfg.Database == "" {
		cfg.Database = "medflow_test"
	}
	if cfg.Username == "" {
		cfg.Username = "test"
	}
	if cfg.Password == "" {
		cfg.Password = "test"
	}

	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage(cfg.Image),
		postgres.WithDatabase(cfg.Database),
		postgres.WithUsername(cfg.Username),
		postgres.WithPassword(cfg.Password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	return &PostgresContainer{
		PostgresContainer: container,
		DSN:               dsn,
	}, nil
}

// Connect returns a sqlx.DB connection to the container
func (c *PostgresContainer) Connect(ctx context.Context) (*sqlx.DB, error) {
	db, err := sqlx.ConnectContext(ctx, "postgres", c.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}
	return db, nil
}

// Terminate stops and removes the container
func (c *PostgresContainer) Terminate(ctx context.Context) error {
	return c.PostgresContainer.Terminate(ctx)
}

// CreateAppRole creates the medflow_app role (non-superuser) and applies FORCE RLS.
// This mirrors migration 000008+000009: services connect as medflow_app at runtime.
// Call this after CreatePublicSchema and CreateServiceSchemas.
func (c *PostgresContainer) CreateAppRole(ctx context.Context, db *sqlx.DB) error {
	sql := `
		-- Create non-superuser app role (mirrors init-db.sql + migration 000008)
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'medflow_app') THEN
				CREATE ROLE medflow_app WITH LOGIN PASSWORD 'test' NOSUPERUSER NOCREATEDB NOCREATEROLE;
			END IF;
		END
		$$;

		-- Grant permissions on all schemas
		GRANT CONNECT ON DATABASE medflow_test TO medflow_app;
		GRANT USAGE ON SCHEMA public TO medflow_app;
		GRANT USAGE ON SCHEMA users TO medflow_app;
		GRANT USAGE ON SCHEMA staff TO medflow_app;
		GRANT USAGE ON SCHEMA inventory TO medflow_app;

		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO medflow_app;
		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA users TO medflow_app;
		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA staff TO medflow_app;
		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA inventory TO medflow_app;

		GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO medflow_app;
		GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA users TO medflow_app;
		GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA staff TO medflow_app;
		GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA inventory TO medflow_app;

		-- Default privileges for future tables
		ALTER DEFAULT PRIVILEGES IN SCHEMA public    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO medflow_app;
		ALTER DEFAULT PRIVILEGES IN SCHEMA users     GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO medflow_app;
		ALTER DEFAULT PRIVILEGES IN SCHEMA staff     GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO medflow_app;
		ALTER DEFAULT PRIVILEGES IN SCHEMA inventory GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO medflow_app;

		-- Grant execute on functions
		GRANT EXECUTE ON FUNCTION public.update_updated_at() TO medflow_app;

		-- FORCE ROW LEVEL SECURITY on all tenant-scoped tables
		-- users schema
		ALTER TABLE users.roles FORCE ROW LEVEL SECURITY;
		ALTER TABLE users.users FORCE ROW LEVEL SECURITY;
		ALTER TABLE users.user_roles FORCE ROW LEVEL SECURITY;
		ALTER TABLE users.audit_logs FORCE ROW LEVEL SECURITY;

		-- staff schema
		ALTER TABLE staff.employees FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.time_entries FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.time_breaks FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.shift_templates FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.shift_assignments FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.user_cache FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.time_corrections FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.employee_addresses FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.employee_contacts FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.employee_financials FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.employee_social_insurance FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.employee_documents FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.employee_files FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.absences FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.vacation_balances FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.compliance_settings FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.compliance_violations FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.arbzg_compliance_log FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.compliance_alerts FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.time_correction_requests FORCE ROW LEVEL SECURITY;
		ALTER TABLE staff.document_processing_audit FORCE ROW LEVEL SECURITY;

		-- inventory schema
		ALTER TABLE inventory.storage_rooms FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.storage_cabinets FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.storage_shelves FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.inventory_items FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.inventory_batches FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.stock_adjustments FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.inventory_alerts FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.hazardous_substance_details FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.item_documents FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.user_cache FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.device_inspections FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.device_trainings FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.device_incidents FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.temperature_readings FORCE ROW LEVEL SECURITY;
	`

	if _, err := db.ExecContext(ctx, sql); err != nil {
		return fmt.Errorf("failed to create app role and apply FORCE RLS: %w", err)
	}

	// Build the app role DSN by replacing the user in the superuser DSN
	c.AppRoleDSN = replaceUserInDSN(c.DSN, "medflow_app", "test")

	return nil
}

// replaceUserInDSN replaces the user:password in a postgres DSN string.
// Handles both URL format (postgres://user:pass@host) and key=value format.
func replaceUserInDSN(dsn, newUser, newPassword string) string {
	// testcontainers returns URL format: postgres://user:pass@host:port/db?params
	if len(dsn) > 11 && dsn[:11] == "postgres://" {
		// Find the @ sign
		atIdx := -1
		for i := 11; i < len(dsn); i++ {
			if dsn[i] == '@' {
				atIdx = i
				break
			}
		}
		if atIdx > 0 {
			return fmt.Sprintf("postgres://%s:%s@%s", newUser, newPassword, dsn[atIdx+1:])
		}
	}
	// Fallback: return original DSN (shouldn't happen with testcontainers)
	return dsn
}

// CreatePublicSchema creates the public schema with tenants table and service schemas.
// This mirrors the Supabase migration setup with RLS-based multi-tenancy.
func (c *PostgresContainer) CreatePublicSchema(ctx context.Context, db *sqlx.DB) error {
	schema := `
		-- Create service schemas
		CREATE SCHEMA IF NOT EXISTS users;
		CREATE SCHEMA IF NOT EXISTS staff;
		CREATE SCHEMA IF NOT EXISTS inventory;

		-- Updated at trigger function
		CREATE OR REPLACE FUNCTION public.update_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		-- Tenants registry (public schema, NO RLS)
		CREATE TABLE IF NOT EXISTS public.tenants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(100) UNIQUE NOT NULL,
			subscription_status VARCHAR(50) DEFAULT 'trial',
			settings JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);

		-- User-tenant lookup (public schema, NO RLS)
		CREATE TABLE IF NOT EXISTS public.user_tenant_lookup (
			email VARCHAR(255) PRIMARY KEY,
			username VARCHAR(100),
			user_id UUID NOT NULL,
			tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
			tenant_slug VARCHAR(100) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- Tenant audit log (public schema, NO RLS)
		CREATE TABLE IF NOT EXISTS public.tenant_audit_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID REFERENCES public.tenants(id),
			action VARCHAR(100) NOT NULL,
			actor_id UUID,
			actor_name VARCHAR(255),
			details JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`

	_, err := db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to create public schema: %w", err)
	}

	return nil
}

// CreateServiceSchemas creates tables for specific service schemas.
// Call this after CreatePublicSchema to set up tables needed by specific services.
func (c *PostgresContainer) CreateServiceSchemas(ctx context.Context, db *sqlx.DB, schemas ...string) error {
	for _, s := range schemas {
		var ddl string
		switch s {
		case "users":
			ddl = usersSchemaSQL
		case "staff":
			ddl = staffSchemaSQL
		case "inventory":
			ddl = inventorySchemaSQL
		default:
			return fmt.Errorf("unknown schema: %s", s)
		}
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("failed to create %s schema tables: %w", s, err)
		}
	}
	return nil
}

// usersSchemaSQL creates the users schema tables with RLS policies
var usersSchemaSQL = `
	CREATE TABLE IF NOT EXISTS users.roles (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		name VARCHAR(50) NOT NULL,
		display_name VARCHAR(100) NOT NULL,
		display_name_de VARCHAR(100),
		description TEXT,
		is_system BOOLEAN DEFAULT false,
		is_default BOOLEAN DEFAULT false,
		is_manager BOOLEAN DEFAULT false,
		can_receive_delegation BOOLEAN DEFAULT false,
		level INTEGER DEFAULT 0,
		permissions JSONB DEFAULT '[]',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		UNIQUE(tenant_id, name)
	);
	ALTER TABLE users.roles ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON users.roles;
	CREATE POLICY tenant_isolation ON users.roles
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS users.users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		email VARCHAR(255) NOT NULL,
		username VARCHAR(100),
		password_hash TEXT NOT NULL,
		first_name VARCHAR(100) NOT NULL,
		last_name VARCHAR(100) NOT NULL,
		avatar_url TEXT,
		status VARCHAR(20) DEFAULT 'active',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		last_login_at TIMESTAMPTZ,
		UNIQUE(tenant_id, email)
	);
	ALTER TABLE users.users ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON users.users;
	CREATE POLICY tenant_isolation ON users.users
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS users.user_roles (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		user_id UUID NOT NULL REFERENCES users.users(id),
		role_id UUID NOT NULL REFERENCES users.roles(id),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(tenant_id, user_id, role_id)
	);
	ALTER TABLE users.user_roles ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON users.user_roles;
	CREATE POLICY tenant_isolation ON users.user_roles
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS users.audit_logs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		actor_id UUID,
		actor_name VARCHAR(255),
		action VARCHAR(100) NOT NULL,
		resource_type VARCHAR(100),
		resource_id UUID,
		target_user_id UUID,
		target_user_name VARCHAR(255),
		old_values JSONB,
		new_values JSONB,
		details JSONB DEFAULT '{}',
		ip_address VARCHAR(45),
		user_agent TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE users.audit_logs ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON users.audit_logs;
	CREATE POLICY tenant_isolation ON users.audit_logs
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
`

// staffSchemaSQL creates core staff schema tables with RLS policies.
// Must match production migrations (000004 + 000010 + 000012).
var staffSchemaSQL = `
	CREATE TABLE IF NOT EXISTS staff.employees (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		user_id UUID,
		employee_number VARCHAR(50),
		first_name VARCHAR(100) NOT NULL,
		last_name VARCHAR(100) NOT NULL,
		avatar_url TEXT,
		date_of_birth DATE,
		gender VARCHAR(20),
		nationality VARCHAR(100),
		birth_place VARCHAR(100),
		marital_status VARCHAR(50),
		job_title VARCHAR(255),
		department VARCHAR(100),
		employment_type VARCHAR(50) NOT NULL DEFAULT 'full_time',
		contract_type VARCHAR(50),
		hire_date DATE NOT NULL DEFAULT CURRENT_DATE,
		probation_end_date DATE,
		termination_date DATE,
		weekly_hours DECIMAL(5,2),
		vacation_days INTEGER,
		work_time_model VARCHAR(50),
		status VARCHAR(50) NOT NULL DEFAULT 'active',
		show_in_staff_list BOOLEAN NOT NULL DEFAULT true,
		notes TEXT,
		email VARCHAR(255),
		phone VARCHAR(50),
		mobile VARCHAR(50),
		created_by UUID,
		updated_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		UNIQUE(tenant_id, employee_number)
	);
	ALTER TABLE staff.employees ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.employees;
	CREATE POLICY tenant_isolation ON staff.employees
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.employee_addresses (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,
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
	);
	ALTER TABLE staff.employee_addresses ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.employee_addresses;
	CREATE POLICY tenant_isolation ON staff.employee_addresses
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.employee_contacts (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,
		contact_type VARCHAR(50) NOT NULL DEFAULT 'emergency',
		name VARCHAR(255) NOT NULL,
		relationship VARCHAR(100),
		phone VARCHAR(50),
		email VARCHAR(255),
		is_primary BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE staff.employee_contacts ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.employee_contacts;
	CREATE POLICY tenant_isolation ON staff.employee_contacts
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.employee_financials (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,
		iban VARCHAR(34),
		bic VARCHAR(11),
		tax_id VARCHAR(20),
		tax_class VARCHAR(10),
		church_tax BOOLEAN DEFAULT false,
		child_allowance DECIMAL(3,1) DEFAULT 0,
		salary_type VARCHAR(20) DEFAULT 'monthly',
		base_salary_cents INTEGER,
		currency VARCHAR(3) DEFAULT 'EUR',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE staff.employee_financials ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.employee_financials;
	CREATE POLICY tenant_isolation ON staff.employee_financials
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.employee_social_insurance (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,
		social_security_number VARCHAR(20),
		health_insurance_provider VARCHAR(255),
		health_insurance_number VARCHAR(50),
		pension_insurance BOOLEAN DEFAULT true,
		unemployment_insurance BOOLEAN DEFAULT true,
		health_insurance BOOLEAN DEFAULT true,
		care_insurance BOOLEAN DEFAULT true,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE staff.employee_social_insurance ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.employee_social_insurance;
	CREATE POLICY tenant_isolation ON staff.employee_social_insurance
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.employee_documents (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,
		document_type VARCHAR(50) NOT NULL,
		file_name VARCHAR(255),
		file_path TEXT,
		file_size INTEGER,
		mime_type VARCHAR(100),
		issue_date DATE,
		expiry_date DATE,
		status VARCHAR(20) DEFAULT 'active',
		notes TEXT,
		uploaded_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ
	);
	ALTER TABLE staff.employee_documents ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.employee_documents;
	CREATE POLICY tenant_isolation ON staff.employee_documents
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.employee_files (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		file_type VARCHAR(50),
		file_path TEXT NOT NULL,
		file_size INTEGER,
		mime_type VARCHAR(100),
		category VARCHAR(50) DEFAULT 'other',
		uploaded_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ
	);
	ALTER TABLE staff.employee_files ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.employee_files;
	CREATE POLICY tenant_isolation ON staff.employee_files
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.time_entries (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id),
		entry_date DATE NOT NULL DEFAULT CURRENT_DATE,
		clock_in TIMESTAMPTZ NOT NULL,
		clock_out TIMESTAMPTZ,
		total_work_minutes INTEGER DEFAULT 0,
		total_break_minutes INTEGER DEFAULT 0,
		status VARCHAR(20) DEFAULT 'active',
		entry_type VARCHAR(20) DEFAULT 'regular',
		is_manual_entry BOOLEAN DEFAULT false,
		notes TEXT,
		created_by UUID,
		updated_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ
	);
	ALTER TABLE staff.time_entries ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.time_entries;
	CREATE POLICY tenant_isolation ON staff.time_entries
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.time_breaks (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		time_entry_id UUID NOT NULL REFERENCES staff.time_entries(id) ON DELETE CASCADE,
		start_time TIMESTAMPTZ NOT NULL,
		end_time TIMESTAMPTZ,
		break_type VARCHAR(20) DEFAULT 'regular',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE staff.time_breaks ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.time_breaks;
	CREATE POLICY tenant_isolation ON staff.time_breaks
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.time_corrections (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id),
		time_entry_id UUID REFERENCES staff.time_entries(id),
		correction_date DATE NOT NULL DEFAULT CURRENT_DATE,
		original_clock_in TIMESTAMPTZ,
		original_clock_out TIMESTAMPTZ,
		corrected_clock_in TIMESTAMPTZ,
		corrected_clock_out TIMESTAMPTZ,
		reason TEXT,
		status VARCHAR(20) DEFAULT 'pending',
		corrected_by UUID,
		approved_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ
	);
	ALTER TABLE staff.time_corrections ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.time_corrections;
	CREATE POLICY tenant_isolation ON staff.time_corrections
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.shift_templates (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		name VARCHAR(100) NOT NULL,
		description TEXT,
		start_time TIME NOT NULL,
		end_time TIME NOT NULL,
		break_duration_minutes INTEGER DEFAULT 0,
		shift_type VARCHAR(50) DEFAULT 'regular',
		color VARCHAR(20) DEFAULT '#22c55e',
		is_active BOOLEAN DEFAULT true,
		created_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		UNIQUE(tenant_id, name)
	);
	ALTER TABLE staff.shift_templates ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.shift_templates;
	CREATE POLICY tenant_isolation ON staff.shift_templates
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.shift_assignments (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id),
		shift_template_id UUID REFERENCES staff.shift_templates(id),
		shift_date DATE NOT NULL,
		start_time TIME NOT NULL,
		end_time TIME NOT NULL,
		break_duration_minutes INTEGER DEFAULT 0,
		shift_type VARCHAR(50) DEFAULT 'regular',
		status VARCHAR(20) DEFAULT 'scheduled',
		has_conflict BOOLEAN DEFAULT false,
		conflict_reason TEXT,
		notes TEXT,
		created_by UUID,
		updated_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ
	);
	ALTER TABLE staff.shift_assignments ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.shift_assignments;
	CREATE POLICY tenant_isolation ON staff.shift_assignments
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.user_cache (
		user_id UUID PRIMARY KEY,
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		email VARCHAR(255),
		first_name VARCHAR(100),
		last_name VARCHAR(100),
		role_name VARCHAR(50),
		avatar_url TEXT,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE staff.user_cache ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.user_cache;
	CREATE POLICY tenant_isolation ON staff.user_cache
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.absences (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,
		start_date DATE NOT NULL,
		end_date DATE NOT NULL,
		absence_type VARCHAR(50) NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'pending',
		requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		reviewed_by UUID,
		reviewed_at TIMESTAMPTZ,
		rejection_reason TEXT,
		vacation_days_used DECIMAL(4,2),
		employee_note TEXT,
		manager_note TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		created_by UUID,
		updated_by UUID,
		CONSTRAINT absences_type_valid CHECK (
			absence_type IN (
				'vacation', 'sick', 'sick_child', 'training', 'special_leave',
				'unpaid_leave', 'parental_leave', 'comp_time', 'other'
			)
		),
		CONSTRAINT absences_status_valid CHECK (
			status IN ('pending', 'approved', 'rejected', 'cancelled')
		),
		CONSTRAINT absences_dates_valid CHECK (end_date >= start_date)
	);
	ALTER TABLE staff.absences ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.absences;
	CREATE POLICY tenant_isolation ON staff.absences
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.vacation_balances (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,
		year INTEGER NOT NULL,
		annual_entitlement DECIMAL(5,2) NOT NULL DEFAULT 30,
		carryover_from_previous DECIMAL(5,2) NOT NULL DEFAULT 0,
		additional_granted DECIMAL(5,2) NOT NULL DEFAULT 0,
		taken DECIMAL(5,2) NOT NULL DEFAULT 0,
		planned DECIMAL(5,2) NOT NULL DEFAULT 0,
		pending DECIMAL(5,2) NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT vacation_balances_tenant_unique UNIQUE (tenant_id, employee_id, year),
		CONSTRAINT vacation_balances_year_valid CHECK (year >= 2000 AND year <= 2100),
		CONSTRAINT vacation_balances_entitlement_valid CHECK (annual_entitlement >= 0),
		CONSTRAINT vacation_balances_taken_valid CHECK (taken >= 0),
		CONSTRAINT vacation_balances_planned_valid CHECK (planned >= 0),
		CONSTRAINT vacation_balances_pending_valid CHECK (pending >= 0)
	);
	ALTER TABLE staff.vacation_balances ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.vacation_balances;
	CREATE POLICY tenant_isolation ON staff.vacation_balances
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.compliance_settings (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		min_break_6h_minutes INTEGER NOT NULL DEFAULT 30,
		min_break_9h_minutes INTEGER NOT NULL DEFAULT 45,
		min_break_segment_minutes INTEGER NOT NULL DEFAULT 15,
		max_daily_hours INTEGER NOT NULL DEFAULT 10,
		target_daily_hours INTEGER NOT NULL DEFAULT 8,
		max_weekly_hours INTEGER NOT NULL DEFAULT 48,
		min_rest_between_shifts_hours INTEGER NOT NULL DEFAULT 11,
		alert_no_break_after_minutes INTEGER NOT NULL DEFAULT 360,
		alert_break_too_long_minutes INTEGER NOT NULL DEFAULT 60,
		alert_approaching_max_hours_minutes INTEGER NOT NULL DEFAULT 30,
		notify_employee_violations BOOLEAN NOT NULL DEFAULT true,
		notify_manager_violations BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT compliance_settings_tenant_unique UNIQUE (tenant_id)
	);
	ALTER TABLE staff.compliance_settings ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.compliance_settings;
	CREATE POLICY tenant_isolation ON staff.compliance_settings
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.compliance_violations (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id),
		violation_type VARCHAR(50) NOT NULL,
		violation_date DATE NOT NULL,
		time_entry_id UUID REFERENCES staff.time_entries(id),
		shift_assignment_id UUID REFERENCES staff.shift_assignments(id),
		expected_value VARCHAR(100),
		actual_value VARCHAR(100),
		description TEXT,
		status VARCHAR(20) NOT NULL DEFAULT 'open',
		acknowledged_by UUID REFERENCES staff.employees(id),
		acknowledged_at TIMESTAMPTZ,
		resolution_notes TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE staff.compliance_violations ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.compliance_violations;
	CREATE POLICY tenant_isolation ON staff.compliance_violations
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.arbzg_compliance_log (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,
		time_entry_id UUID,
		violation_date DATE NOT NULL,
		violation_type VARCHAR(100) NOT NULL,
		severity VARCHAR(20) NOT NULL DEFAULT 'warning',
		description TEXT NOT NULL,
		details JSONB,
		acknowledged_by UUID,
		acknowledged_at TIMESTAMPTZ,
		resolution_note TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT arbzg_compliance_severity_valid CHECK (
			severity IN ('warning', 'violation', 'critical')
		)
	);
	ALTER TABLE staff.arbzg_compliance_log ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.arbzg_compliance_log;
	CREATE POLICY tenant_isolation ON staff.arbzg_compliance_log
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.compliance_alerts (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id),
		alert_type VARCHAR(50) NOT NULL,
		severity VARCHAR(20) NOT NULL DEFAULT 'warning',
		message TEXT NOT NULL,
		action_label VARCHAR(100),
		is_active BOOLEAN NOT NULL DEFAULT true,
		dismissed_by UUID REFERENCES staff.employees(id),
		dismissed_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE staff.compliance_alerts ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.compliance_alerts;
	CREATE POLICY tenant_isolation ON staff.compliance_alerts
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.time_correction_requests (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		employee_id UUID NOT NULL REFERENCES staff.employees(id),
		time_entry_id UUID REFERENCES staff.time_entries(id),
		requested_date DATE NOT NULL,
		requested_clock_in TIMESTAMPTZ,
		requested_clock_out TIMESTAMPTZ,
		request_type VARCHAR(50) NOT NULL,
		reason TEXT NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		reviewed_by UUID REFERENCES staff.employees(id),
		reviewed_at TIMESTAMPTZ,
		rejection_reason TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ
	);
	ALTER TABLE staff.time_correction_requests ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.time_correction_requests;
	CREATE POLICY tenant_isolation ON staff.time_correction_requests
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS staff.document_processing_audit (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		document_type VARCHAR(50) NOT NULL,
		consent_timestamp TIMESTAMPTZ NOT NULL,
		consent_given_by UUID NOT NULL,
		fields_extracted TEXT[] NOT NULL DEFAULT '{}',
		processing_duration_ms INTEGER,
		image_deleted_at TIMESTAMPTZ NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE staff.document_processing_audit ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON staff.document_processing_audit;
	CREATE POLICY tenant_isolation ON staff.document_processing_audit
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
`

// inventorySchemaSQL creates core inventory schema tables with RLS policies.
// Must match the production migrations (000005 + 000014).
var inventorySchemaSQL = `
	CREATE TABLE IF NOT EXISTS inventory.storage_rooms (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		name VARCHAR(100) NOT NULL,
		description TEXT,
		floor VARCHAR(50),
		building VARCHAR(100),
		is_active BOOLEAN DEFAULT true,
		created_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		UNIQUE(tenant_id, name)
	);
	ALTER TABLE inventory.storage_rooms ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.storage_rooms;
	CREATE POLICY tenant_isolation ON inventory.storage_rooms
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS inventory.storage_cabinets (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		room_id UUID NOT NULL REFERENCES inventory.storage_rooms(id),
		name VARCHAR(255) NOT NULL,
		description TEXT,
		temperature_controlled BOOLEAN NOT NULL DEFAULT FALSE,
		target_temperature_celsius DECIMAL(5,2),
		requires_key BOOLEAN NOT NULL DEFAULT FALSE,
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		min_temperature_celsius DECIMAL(5,2),
		max_temperature_celsius DECIMAL(5,2),
		temperature_monitoring_enabled BOOLEAN NOT NULL DEFAULT FALSE,
		created_by UUID,
		updated_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ
	);
	ALTER TABLE inventory.storage_cabinets ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.storage_cabinets;
	CREATE POLICY tenant_isolation ON inventory.storage_cabinets
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS inventory.storage_shelves (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		cabinet_id UUID NOT NULL REFERENCES inventory.storage_cabinets(id),
		name VARCHAR(100) NOT NULL,
		position INTEGER,
		is_active BOOLEAN DEFAULT true,
		created_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ
	);
	ALTER TABLE inventory.storage_shelves ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.storage_shelves;
	CREATE POLICY tenant_isolation ON inventory.storage_shelves
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS inventory.inventory_items (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		name VARCHAR(255) NOT NULL,
		description TEXT,
		category VARCHAR(100),
		barcode VARCHAR(100),
		article_number VARCHAR(100),
		manufacturer VARCHAR(255),
		supplier VARCHAR(255),
		unit VARCHAR(50) NOT NULL,
		min_stock INTEGER NOT NULL DEFAULT 0,
		max_stock INTEGER,
		reorder_point INTEGER,
		reorder_quantity INTEGER,
		use_batch_tracking BOOLEAN NOT NULL DEFAULT FALSE,
		requires_cooling BOOLEAN NOT NULL DEFAULT FALSE,
		is_hazardous BOOLEAN NOT NULL DEFAULT FALSE,
		shelf_life_days INTEGER,
		default_location_id UUID,
		unit_price_cents INTEGER,
		currency VARCHAR(3) DEFAULT 'EUR',
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		-- Compliance fields (000014)
		manufacturer_address TEXT,
		ce_marking_number VARCHAR(100),
		notified_body_id VARCHAR(100),
		acquisition_date DATE,
		serial_number VARCHAR(100),
		udi_di VARCHAR(255),
		udi_pi VARCHAR(255),
		-- Medical device fields (000015 MPBetreibV)
		is_medical_device BOOLEAN NOT NULL DEFAULT FALSE,
		device_type VARCHAR(100),
		device_model VARCHAR(255),
		authorized_representative TEXT,
		importer TEXT,
		operational_id_number VARCHAR(100),
		location_assignment TEXT,
		risk_class VARCHAR(20),
		stk_interval_months INTEGER,
		mtk_interval_months INTEGER,
		last_stk_date DATE,
		next_stk_due DATE,
		last_mtk_date DATE,
		next_mtk_due DATE,
		shelf_life_after_opening_days INTEGER,
		created_by UUID,
		updated_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ
	);
	ALTER TABLE inventory.inventory_items ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.inventory_items;
	CREATE POLICY tenant_isolation ON inventory.inventory_items
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
	CREATE INDEX IF NOT EXISTS idx_inventory_items_tenant ON inventory.inventory_items(tenant_id);

	CREATE TABLE IF NOT EXISTS inventory.inventory_batches (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE RESTRICT,
		location_id UUID,
		batch_number VARCHAR(100) NOT NULL,
		lot_number VARCHAR(100),
		initial_quantity INTEGER NOT NULL,
		current_quantity INTEGER NOT NULL,
		reserved_quantity INTEGER NOT NULL DEFAULT 0,
		manufactured_date DATE,
		expiry_date DATE,
		received_date DATE NOT NULL DEFAULT CURRENT_DATE,
		opened_at TIMESTAMPTZ,
		status VARCHAR(50) NOT NULL DEFAULT 'available',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		CONSTRAINT batches_quantity_valid CHECK (current_quantity >= 0),
		CONSTRAINT batches_reserved_valid CHECK (reserved_quantity >= 0 AND reserved_quantity <= current_quantity),
		CONSTRAINT batches_status_valid CHECK (status IN ('available', 'reserved', 'quarantine', 'expired', 'depleted'))
	);
	ALTER TABLE inventory.inventory_batches ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.inventory_batches;
	CREATE POLICY tenant_isolation ON inventory.inventory_batches
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS inventory.stock_adjustments (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE RESTRICT,
		batch_id UUID REFERENCES inventory.inventory_batches(id),
		adjustment_type VARCHAR(50) NOT NULL,
		quantity INTEGER NOT NULL,
		previous_quantity INTEGER NOT NULL DEFAULT 0,
		new_quantity INTEGER NOT NULL DEFAULT 0,
		from_location_id UUID,
		to_location_id UUID,
		reason TEXT,
		reference_type VARCHAR(50),
		reference_id UUID,
		performed_by VARCHAR(255),
		performed_by_name VARCHAR(255),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		created_by UUID
	);
	ALTER TABLE inventory.stock_adjustments ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.stock_adjustments;
	CREATE POLICY tenant_isolation ON inventory.stock_adjustments
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS inventory.inventory_alerts (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		item_id UUID REFERENCES inventory.inventory_items(id),
		batch_id UUID REFERENCES inventory.inventory_batches(id),
		alert_type VARCHAR(50) NOT NULL,
		severity VARCHAR(20) DEFAULT 'warning',
		message TEXT NOT NULL,
		status VARCHAR(20) DEFAULT 'open',
		acknowledged_by UUID,
		acknowledged_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE inventory.inventory_alerts ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.inventory_alerts;
	CREATE POLICY tenant_isolation ON inventory.inventory_alerts
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	-- Hazardous substance details (000014)
	CREATE TABLE IF NOT EXISTS inventory.hazardous_substance_details (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE CASCADE,
		ghs_pictogram_codes TEXT,
		h_statements TEXT,
		p_statements TEXT,
		signal_word VARCHAR(50),
		usage_area TEXT,
		storage_instructions TEXT,
		emergency_procedures TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		created_by UUID,
		updated_by UUID,
		CONSTRAINT uq_hazardous_tenant_item UNIQUE(tenant_id, item_id)
	);
	ALTER TABLE inventory.hazardous_substance_details ENABLE ROW LEVEL SECURITY;
	ALTER TABLE inventory.hazardous_substance_details FORCE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.hazardous_substance_details;
	CREATE POLICY tenant_isolation ON inventory.hazardous_substance_details
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	-- Device inspections (000015 - Medizinproduktebuch)
	CREATE TABLE IF NOT EXISTS inventory.device_inspections (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE CASCADE,
		inspection_type VARCHAR(10) NOT NULL,
		inspection_date DATE NOT NULL,
		next_due_date DATE,
		result VARCHAR(50) NOT NULL,
		performed_by TEXT NOT NULL,
		report_reference VARCHAR(255),
		notes TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		created_by UUID,
		updated_by UUID,
		CONSTRAINT device_inspections_type_valid CHECK (inspection_type IN ('STK', 'MTK')),
		CONSTRAINT device_inspections_result_valid CHECK (result IN ('passed', 'failed', 'conditional'))
	);
	ALTER TABLE inventory.device_inspections ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.device_inspections;
	CREATE POLICY tenant_isolation ON inventory.device_inspections
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	-- Device trainings (000015 - Medizinproduktebuch)
	CREATE TABLE IF NOT EXISTS inventory.device_trainings (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE CASCADE,
		training_date DATE NOT NULL,
		trainer_name TEXT NOT NULL,
		trainer_qualification TEXT,
		attendee_names TEXT NOT NULL,
		topic TEXT,
		notes TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		created_by UUID,
		updated_by UUID
	);
	ALTER TABLE inventory.device_trainings ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.device_trainings;
	CREATE POLICY tenant_isolation ON inventory.device_trainings
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	-- Device incidents (000015 - Medizinproduktebuch)
	CREATE TABLE IF NOT EXISTS inventory.device_incidents (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE CASCADE,
		incident_date DATE NOT NULL,
		incident_type VARCHAR(50) NOT NULL,
		description TEXT NOT NULL,
		consequences TEXT,
		corrective_action TEXT,
		reported_to TEXT,
		report_date DATE,
		report_reference VARCHAR(255),
		notes TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		created_by UUID,
		updated_by UUID,
		CONSTRAINT device_incidents_type_valid CHECK (incident_type IN ('malfunction', 'near_miss', 'serious_incident'))
	);
	ALTER TABLE inventory.device_incidents ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.device_incidents;
	CREATE POLICY tenant_isolation ON inventory.device_incidents
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	-- Temperature readings (000015 - cold-chain monitoring)
	CREATE TABLE IF NOT EXISTS inventory.temperature_readings (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		cabinet_id UUID NOT NULL REFERENCES inventory.storage_cabinets(id) ON DELETE CASCADE,
		temperature_celsius DECIMAL(5,2) NOT NULL,
		recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		recorded_by UUID,
		source VARCHAR(20) NOT NULL DEFAULT 'manual',
		is_excursion BOOLEAN NOT NULL DEFAULT FALSE,
		notes TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT temperature_readings_source_valid CHECK (source IN ('manual', 'webhook', 'sensor'))
	);
	ALTER TABLE inventory.temperature_readings ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.temperature_readings;
	CREATE POLICY tenant_isolation ON inventory.temperature_readings
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	-- Item documents (000014)
	CREATE TABLE IF NOT EXISTS inventory.item_documents (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE CASCADE,
		document_type VARCHAR(50) NOT NULL,
		file_name VARCHAR(255) NOT NULL,
		file_path TEXT NOT NULL,
		file_size_bytes INTEGER,
		mime_type VARCHAR(100),
		uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		uploaded_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		CONSTRAINT item_documents_type_valid CHECK (document_type IN ('sdb', 'manual', 'certificate'))
	);
	ALTER TABLE inventory.item_documents ENABLE ROW LEVEL SECURITY;
	ALTER TABLE inventory.item_documents FORCE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.item_documents;
	CREATE POLICY tenant_isolation ON inventory.item_documents
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS inventory.user_cache (
		user_id UUID PRIMARY KEY,
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		email VARCHAR(255),
		first_name VARCHAR(100),
		last_name VARCHAR(100),
		role_name VARCHAR(50),
		avatar_url TEXT,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE inventory.user_cache ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.user_cache;
	CREATE POLICY tenant_isolation ON inventory.user_cache
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
`
