-- MedFlow Schema-per-Tenant: Public Schema
-- Migration: Create tenants table (tenant registry)

-- Shared function for updated_at timestamps (used by all schemas)
CREATE OR REPLACE FUNCTION public.update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Tenants table: Central registry of all tenants
-- This is the ONLY shared table - all tenant data lives in separate schemas
CREATE TABLE public.tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Identity
    slug VARCHAR(100) NOT NULL UNIQUE,           -- URL-friendly: "praxis-mueller"
    schema_name VARCHAR(100) NOT NULL UNIQUE,    -- DB schema: "tenant_praxis_mueller"
    name VARCHAR(255) NOT NULL,                  -- Display: "Zahnarztpraxis Dr. Müller"

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
    db_user VARCHAR(100),

    -- Feature Flags (enable/disable features per tenant)
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

-- Indexes
CREATE INDEX idx_tenants_slug ON public.tenants(slug) WHERE deleted_at IS NULL;
CREATE INDEX idx_tenants_schema ON public.tenants(schema_name) WHERE deleted_at IS NULL;
CREATE INDEX idx_tenants_status ON public.tenants(subscription_status) WHERE deleted_at IS NULL;
CREATE INDEX idx_tenants_tier ON public.tenants(subscription_tier) WHERE deleted_at IS NULL;
CREATE INDEX idx_tenants_deleted ON public.tenants(deleted_at) WHERE deleted_at IS NOT NULL;

-- Updated_at trigger
CREATE TRIGGER tenants_updated_at
    BEFORE UPDATE ON public.tenants
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- Comments
COMMENT ON TABLE public.tenants IS 'Central registry of all tenants. Each tenant has a separate PostgreSQL schema.';
COMMENT ON COLUMN public.tenants.slug IS 'URL-friendly identifier used in URLs (e.g., app.medflow.de/praxis-mueller)';
COMMENT ON COLUMN public.tenants.schema_name IS 'PostgreSQL schema name where this tenant''s data lives';
COMMENT ON COLUMN public.tenants.kms_key_arn IS 'AWS KMS key ARN for per-tenant encryption (premium/enterprise only)';
COMMENT ON COLUMN public.tenants.features IS 'Feature flags JSON: {"betaFeatures": true, "customReports": false}';
