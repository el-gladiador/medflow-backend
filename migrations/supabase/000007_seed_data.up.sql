-- MedFlow RLS Multi-Tenancy: Seed Data
-- Seeds two demo tenants with admin users for local development

-- ============================================================================
-- SEED TENANTS
-- ============================================================================
INSERT INTO public.tenants (id, slug, name, email, subscription_tier, subscription_status)
VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'test-practice', 'Zahnarztpraxis Dr. Müller', 'info@praxis-mueller.de', 'standard', 'active'),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'demo-clinic', 'Gemeinschaftspraxis am Park', 'info@praxis-park.de', 'premium', 'active')
ON CONFLICT (slug) DO NOTHING;

-- ============================================================================
-- SEED DEFAULT ROLES (per tenant)
-- ============================================================================

-- Roles for test-practice
INSERT INTO users.roles (id, tenant_id, name, display_name, display_name_de, description, is_system, is_default, is_manager, can_receive_delegation, level, permissions)
VALUES
    (gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'admin', 'Administrator', 'Administrator', 'Full access to all features', TRUE, FALSE, TRUE, FALSE, 100, '["*"]'::jsonb),
    (gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'manager', 'Manager', 'Manager', 'Manage staff, inventory, and reports', TRUE, FALSE, TRUE, TRUE, 50, '["staff.*", "inventory.*", "reports.*", "users.read"]'::jsonb),
    (gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'staff', 'Staff Member', 'Mitarbeiter', 'Basic access for daily operations', TRUE, TRUE, FALSE, FALSE, 10, '["inventory.read", "inventory.adjust", "profile.*"]'::jsonb)
ON CONFLICT (tenant_id, name) DO NOTHING;

-- Roles for demo-clinic
INSERT INTO users.roles (id, tenant_id, name, display_name, display_name_de, description, is_system, is_default, is_manager, can_receive_delegation, level, permissions)
VALUES
    (gen_random_uuid(), 'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'admin', 'Administrator', 'Administrator', 'Full access to all features', TRUE, FALSE, TRUE, FALSE, 100, '["*"]'::jsonb),
    (gen_random_uuid(), 'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'manager', 'Manager', 'Manager', 'Manage staff, inventory, and reports', TRUE, FALSE, TRUE, TRUE, 50, '["staff.*", "inventory.*", "reports.*", "users.read"]'::jsonb),
    (gen_random_uuid(), 'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'staff', 'Staff Member', 'Mitarbeiter', 'Basic access for daily operations', TRUE, TRUE, FALSE, FALSE, 10, '["inventory.read", "inventory.adjust", "profile.*"]'::jsonb)
ON CONFLICT (tenant_id, name) DO NOTHING;

-- ============================================================================
-- SEED COMPLIANCE SETTINGS (per tenant)
-- ============================================================================
INSERT INTO staff.compliance_settings (tenant_id)
VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22')
ON CONFLICT (tenant_id) DO NOTHING;
