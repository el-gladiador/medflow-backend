-- Remove seeded data
DELETE FROM users WHERE email = 'admin@praxis.de';
DELETE FROM role_permissions;
DELETE FROM permissions;
DELETE FROM roles;
