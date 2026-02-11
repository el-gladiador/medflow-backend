-- MedFlow Schema-per-Tenant: User-Tenant Lookup Table
-- Migration: Create user_tenant_lookup for O(1) tenant resolution during login
--
-- This table enables fast lookup of which tenant a user belongs to,
-- eliminating the need to scan all tenant schemas during authentication.

CREATE TABLE public.user_tenant_lookup (
    -- Email is the primary key for O(1) lookup during login
    email VARCHAR(255) PRIMARY KEY,

    -- User reference (for reverse lookups and cleanup)
    user_id UUID NOT NULL,

    -- Tenant information (cached from public.tenants for fast access)
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    tenant_slug VARCHAR(100) NOT NULL,
    tenant_schema VARCHAR(100) NOT NULL,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for reverse lookups (e.g., when deleting a user)
CREATE INDEX idx_user_tenant_lookup_user_id ON public.user_tenant_lookup(user_id);

-- Index for tenant-level queries (e.g., listing all users in a tenant)
CREATE INDEX idx_user_tenant_lookup_tenant_id ON public.user_tenant_lookup(tenant_id);

-- Updated_at trigger
CREATE TRIGGER user_tenant_lookup_updated_at
    BEFORE UPDATE ON public.user_tenant_lookup
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- Comments
COMMENT ON TABLE public.user_tenant_lookup IS
    'Maps user emails to their tenant for O(1) login resolution. Populated via user.created/deleted events.';
COMMENT ON COLUMN public.user_tenant_lookup.email IS
    'Primary key - enables direct lookup during login without scanning tenant schemas.';
COMMENT ON COLUMN public.user_tenant_lookup.user_id IS
    'User UUID within the tenant schema. Used for reverse lookups during user deletion.';
COMMENT ON COLUMN public.user_tenant_lookup.tenant_schema IS
    'PostgreSQL schema name (e.g., tenant_praxis_mueller). Used to set search_path during auth.';
