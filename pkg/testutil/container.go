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

		-- inventory schema
		ALTER TABLE inventory.storage_rooms FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.storage_cabinets FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.storage_shelves FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.inventory_items FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.inventory_batches FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.inventory_alerts FORCE ROW LEVEL SECURITY;
		ALTER TABLE inventory.user_cache FORCE ROW LEVEL SECURITY;
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
			tenant_id UUID NOT NULL REFERENCES public.tenants(id),
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

// staffSchemaSQL creates core staff schema tables with RLS policies
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
		job_title VARCHAR(100),
		department VARCHAR(100),
		employment_type VARCHAR(50) DEFAULT 'full_time',
		hire_date DATE,
		probation_end_date DATE,
		termination_date DATE,
		status VARCHAR(20) DEFAULT 'active',
		show_in_staff_list BOOLEAN DEFAULT true,
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
`

// inventorySchemaSQL creates core inventory schema tables with RLS policies
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
		name VARCHAR(100) NOT NULL,
		description TEXT,
		is_active BOOLEAN DEFAULT true,
		created_by UUID,
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
		sku VARCHAR(100),
		description TEXT,
		category VARCHAR(100),
		unit VARCHAR(50) DEFAULT 'piece',
		min_stock_level INTEGER DEFAULT 0,
		reorder_point INTEGER DEFAULT 0,
		shelf_id UUID REFERENCES inventory.storage_shelves(id),
		is_active BOOLEAN DEFAULT true,
		created_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ,
		UNIQUE(tenant_id, sku)
	);
	ALTER TABLE inventory.inventory_items ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.inventory_items;
	CREATE POLICY tenant_isolation ON inventory.inventory_items
		FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
		WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

	CREATE TABLE IF NOT EXISTS inventory.inventory_batches (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL REFERENCES public.tenants(id),
		item_id UUID NOT NULL REFERENCES inventory.inventory_items(id),
		batch_number VARCHAR(100),
		quantity INTEGER NOT NULL DEFAULT 0,
		expiry_date DATE,
		supplier VARCHAR(255),
		purchase_price DECIMAL(10,2),
		notes TEXT,
		created_by UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMPTZ
	);
	ALTER TABLE inventory.inventory_batches ENABLE ROW LEVEL SECURITY;
	DROP POLICY IF EXISTS tenant_isolation ON inventory.inventory_batches;
	CREATE POLICY tenant_isolation ON inventory.inventory_batches
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
