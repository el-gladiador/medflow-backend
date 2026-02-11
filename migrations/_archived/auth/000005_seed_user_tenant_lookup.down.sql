-- Rollback: Clear seeded user-tenant lookup data
-- Note: This only clears the data, not the table structure

TRUNCATE TABLE public.user_tenant_lookup;
