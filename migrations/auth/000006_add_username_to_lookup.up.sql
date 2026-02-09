-- MedFlow: Add username support to user-tenant lookup table
-- Migration: Enable username-based login in addition to email
--
-- This allows users to log in with either their email or username.
-- Username lookup is also O(1) thanks to the index.

-- Add username column to the lookup table
ALTER TABLE public.user_tenant_lookup
ADD COLUMN username VARCHAR(100);

-- Create index for username lookups during login
CREATE INDEX idx_user_tenant_lookup_username ON public.user_tenant_lookup(username)
WHERE username IS NOT NULL;

-- Add unique constraint on username (within the lookup table)
-- Note: This is safe because usernames are unique within a tenant,
-- and we only have one entry per user in this table
CREATE UNIQUE INDEX idx_user_tenant_lookup_username_unique ON public.user_tenant_lookup(username)
WHERE username IS NOT NULL;

-- Comments
COMMENT ON COLUMN public.user_tenant_lookup.username IS
    'Optional username for login. Enables login with username instead of email.';
