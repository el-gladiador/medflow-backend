-- ============================================================================
-- MedFlow: One-time Supabase Setup
-- ============================================================================
--
-- Run this ONCE on a new Supabase project BEFORE running migrations.
-- It creates the medflow_app role and required schemas that the Docker
-- init-db.sql normally handles for local development.
--
-- Usage:
--   psql "postgres://postgres:<SUPABASE_DB_PASSWORD>@db.<PROJECT_REF>.supabase.co:5432/postgres" -f scripts/supabase-setup.sql
--
-- Or via Makefile:
--   make supabase-setup
--
-- IMPORTANT: After running this, change the medflow_app password:
--   ALTER ROLE medflow_app WITH PASSWORD 'your-strong-password-here';
--
-- ============================================================================

-- Create medflow_app role (idempotent)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'medflow_app') THEN
        -- CHANGE THIS PASSWORD after setup!
        CREATE ROLE medflow_app WITH LOGIN PASSWORD 'change-me-immediately' NOSUPERUSER NOCREATEDB NOCREATEROLE;
        RAISE NOTICE 'Created medflow_app role. IMPORTANT: Change the password!';
    ELSE
        RAISE NOTICE 'medflow_app role already exists, skipping.';
    END IF;
END
$$;

-- Create schemas if they don't exist (Supabase only has public by default)
CREATE SCHEMA IF NOT EXISTS users;
CREATE SCHEMA IF NOT EXISTS staff;
CREATE SCHEMA IF NOT EXISTS inventory;

-- Grant schema usage to postgres role (Supabase's default superuser)
-- This ensures migrations run by postgres can create objects in these schemas
GRANT ALL ON SCHEMA users TO postgres;
GRANT ALL ON SCHEMA staff TO postgres;
GRANT ALL ON SCHEMA inventory TO postgres;

-- ============================================================================
-- Verification
-- ============================================================================
DO $$
BEGIN
    RAISE NOTICE '------------------------------------------------------------';
    RAISE NOTICE 'Supabase setup complete!';
    RAISE NOTICE '';
    RAISE NOTICE 'Next steps:';
    RAISE NOTICE '  1. Change medflow_app password:';
    RAISE NOTICE '     ALTER ROLE medflow_app WITH PASSWORD ''your-strong-password'';';
    RAISE NOTICE '  2. Update deployments/.env.supabase with the new password';
    RAISE NOTICE '  3. Run migrations: make dev-supabase migrate-up';
    RAISE NOTICE '------------------------------------------------------------';
END
$$;
