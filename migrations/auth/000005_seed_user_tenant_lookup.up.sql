-- MedFlow Schema-per-Tenant: Seed User-Tenant Lookup Table
-- NOTE: In bridge model (separate databases per service), user data is in user-db,
-- not auth-db. Seeding must be done via scripts or events, not cross-db queries.
--
-- This migration is a no-op. User-tenant mappings are populated by:
-- 1. Bridge model setup scripts (scripts/bridge_model/seed_*_lookup.sql)
-- 2. User service events (user.created, user.updated, user.deleted)

-- Add a comment for documentation
COMMENT ON TABLE public.user_tenant_lookup IS 'Maps user emails to their tenant for O(1) login resolution. Seeded by bridge model scripts and kept in sync via user service events.';
