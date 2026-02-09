-- Rollback: Remove username index

DROP INDEX IF EXISTS idx_users_username;
