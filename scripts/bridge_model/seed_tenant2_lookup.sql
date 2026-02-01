-- Run against Auth database (postgres-auth:5433)
-- Seeds user_tenant_lookup for the second tenant user

INSERT INTO public.user_tenant_lookup (
    email,
    user_id,
    tenant_id,
    tenant_slug,
    tenant_schema
) VALUES (
    'demo@medflow.de',
    'b0000000-0000-0000-0000-000000000002',
    'b0000000-0000-0000-0000-000000000001',
    'demo-clinic',
    'tenant_demo_clinic'
) ON CONFLICT (email) DO UPDATE SET
    user_id = EXCLUDED.user_id,
    tenant_id = EXCLUDED.tenant_id,
    tenant_slug = EXCLUDED.tenant_slug,
    tenant_schema = EXCLUDED.tenant_schema,
    updated_at = NOW();

\echo 'User-tenant lookup seeded for demo@medflow.de!'
