-- Rollback: Drop tenants table

DROP TRIGGER IF EXISTS tenants_updated_at ON public.tenants;
DROP TABLE IF EXISTS public.tenants;
-- Note: We keep the update_updated_at function as it may be used by tenant schemas
