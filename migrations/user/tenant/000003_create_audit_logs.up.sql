-- MedFlow Schema-per-Tenant: Tenant Schema
-- Migration: Create unified audit_logs table
-- This is the canonical audit table for all user-related audit events

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Actor (who performed the action)
    actor_id UUID REFERENCES users(id),
    actor_name VARCHAR(255) NOT NULL,

    -- Action (what was done)
    action VARCHAR(100) NOT NULL,

    -- Resource (target of the action)
    resource_type VARCHAR(100),
    resource_id UUID,

    -- User-specific target (optional, for user management actions)
    target_user_id UUID REFERENCES users(id),
    target_user_name VARCHAR(255),

    -- Change tracking (for data modifications)
    old_values JSONB,
    new_values JSONB,

    -- Additional context
    details JSONB,

    -- Request context
    ip_address INET,
    user_agent TEXT,

    -- Timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient lookup
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target ON audit_logs(target_user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource_type, resource_id);

-- Comment explaining the table purpose
COMMENT ON TABLE audit_logs IS 'Unified audit log for all tenant-specific user and permission events';
