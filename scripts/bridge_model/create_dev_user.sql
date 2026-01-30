-- Run against User database (postgres-users:5434)
-- Creates dev user in tenant schema

SET search_path TO tenant_test_practice;

-- Create default roles first
INSERT INTO roles (id, name, display_name, description) VALUES
('192c6b0a-ab32-46ce-9a64-7ccbfa46f357', 'admin', 'Administrator', 'Full access to all features'),
('c7cfe02c-9bc8-402d-9b58-30bec7d8d95c', 'manager', 'Manager', 'Manage staff, inventory, and reports'),
('b7d06c53-a3b4-4ee9-ace7-fe40c9d343ee', 'staff', 'Staff Member', 'Basic access for daily operations')
ON CONFLICT (id) DO NOTHING;

-- Create dev user
INSERT INTO users (
    id,
    email,
    first_name,
    last_name,
    password_hash,
    email_verified_at,
    status
) VALUES (
    'a0000000-0000-0000-0000-000000000002',
    'mohammadamiri.py@gmail.com',
    'Mohammad',
    'Amiri',
    '$2a$10$K4.SJgxG.XpjlPu7GJCqLew.qmK5Yf2iKOQ0YLs2kCAM4NlLWlXRG',  -- Password: medflow_test
    NOW(),
    'active'
) ON CONFLICT (email) DO NOTHING;

-- Assign admin role
INSERT INTO user_roles (user_id, role_id)
VALUES ('a0000000-0000-0000-0000-000000000002', '192c6b0a-ab32-46ce-9a64-7ccbfa46f357')
ON CONFLICT DO NOTHING;

\echo 'Dev user created in user service tenant schema!'
