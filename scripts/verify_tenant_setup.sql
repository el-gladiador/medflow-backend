-- Verification Script for Test Tenant Setup (RLS-based multi-tenancy)
-- Run this to verify the test tenant and user were created correctly

\echo ''
\echo '========================================='
\echo 'MedFlow Tenant Setup Verification'
\echo '========================================='
\echo ''

-- 1. Check tenant exists in public schema
\echo '1. Checking tenant exists in public.tenants...'
SELECT
    id,
    slug,
    name,
    subscription_status,
    created_at
FROM public.tenants
WHERE slug = 'test-practice';

\echo ''
\echo '2. Checking RLS policies exist on users schema...'
SELECT tablename, policyname
FROM pg_policies
WHERE schemaname = 'users'
ORDER BY tablename;

\echo ''
\echo '3. Checking RLS policies exist on staff schema...'
SELECT tablename, policyname
FROM pg_policies
WHERE schemaname = 'staff'
ORDER BY tablename;

\echo ''
\echo '4. Checking RLS policies exist on inventory schema...'
SELECT tablename, policyname
FROM pg_policies
WHERE schemaname = 'inventory'
ORDER BY tablename;

\echo ''
\echo '5. Checking user exists for tenant (via RLS)...'
SET LOCAL app.current_tenant = 'a0000000-0000-0000-0000-000000000001';
SET LOCAL search_path TO users, public;
SELECT
    id,
    email,
    first_name,
    last_name,
    status
FROM users
LIMIT 5;

\echo ''
\echo '6. Checking user-tenant lookup entries...'
SET LOCAL search_path TO public;
SELECT
    email,
    username,
    user_id,
    tenant_id,
    tenant_slug
FROM public.user_tenant_lookup
WHERE tenant_id = 'a0000000-0000-0000-0000-000000000001';

\echo ''
\echo '7. Checking default roles for tenant...'
SET LOCAL app.current_tenant = 'a0000000-0000-0000-0000-000000000001';
SET LOCAL search_path TO users, public;
SELECT id, name, display_name, level
FROM roles
ORDER BY level DESC;

\echo ''
\echo '8. Tenant audit log...'
SELECT
    action,
    actor_name,
    details,
    created_at
FROM public.tenant_audit_log
WHERE tenant_id = 'a0000000-0000-0000-0000-000000000001'
ORDER BY created_at DESC
LIMIT 5;

\echo ''
\echo '========================================='
\echo 'Verification Complete!'
\echo '========================================='
\echo ''
