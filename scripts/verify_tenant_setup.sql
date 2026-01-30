-- Verification Script for Test Tenant Setup
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
    schema_name,
    name,
    subscription_tier,
    subscription_status,
    email
FROM public.tenants
WHERE slug = 'test-practice';

\echo ''
\echo '2. Checking schema exists...'
SELECT nspname AS schema_name
FROM pg_namespace
WHERE nspname = 'tenant_test_practice';

\echo ''
\echo '3. Checking tables in tenant schema...'
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'tenant_test_practice'
ORDER BY table_name;

\echo ''
\echo '4. Checking user exists in tenant...'
SET search_path TO tenant_test_practice;
SELECT
    id,
    email,
    first_name,
    last_name,
    status,
    email_verified_at IS NOT NULL AS email_verified
FROM users
WHERE email = 'mohammadamiri.py@gmail.com';

\echo ''
\echo '5. Checking user roles...'
SELECT
    u.email,
    r.name AS role_name,
    r.display_name
FROM users u
JOIN user_roles ur ON u.id = ur.user_id
JOIN roles r ON ur.role_id = r.id
WHERE u.email = 'mohammadamiri.py@gmail.com';

\echo ''
\echo '6. Verifying tenant isolation (should return 0 rows)...'
SET search_path TO public;
SELECT COUNT(*) AS user_count_in_public_schema
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name = 'users';

\echo ''
\echo '7. Checking default roles in tenant schema...'
SET search_path TO tenant_test_practice;
SELECT id, name, display_name, description
FROM roles
ORDER BY name;

\echo ''
\echo '8. Tenant audit log...'
SELECT
    action,
    performed_by,
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
