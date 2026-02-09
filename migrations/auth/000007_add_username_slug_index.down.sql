-- MedFlow: Rollback username unique constraint change
-- Reverts to global username uniqueness (pre-subdomain login behavior)

-- Drop the tenant-scoped unique index
DROP INDEX IF EXISTS idx_user_tenant_lookup_username_tenant_unique;

-- Restore global unique constraint on username
CREATE UNIQUE INDEX idx_user_tenant_lookup_username_unique
    ON public.user_tenant_lookup(username)
    WHERE username IS NOT NULL;
