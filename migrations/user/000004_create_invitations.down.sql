-- Remove indexes
DROP INDEX IF EXISTS idx_invitations_unique_pending_email;
DROP INDEX IF EXISTS idx_invitations_created_by;
DROP INDEX IF EXISTS idx_invitations_expires_at;
DROP INDEX IF EXISTS idx_invitations_status;
DROP INDEX IF EXISTS idx_invitations_token_hash;
DROP INDEX IF EXISTS idx_invitations_email;

-- Remove table
DROP TABLE IF EXISTS user_invitations;
