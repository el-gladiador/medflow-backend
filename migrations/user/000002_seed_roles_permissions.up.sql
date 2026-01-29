-- Seed Roles and Permissions
-- Based on src/config/roles.ts from frontend

-- Insert Roles
INSERT INTO roles (name, display_name, display_name_de, description, level, is_manager, can_receive_delegation) VALUES
('admin', 'Administrator', 'Administrator', 'Practice owner/administrator with full system access', '100', true, false),
('manager', 'Manager', 'Verwaltung/Leitung', 'Manager/HR with staff and scheduling management', '80', true, true),
('MFA', 'Medical Assistant', 'Medizinische Fachangestellte', 'Medical assistant with basic employee access', '50', false, false),
('Pflege', 'Nurse', 'Pflegekraft', 'Nurse/caregiver with basic employee access', '40', false, false),
('Praktikant', 'Intern', 'Praktikant', 'Intern with most restricted access', '30', false, false),
('other', 'Other', 'Sonstige', 'Flexible/custom role with minimal base permissions', '10', false, false);

-- Insert Permissions by category
INSERT INTO permissions (name, description, category, is_admin_only) VALUES
-- Vacation permissions
('vacation:view_calendar', 'View the absence calendar', 'vacation', false),
('vacation:view_all', 'View all vacation details (not just own)', 'vacation', false),
('vacation:request', 'Request vacation for self', 'vacation', false),
('vacation:approve', 'Approve/reject vacation requests', 'vacation', false),
('vacation:edit', 'Edit any vacation entry', 'vacation', false),
('vacation:delete', 'Delete vacation entries', 'vacation', false),
('vacation:add_for_others', 'Add absence entries for other employees', 'vacation', false),

-- Staff permissions
('staff:view', 'View staff list', 'staff', false),
('staff:edit', 'Edit staff details', 'staff', false),
('staff:add', 'Add new staff members', 'staff', false),
('staff:delete', 'Delete staff members', 'staff', false),
('staff:view_details', 'View certificates, documents, contact info', 'staff', false),
('staff:view_sensitive', 'View birthday, employment type, start date', 'staff', false),

-- Schedule permissions
('schedule:view', 'View schedule/shifts', 'schedule', false),
('schedule:edit', 'Edit shifts', 'schedule', false),
('schedule:manage', 'Full schedule management', 'schedule', false),
('schedule:view_team', 'See all team schedules', 'schedule', false),
('schedule:view_own', 'See only own schedule', 'schedule', false),

-- Reports permissions
('reports:view_team', 'See team reports', 'reports', false),
('reports:view_own', 'See personal reports only', 'reports', false),

-- Role management permissions
('roles:view', 'View role configuration', 'roles', false),
('roles:manage', 'Manage roles', 'roles', true),

-- User management permissions
('users:create', 'Create new users', 'users', false),
('users:delete', 'Delete users', 'users', true),

-- Access delegation permissions
('access:delegate', 'Delegate access giver status', 'access', true),
('access:grant', 'Grant/revoke permissions to others', 'access', false),

-- Audit permissions
('audit:view', 'View permission audit logs', 'audit', false),

-- Inventory permissions
('inventory:view', 'View inventory', 'inventory', false),
('inventory:edit', 'Edit inventory', 'inventory', false),
('inventory:manage', 'Full inventory management', 'inventory', false);

-- Map permissions to roles

-- Admin gets all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'admin';

-- Manager permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'manager'
AND p.name IN (
    'vacation:view_calendar', 'vacation:view_all', 'vacation:request', 'vacation:approve',
    'vacation:edit', 'vacation:delete', 'vacation:add_for_others',
    'staff:view', 'staff:edit', 'staff:add', 'staff:view_details', 'staff:view_sensitive',
    'schedule:view', 'schedule:edit', 'schedule:manage', 'schedule:view_team',
    'reports:view_team',
    'users:create',
    'roles:view',
    'access:grant',
    'inventory:view', 'inventory:edit'
);

-- MFA permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'MFA'
AND p.name IN (
    'vacation:view_calendar', 'vacation:request',
    'staff:view',
    'schedule:view', 'schedule:view_own',
    'reports:view_own',
    'inventory:view'
);

-- Pflege permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'Pflege'
AND p.name IN (
    'vacation:view_calendar', 'vacation:request',
    'staff:view',
    'schedule:view', 'schedule:view_own',
    'reports:view_own',
    'inventory:view'
);

-- Praktikant permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'Praktikant'
AND p.name IN (
    'vacation:view_calendar', 'vacation:request',
    'staff:view',
    'schedule:view', 'schedule:view_own',
    'reports:view_own'
);

-- Other role permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'other'
AND p.name IN (
    'staff:view',
    'schedule:view_own'
);

-- Create default admin user (password: admin123)
-- BCrypt hash for 'admin123': $2a$10$rQEY7bNVHp5LCXFJ4eF4eOz5Qft5ZxCnH.7KGfKtZwLOw7L1QXXP6
INSERT INTO users (email, password_hash, name, avatar, role_id, is_active, is_access_giver)
SELECT
    'admin@praxis.de',
    '$2a$10$rQEY7bNVHp5LCXFJ4eF4eOz5Qft5ZxCnH.7KGfKtZwLOw7L1QXXP6',
    'Admin User',
    '/placeholder-avatar.png',
    r.id,
    true,
    true
FROM roles r WHERE r.name = 'admin';
