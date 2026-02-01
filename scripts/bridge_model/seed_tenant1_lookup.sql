-- Run against Auth database (postgres-auth:5433)
-- Seeds user_tenant_lookup for the first tenant user

INSERT INTO public.user_tenant_lookup (
    email,
    user_id,
    tenant_id,
    tenant_slug,
    tenant_schema
) VALUES (
    'mohammadamiri.py@gmail.com',
    'a0000000-0000-0000-0000-000000000002',
    'a0000000-0000-0000-0000-000000000001',
    'test-practice',
    'tenant_test_practice'
) ON CONFLICT (email) DO UPDATE SET
    user_id = EXCLUDED.user_id,
    tenant_id = EXCLUDED.tenant_id,
    tenant_slug = EXCLUDED.tenant_slug,
    tenant_schema = EXCLUDED.tenant_schema,
    updated_at = NOW();

\echo 'User-tenant lookup seeded for mohammadamiri.py@gmail.com!'
