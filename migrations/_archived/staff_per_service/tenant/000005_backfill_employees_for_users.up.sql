-- MedFlow Schema-per-Tenant: Tenant Schema
-- Migration: Backfill employee records for existing users
-- This migration creates employee records for all users in user_cache who don't have employee records yet

INSERT INTO employees (
    id,
    user_id,
    first_name,
    last_name,
    email,
    employment_type,
    hire_date,
    status,
    job_title,
    created_at,
    updated_at
)
SELECT
    gen_random_uuid(),
    u.user_id,
    u.first_name,
    u.last_name,
    u.email,
    'full_time',
    CURRENT_DATE,  -- Use current date as hire date since we don't have user creation date
    'active',
    CASE
        WHEN u.role_name = 'admin' THEN 'Administrator'
        WHEN u.role_name = 'manager' THEN 'Manager'
        WHEN u.role_name = 'staff' THEN 'Staff Member'
        WHEN u.role_name = 'viewer' THEN 'Viewer'
        ELSE INITCAP(u.role_name)  -- Capitalize unknown roles
    END,
    NOW(),
    NOW()
FROM user_cache u
WHERE NOT EXISTS (
    SELECT 1
    FROM employees e
    WHERE e.user_id = u.user_id
);
