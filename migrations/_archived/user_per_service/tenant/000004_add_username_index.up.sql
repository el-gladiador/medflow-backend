-- MedFlow: Add username index for login support
-- Migration: Enable username-based login alongside email
--
-- This index enables efficient username lookups during authentication.
-- Username is optional, so the index uses a partial condition.

-- Create unique index on username for efficient lookups and uniqueness enforcement
CREATE UNIQUE INDEX idx_users_username ON users(username)
WHERE username IS NOT NULL AND deleted_at IS NULL;

-- Comment
COMMENT ON COLUMN users.username IS
    'Optional username for login. Users can log in with either email or username.';
