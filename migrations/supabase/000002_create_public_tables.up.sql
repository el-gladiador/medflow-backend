-- MedFlow RLS Multi-Tenancy: Public Schema Tables
-- Shared infrastructure tables - NO RLS (not tenant-scoped)

-- ============================================================================
-- TENANTS (tenant registry)
-- ============================================================================
CREATE TABLE public.tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Identity
    slug VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,

    -- Contact
    email VARCHAR(255),
    phone VARCHAR(50),

    -- Address (German format)
    street VARCHAR(255),
    city VARCHAR(100),
    postal_code VARCHAR(20),
    country VARCHAR(100) NOT NULL DEFAULT 'Germany',

    -- Subscription & Billing
    subscription_tier VARCHAR(50) NOT NULL DEFAULT 'standard',
    subscription_status VARCHAR(50) NOT NULL DEFAULT 'active',
    trial_ends_at TIMESTAMPTZ,
    billing_email VARCHAR(255),

    -- Security (per-tenant encryption for premium)
    kms_key_arn VARCHAR(255),

    -- Feature Flags
    features JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Limits
    max_users INTEGER NOT NULL DEFAULT 50,
    max_storage_gb INTEGER NOT NULL DEFAULT 10,

    -- Settings (German defaults)
    settings JSONB NOT NULL DEFAULT '{
        "language": "de",
        "timezone": "Europe/Berlin",
        "dateFormat": "DD.MM.YYYY",
        "currency": "EUR"
    }'::jsonb,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- Constraints
    CONSTRAINT tenants_slug_format CHECK (
        slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$' AND
        char_length(slug) >= 2 AND
        char_length(slug) <= 100
    ),
    CONSTRAINT tenants_tier_valid CHECK (
        subscription_tier IN ('free', 'standard', 'premium', 'enterprise')
    ),
    CONSTRAINT tenants_status_valid CHECK (
        subscription_status IN ('active', 'trial', 'suspended', 'cancelled')
    ),
    CONSTRAINT tenants_email_format CHECK (
        email IS NULL OR email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'
    )
);

CREATE INDEX idx_tenants_slug ON public.tenants(slug) WHERE deleted_at IS NULL;
CREATE INDEX idx_tenants_status ON public.tenants(subscription_status) WHERE deleted_at IS NULL;
CREATE INDEX idx_tenants_deleted ON public.tenants(deleted_at) WHERE deleted_at IS NOT NULL;

CREATE TRIGGER tenants_updated_at
    BEFORE UPDATE ON public.tenants
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

COMMENT ON TABLE public.tenants IS 'Central registry of all tenants (practices/clinics)';

-- ============================================================================
-- SESSIONS (auth sessions - shared across tenants)
-- ============================================================================
CREATE TABLE public.sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    refresh_token_hash VARCHAR(255) NOT NULL UNIQUE,
    user_agent TEXT,
    ip_address INET,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_used_at TIMESTAMPTZ DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_sessions_user_id ON public.sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON public.sessions(expires_at) WHERE revoked_at IS NULL;
CREATE INDEX idx_sessions_refresh_token ON public.sessions(refresh_token_hash);

-- ============================================================================
-- TOKEN BLACKLIST (revoked tokens)
-- ============================================================================
CREATE TABLE public.token_blacklist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_jti VARCHAR(255) NOT NULL UNIQUE,
    user_id UUID NOT NULL,
    blacklisted_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_token_blacklist_jti ON public.token_blacklist(token_jti);
CREATE INDEX idx_token_blacklist_expires ON public.token_blacklist(expires_at);

-- ============================================================================
-- USER-TENANT LOOKUP (O(1) login resolution)
-- ============================================================================
CREATE TABLE public.user_tenant_lookup (
    email VARCHAR(255) PRIMARY KEY,
    user_id UUID NOT NULL,
    username VARCHAR(100),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    tenant_slug VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_tenant_lookup_user_id ON public.user_tenant_lookup(user_id);
CREATE INDEX idx_user_tenant_lookup_tenant_id ON public.user_tenant_lookup(tenant_id);
CREATE INDEX idx_user_tenant_lookup_username ON public.user_tenant_lookup(username) WHERE username IS NOT NULL;
CREATE UNIQUE INDEX idx_user_tenant_lookup_username_tenant_unique
    ON public.user_tenant_lookup(username, tenant_slug)
    WHERE username IS NOT NULL;

CREATE TRIGGER user_tenant_lookup_updated_at
    BEFORE UPDATE ON public.user_tenant_lookup
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

COMMENT ON TABLE public.user_tenant_lookup IS
    'Maps user emails to their tenant for O(1) login resolution';

-- ============================================================================
-- TENANT AUDIT LOG (system-wide audit)
-- ============================================================================
CREATE TABLE public.tenant_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    event_type VARCHAR(50) NOT NULL,
    event_data JSONB NOT NULL DEFAULT '{}',
    performed_by UUID,
    performed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip_address INET,
    user_agent TEXT,

    CONSTRAINT tenant_audit_event_type_valid CHECK (
        event_type IN (
            'created', 'updated', 'suspended', 'reactivated', 'deleted',
            'tier_changed', 'settings_updated', 'data_exported',
            'user_invited', 'user_removed'
        )
    )
);

CREATE INDEX idx_tenant_audit_tenant ON public.tenant_audit_log(tenant_id);
CREATE INDEX idx_tenant_audit_type ON public.tenant_audit_log(event_type);
CREATE INDEX idx_tenant_audit_time ON public.tenant_audit_log(performed_at DESC);

-- ============================================================================
-- OUTBOX (transactional outbox for reliable event publishing)
-- ============================================================================
CREATE TABLE public.outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    routing_key VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    retries INTEGER DEFAULT 0
);

CREATE INDEX idx_outbox_unpublished ON public.outbox(created_at) WHERE published_at IS NULL;
