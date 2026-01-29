-- Add invitation-related audit actions
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'create_invitation';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'accept_invitation';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'revoke_invitation';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'resend_invitation';
