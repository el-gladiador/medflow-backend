-- Create Test Tenant for Development
-- This script creates a test tenant "test-practice" for development and testing
-- Uses RLS-based pooled multi-tenancy (no per-tenant schema)

-- Insert test tenant into public.tenants
INSERT INTO public.tenants (
    id,
    slug,
    name,
    subscription_status,
    settings
) VALUES (
    'a0000000-0000-0000-0000-000000000001',  -- Fixed UUID for dev
    'test-practice',
    'Test Dental Practice (Dev)',
    'active',
    '{
        "language": "de",
        "timezone": "Europe/Berlin",
        "dateFormat": "DD.MM.YYYY",
        "currency": "EUR"
    }'::jsonb
) ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    subscription_status = 'active',
    updated_at = NOW();

-- Log tenant creation
INSERT INTO public.tenant_audit_log (
    tenant_id,
    action,
    details
) VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'created',
    '{"method": "dev_setup_script", "environment": "development"}'::jsonb
);

\echo 'Test tenant "test-practice" created successfully!'
