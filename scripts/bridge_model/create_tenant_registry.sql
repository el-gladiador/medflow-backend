-- Run against Auth database (postgres-auth:5433)
-- Creates tenant entry in central registry

INSERT INTO public.tenants (
    id,
    slug,
    schema_name,
    name,
    email,
    subscription_tier,
    subscription_status,
    settings
) VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'test-practice',
    'tenant_test_practice',
    'Test Dental Practice (Dev)',
    'mohammadamiri.py@gmail.com',
    'enterprise',
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

-- Log creation
INSERT INTO public.tenant_audit_log (
    tenant_id,
    event_type,
    event_data
) VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'created',
    '{"method": "bridge_model_setup", "environment": "development"}'::jsonb
);

\echo 'Tenant registered in auth database!'
