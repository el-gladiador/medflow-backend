-- MedFlow Database Initialization
-- This script runs ONCE when the PostgreSQL container is first initialized.
-- It creates the non-superuser application role used by all microservices.
--
-- The 'medflow' superuser (POSTGRES_USER) is used only for migrations.
-- The 'medflow_app' role is used by services at runtime, ensuring RLS applies.

-- Create the application role (non-superuser, so RLS policies are enforced)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'medflow_app') THEN
        CREATE ROLE medflow_app WITH LOGIN PASSWORD 'devpassword' NOSUPERUSER NOCREATEDB NOCREATEROLE;
        RAISE NOTICE 'Created medflow_app role';
    END IF;
END
$$;

-- Grant connect to the database
GRANT CONNECT ON DATABASE medflow TO medflow_app;
