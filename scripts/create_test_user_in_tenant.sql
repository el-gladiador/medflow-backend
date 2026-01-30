-- Create Test User in Tenant Schema
-- This script creates a dev user in the test tenant with admin privileges
-- Password: medflow_test

-- Set search path to test tenant
SET search_path TO tenant_test_practice;

-- Insert dev user into tenant's users table
INSERT INTO users (
    id,
    email,
    first_name,
    last_name,
    password_hash,
    email_verified_at,
    status,
    settings
) VALUES (
    'a0000000-0000-0000-0000-000000000002',  -- Fixed UUID for dev user
    'mohammadamiri.py@gmail.com',
    'Mohammad',
    'Amiri',
    '$2a$10$K4.SJgxG.XpjlPu7GJCqLew.qmK5Yf2iKOQ0YLs2kCAM4NlLWlXRG',  -- Password: medflow_test
    NOW(),
    'active',
    '{}'::jsonb
) ON CONFLICT (email) DO UPDATE SET
    status = 'active',
    email_verified_at = NOW();

-- Assign admin role to dev user
INSERT INTO user_roles (user_id, role_id)
SELECT
    'a0000000-0000-0000-0000-000000000002',
    id
FROM roles
WHERE name = 'admin'
ON CONFLICT DO NOTHING;

-- Create an audit log entry
INSERT INTO audit_log (
    user_id,
    action,
    resource_type,
    resource_id,
    details
) VALUES (
    'a0000000-0000-0000-0000-000000000002',
    'user_created',
    'user',
    'a0000000-0000-0000-0000-000000000002',
    '{"method": "dev_setup_script", "environment": "development"}'::jsonb
);

\echo 'Dev user created in tenant_test_practice with admin role!'
