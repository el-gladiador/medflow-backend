-- Create Test Tenant for Development
-- This script creates a test tenant "test-practice" for development and testing

-- Insert test tenant into public.tenants
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
    'a0000000-0000-0000-0000-000000000001',  -- Fixed UUID for dev
    'test-practice',
    'tenant_test_practice',
    'Test Dental Practice (Dev)',
    'mohammadamiri.py@gmail.com',
    'enterprise',  -- Give dev full features
    'active',
    '{
        "language": "de",
        "timezone": "Europe/Berlin",
        "dateFormat": "DD.MM.YYYY",
        "currency": "EUR"
    }'::jsonb
) ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    email = EXCLUDED.email,
    subscription_status = 'active',
    updated_at = NOW();

-- Create schema for test tenant
CREATE SCHEMA IF NOT EXISTS tenant_test_practice;

-- Grant permissions
GRANT USAGE ON SCHEMA tenant_test_practice TO medflow;
GRANT ALL ON ALL TABLES IN SCHEMA tenant_test_practice TO medflow;
GRANT ALL ON ALL SEQUENCES IN SCHEMA tenant_test_practice TO medflow;
ALTER DEFAULT PRIVILEGES IN SCHEMA tenant_test_practice GRANT ALL ON TABLES TO medflow;
ALTER DEFAULT PRIVILEGES IN SCHEMA tenant_test_practice GRANT ALL ON SEQUENCES TO medflow;

-- Log tenant creation
INSERT INTO public.tenant_audit_log (
    tenant_id,
    event_type,
    event_data
) VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'created',
    '{"method": "dev_setup_script", "environment": "development"}'::jsonb
);

\echo 'Test tenant "test-practice" created successfully!'
