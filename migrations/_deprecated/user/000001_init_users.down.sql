DROP TRIGGER IF EXISTS update_roles_updated_at ON roles;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS audit_logs;
DROP TYPE IF EXISTS audit_action;
DROP TABLE IF EXISTS permission_overrides;
DROP TABLE IF EXISTS access_giver_scope;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TYPE IF EXISTS role_level;
