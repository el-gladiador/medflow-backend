-- Run against User database (postgres-users:5434)
-- Creates dev user in SECOND tenant schema for multi-tenancy testing

SET search_path TO tenant_demo_clinic;

-- Roles are auto-created by tenant schema migration, no need to insert

-- Create demo user
INSERT INTO users (
    id,
    email,
    first_name,
    last_name,
    password_hash,
    email_verified_at,
    status
) VALUES (
    'b0000000-0000-0000-0000-000000000002',
    'demo@medflow.de',
    'Anna',
    'Schmidt',
    '$2a$10$K4.SJgxG.XpjlPu7GJCqLew.qmK5Yf2iKOQ0YLs2kCAM4NlLWlXRG',  -- Password: medflow_test
    NOW(),
    'active'
) ON CONFLICT (email) DO NOTHING;

-- Assign admin role (get the auto-generated admin role ID)
INSERT INTO user_roles (user_id, role_id)
SELECT 'b0000000-0000-0000-0000-000000000002', id
FROM roles WHERE name = 'admin'
ON CONFLICT DO NOTHING;

\echo 'Demo user created in second tenant (demo-clinic) schema!'
