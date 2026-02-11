-- MedFlow RLS Multi-Tenancy: Schema & Function Setup
-- Creates service schemas and shared infrastructure for RLS-based tenant isolation.
--
-- Architecture:
--   - public schema: shared infrastructure (tenants registry, sessions, audit)
--   - users schema: user accounts, roles, permissions (RLS-protected)
--   - staff schema: employees, scheduling, time tracking (RLS-protected)
--   - inventory schema: items, batches, storage locations (RLS-protected)
--
-- Tenant isolation via RLS:
--   1. Every tenant-scoped table has tenant_id UUID NOT NULL
--   2. RLS policy: USING (tenant_id = current_setting('app.current_tenant')::uuid)
--   3. Middleware sets: SET LOCAL app.current_tenant = '<tenant-uuid>'

-- ============================================================================
-- CREATE SCHEMAS
-- ============================================================================
CREATE SCHEMA IF NOT EXISTS users;
CREATE SCHEMA IF NOT EXISTS staff;
CREATE SCHEMA IF NOT EXISTS inventory;

-- ============================================================================
-- SHARED FUNCTIONS
-- ============================================================================

-- Trigger function for automatic updated_at timestamps
CREATE OR REPLACE FUNCTION public.update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- DOWN MIGRATION
-- ============================================================================
-- See 000001_create_schemas_and_functions.down.sql
