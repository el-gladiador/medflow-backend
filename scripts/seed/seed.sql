-- MedFlow: Development Seed Data
-- Creates two test tenants with admin users for local development.
-- Run with: make seed
-- NEVER run this against production.

-- ============================================================================
-- SEED TENANTS
-- ============================================================================
INSERT INTO public.tenants (id, slug, name, email, subscription_tier, subscription_status)
VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'test-practice', 'Zahnarztpraxis Dr. Mueller', 'info@praxis-mueller.de', 'standard', 'active'),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'demo-clinic', 'Gemeinschaftspraxis am Park', 'info@praxis-park.de', 'premium', 'active')
ON CONFLICT (slug) DO UPDATE SET
    deleted_at = NULL,
    subscription_status = EXCLUDED.subscription_status;

-- ============================================================================
-- SEED DEFAULT ROLES (per tenant)
-- ============================================================================

-- Roles for test-practice
INSERT INTO users.roles (id, tenant_id, name, display_name, display_name_de, description, is_system, is_default, is_manager, can_receive_delegation, level, permissions)
VALUES
    ('a1a00001-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'admin', 'Administrator', 'Administrator', 'Full access to all features', TRUE, FALSE, TRUE, FALSE, 100, '["*"]'::jsonb),
    ('a1a00001-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'manager', 'Manager', 'Manager', 'Manage staff, inventory, and reports', TRUE, FALSE, TRUE, TRUE, 50, '["staff.*", "inventory.*", "reports.*", "users.read"]'::jsonb),
    ('a1a00001-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'staff', 'Staff Member', 'Mitarbeiter', 'Basic access for daily operations', TRUE, TRUE, FALSE, FALSE, 10, '["inventory.read", "inventory.adjust", "profile.*"]'::jsonb)
ON CONFLICT (tenant_id, name) DO NOTHING;

-- Roles for demo-clinic
INSERT INTO users.roles (id, tenant_id, name, display_name, display_name_de, description, is_system, is_default, is_manager, can_receive_delegation, level, permissions)
VALUES
    ('b2b00002-0000-0000-0000-000000000001', 'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'admin', 'Administrator', 'Administrator', 'Full access to all features', TRUE, FALSE, TRUE, FALSE, 100, '["*"]'::jsonb),
    ('b2b00002-0000-0000-0000-000000000002', 'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'manager', 'Manager', 'Manager', 'Manage staff, inventory, and reports', TRUE, FALSE, TRUE, TRUE, 50, '["staff.*", "inventory.*", "reports.*", "users.read"]'::jsonb),
    ('b2b00002-0000-0000-0000-000000000003', 'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'staff', 'Staff Member', 'Mitarbeiter', 'Basic access for daily operations', TRUE, TRUE, FALSE, FALSE, 10, '["inventory.read", "inventory.adjust", "profile.*"]'::jsonb)
ON CONFLICT (tenant_id, name) DO NOTHING;

-- ============================================================================
-- SEED ADMIN USERS (per tenant)
-- Password for both: Admin123!
-- ============================================================================

-- Admin user for test-practice
INSERT INTO users.users (id, tenant_id, email, first_name, last_name, password_hash, email_verified_at, status)
VALUES (
    'a1000001-0000-0000-0000-000000000001',
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'admin@praxis-mueller.de',
    'Max',
    'Mueller',
    '$2a$10$0epFj2hIu615UrRCnVtWlum8DmA0mR3xNREU3CTM1Rq1PaM8.psWe',
    NOW(),
    'active'
) ON CONFLICT (tenant_id, email) DO NOTHING;

-- Admin user for demo-clinic
INSERT INTO users.users (id, tenant_id, email, first_name, last_name, password_hash, email_verified_at, status)
VALUES (
    'a2000002-0000-0000-0000-000000000001',
    'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22',
    'admin@praxis-park.de',
    'Lisa',
    'Schmidt',
    '$2a$10$kjLMnnj8ajfFnDKEJ56ZYuJnyE.Tegla23ZlwWO7gUIrw2Tqr1HZS',
    NOW(),
    'active'
) ON CONFLICT (tenant_id, email) DO NOTHING;

-- ============================================================================
-- SEED USER-TENANT LOOKUP (O(1) login resolution)
-- ============================================================================
INSERT INTO public.user_tenant_lookup (email, user_id, tenant_id, tenant_slug)
VALUES
    ('admin@praxis-mueller.de', 'a1000001-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'test-practice'),
    ('admin@praxis-park.de', 'a2000002-0000-0000-0000-000000000001', 'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'demo-clinic')
ON CONFLICT (email) DO NOTHING;

-- ============================================================================
-- SEED USER ROLE ASSIGNMENTS
-- ============================================================================
INSERT INTO users.user_roles (tenant_id, user_id, role_id)
SELECT 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a1000001-0000-0000-0000-000000000001', id
FROM users.roles WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND name = 'admin'
ON CONFLICT (tenant_id, user_id, role_id) DO NOTHING;

INSERT INTO users.user_roles (tenant_id, user_id, role_id)
SELECT 'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'a2000002-0000-0000-0000-000000000001', id
FROM users.roles WHERE tenant_id = 'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22' AND name = 'admin'
ON CONFLICT (tenant_id, user_id, role_id) DO NOTHING;

-- ============================================================================
-- SEED ADMIN EMPLOYEE RECORDS (staff.employees)
-- ============================================================================

-- Employee record for test-practice admin (Max Mueller)
INSERT INTO staff.employees (
    id, tenant_id, user_id, first_name, last_name,
    email, employee_number, job_title, department,
    employment_type, hire_date, status, show_in_staff_list
)
VALUES (
    'e1000001-0000-0000-0000-000000000001',
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'a1000001-0000-0000-0000-000000000001',
    'Max',
    'Mueller',
    'admin@praxis-mueller.de',
    'EMP-001',
    'Praxisinhaber',
    'Verwaltung',
    'full_time',
    '2024-01-01',
    'active',
    true
) ON CONFLICT DO NOTHING;

-- Employee record for demo-clinic admin (Lisa Schmidt)
INSERT INTO staff.employees (
    id, tenant_id, user_id, first_name, last_name,
    email, employee_number, job_title, department,
    employment_type, hire_date, status, show_in_staff_list
)
VALUES (
    'e2000002-0000-0000-0000-000000000001',
    'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22',
    'a2000002-0000-0000-0000-000000000001',
    'Lisa',
    'Schmidt',
    'admin@praxis-park.de',
    'EMP-001',
    'Praxisinhaberin',
    'Verwaltung',
    'full_time',
    '2024-01-01',
    'active',
    true
) ON CONFLICT DO NOTHING;

-- ============================================================================
-- SEED COMPLIANCE SETTINGS (per tenant)
-- ============================================================================
INSERT INTO staff.compliance_settings (tenant_id)
VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22')
ON CONFLICT (tenant_id) DO NOTHING;
