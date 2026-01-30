-- Rollback: Drop users and related tables

DROP TRIGGER IF EXISTS users_updated_at ON users;
DROP TRIGGER IF EXISTS roles_updated_at ON roles;

DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;
