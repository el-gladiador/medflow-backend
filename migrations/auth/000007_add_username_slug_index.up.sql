-- MedFlow: Fix username unique constraint for multi-tenant subdomain login
-- Migration: Username should be unique per tenant, not globally
--
-- Problem: Current unique index on username alone prevents the same username
-- from existing in different tenants (e.g., "admin" in clinic-a and clinic-b).
-- With subdomain-based login, username uniqueness should be scoped to tenant.
--
-- Solution: Replace global username unique index with (username, tenant_slug) index.

-- Drop the global unique constraint on username
DROP INDEX IF EXISTS idx_user_tenant_lookup_username_unique;

-- Create new unique index: username is unique WITHIN a tenant
CREATE UNIQUE INDEX idx_user_tenant_lookup_username_tenant_unique
    ON public.user_tenant_lookup(username, tenant_slug)
    WHERE username IS NOT NULL;

-- Comments
COMMENT ON INDEX idx_user_tenant_lookup_username_tenant_unique IS
    'Ensures username uniqueness within a tenant for subdomain-based login';
