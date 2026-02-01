-- Run against Auth database (postgres-auth:5433)
-- Creates SECOND tenant entry in central registry for multi-tenancy testing

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
    'b0000000-0000-0000-0000-000000000001',
    'demo-clinic',
    'tenant_demo_clinic',
    'Demo Dental Clinic (Test)',
    'demo@medflow.de',
    'standard',
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
    'b0000000-0000-0000-0000-000000000001',
    'created',
    '{"method": "bridge_model_setup", "environment": "development", "purpose": "multi_tenancy_testing"}'::jsonb
);

\echo 'Second tenant (demo-clinic) registered in auth database!'
