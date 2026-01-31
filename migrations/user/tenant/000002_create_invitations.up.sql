-- MedFlow Schema-per-Tenant: Tenant Schema
-- Migration: Create user_invitations table

CREATE TABLE IF NOT EXISTS user_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    token VARCHAR(64) NOT NULL UNIQUE,
    token_hash VARCHAR(255) NOT NULL,
    role_id UUID NOT NULL REFERENCES roles(id),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    accepted_at TIMESTAMP WITH TIME ZONE,
    accepted_user_id UUID REFERENCES users(id),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE,
    revoked_by UUID REFERENCES users(id),
    metadata JSONB DEFAULT '{}',
    CONSTRAINT valid_status CHECK (status IN ('pending', 'accepted', 'expired', 'revoked'))
);

-- Indexes for efficient lookup
CREATE INDEX IF NOT EXISTS idx_invitations_email ON user_invitations(email) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_invitations_token_hash ON user_invitations(token_hash) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_invitations_status ON user_invitations(status);
CREATE INDEX IF NOT EXISTS idx_invitations_expires_at ON user_invitations(expires_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_invitations_created_by ON user_invitations(created_by);

-- Unique constraint: only one pending invitation per email
CREATE UNIQUE INDEX IF NOT EXISTS idx_invitations_unique_pending_email ON user_invitations(email) WHERE status = 'pending';
