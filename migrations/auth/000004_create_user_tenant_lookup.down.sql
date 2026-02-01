-- Rollback: Remove user_tenant_lookup table

DROP TABLE IF EXISTS public.user_tenant_lookup CASCADE;
