-- Rollback: Remove username support from user-tenant lookup table

DROP INDEX IF EXISTS idx_user_tenant_lookup_username_unique;
DROP INDEX IF EXISTS idx_user_tenant_lookup_username;
ALTER TABLE public.user_tenant_lookup DROP COLUMN IF EXISTS username;
