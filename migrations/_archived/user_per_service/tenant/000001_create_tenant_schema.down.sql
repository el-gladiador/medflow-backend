-- Rollback: Drop users and related tables
-- Note: audit_logs is handled by 000003_create_audit_logs.down.sql

DROP TRIGGER IF EXISTS users_updated_at ON users;
DROP TRIGGER IF EXISTS roles_updated_at ON roles;

DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;
