-- MedFlow: Application Role Permissions & FORCE RLS
--
-- Problem: The 'medflow' user is a PostgreSQL superuser (required for migrations).
--          Superusers bypass ALL Row-Level Security policies, making RLS useless.
--
-- Solution: Services connect as 'medflow_app' (non-superuser).
--           This migration grants medflow_app the minimum required permissions
--           and adds FORCE ROW LEVEL SECURITY for defense-in-depth.
--
-- Security model:
--   medflow      = superuser, used ONLY for running migrations
--   medflow_app  = app role, used by all microservices at runtime (RLS enforced)

-- ============================================================================
-- CREATE ROLE (idempotent, in case init-db.sql didn't run)
-- ============================================================================
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'medflow_app') THEN
        CREATE ROLE medflow_app WITH LOGIN PASSWORD 'devpassword' NOSUPERUSER NOCREATEDB NOCREATEROLE;
    END IF;
END
$$;

-- Dynamic database name: works on both local Docker (medflow) and Supabase (postgres)
DO $$ BEGIN EXECUTE format('GRANT CONNECT ON DATABASE %I TO medflow_app', current_database()); END $$;

-- ============================================================================
-- SCHEMA USAGE
-- ============================================================================
GRANT USAGE ON SCHEMA public TO medflow_app;
GRANT USAGE ON SCHEMA users TO medflow_app;
GRANT USAGE ON SCHEMA staff TO medflow_app;
GRANT USAGE ON SCHEMA inventory TO medflow_app;

-- ============================================================================
-- TABLE PERMISSIONS - public schema (shared, no RLS)
-- ============================================================================
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO medflow_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO medflow_app;

-- ============================================================================
-- TABLE PERMISSIONS - users schema (RLS-protected)
-- ============================================================================
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA users TO medflow_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA users TO medflow_app;

-- ============================================================================
-- TABLE PERMISSIONS - staff schema (RLS-protected)
-- ============================================================================
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA staff TO medflow_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA staff TO medflow_app;

-- ============================================================================
-- TABLE PERMISSIONS - inventory schema (RLS-protected)
-- ============================================================================
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA inventory TO medflow_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA inventory TO medflow_app;

-- ============================================================================
-- DEFAULT PRIVILEGES (for future tables created by migrations)
-- ============================================================================
ALTER DEFAULT PRIVILEGES IN SCHEMA public    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO medflow_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA users     GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO medflow_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA staff     GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO medflow_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA inventory GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO medflow_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public    GRANT USAGE, SELECT ON SEQUENCES TO medflow_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA users     GRANT USAGE, SELECT ON SEQUENCES TO medflow_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA staff     GRANT USAGE, SELECT ON SEQUENCES TO medflow_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA inventory GRANT USAGE, SELECT ON SEQUENCES TO medflow_app;

-- ============================================================================
-- FORCE ROW LEVEL SECURITY (defense-in-depth)
-- Even the table owner must obey RLS. Only superusers bypass FORCE RLS.
-- Since medflow_app is NOT a superuser, this provides an extra safety layer.
-- ============================================================================

-- users schema
ALTER TABLE users.users FORCE ROW LEVEL SECURITY;
ALTER TABLE users.roles FORCE ROW LEVEL SECURITY;
ALTER TABLE users.user_roles FORCE ROW LEVEL SECURITY;
ALTER TABLE users.audit_logs FORCE ROW LEVEL SECURITY;
ALTER TABLE users.sessions FORCE ROW LEVEL SECURITY;

-- staff schema
ALTER TABLE staff.employees FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.employee_contacts FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.employee_addresses FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.employee_financials FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.employee_social_insurance FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.employee_documents FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.document_processing_audit FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.time_entries FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.time_breaks FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.time_corrections FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.time_correction_requests FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.shift_templates FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.shift_assignments FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.absences FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.vacation_balances FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.compliance_settings FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.compliance_violations FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.compliance_alerts FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.arbzg_compliance_log FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.user_cache FORCE ROW LEVEL SECURITY;

-- inventory schema (tables may not exist yet, use DO block for safety)
DO $$
BEGIN
    -- Only force RLS on inventory tables if they exist
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'inventory' AND table_name = 'storage_rooms') THEN
        EXECUTE 'ALTER TABLE inventory.storage_rooms FORCE ROW LEVEL SECURITY';
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'inventory' AND table_name = 'inventory_items') THEN
        EXECUTE 'ALTER TABLE inventory.inventory_items FORCE ROW LEVEL SECURITY';
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'inventory' AND table_name = 'inventory_batches') THEN
        EXECUTE 'ALTER TABLE inventory.inventory_batches FORCE ROW LEVEL SECURITY';
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'inventory' AND table_name = 'inventory_alerts') THEN
        EXECUTE 'ALTER TABLE inventory.inventory_alerts FORCE ROW LEVEL SECURITY';
    END IF;
END
$$;

-- ============================================================================
-- GRANT EXECUTE on functions needed by the app
-- ============================================================================
GRANT EXECUTE ON FUNCTION public.update_updated_at() TO medflow_app;
GRANT EXECUTE ON FUNCTION public.delete_tenant_data(UUID) TO medflow_app;
