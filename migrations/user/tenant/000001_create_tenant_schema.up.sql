-- MedFlow Schema-per-Tenant: Tenant Schema
-- Migration: Create users table and related auth tables

-- Users table (tenant-specific users)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Identity
    email VARCHAR(255) NOT NULL UNIQUE,
    username VARCHAR(100),

    -- Profile
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    avatar_url TEXT,

    -- Authentication
    password_hash VARCHAR(255) NOT NULL,
    email_verified_at TIMESTAMPTZ,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    last_login_at TIMESTAMPTZ,
    failed_login_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,

    -- Settings
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- Constraints
    CONSTRAINT users_status_valid CHECK (status IN ('active', 'inactive', 'suspended', 'pending')),
    CONSTRAINT users_email_format CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$')
);

-- Sessions table
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Token
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    refresh_token_hash VARCHAR(255),

    -- Device info
    device_name VARCHAR(255),
    device_type VARCHAR(50),
    ip_address INET,
    user_agent TEXT,

    -- Validity
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Roles table (custom roles per tenant)
-- Uses JSONB array for permissions - simpler and more flexible than separate permission tables
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Identity
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    display_name_de VARCHAR(255),  -- German display name
    description TEXT,

    -- Role type flags
    is_system BOOLEAN NOT NULL DEFAULT FALSE,       -- System roles can't be deleted
    is_default BOOLEAN NOT NULL DEFAULT FALSE,      -- Assigned to new users
    is_manager BOOLEAN NOT NULL DEFAULT FALSE,      -- Can manage other users
    can_receive_delegation BOOLEAN NOT NULL DEFAULT FALSE,  -- Can receive delegated permissions

    -- Role hierarchy level (higher = more privileges)
    level INTEGER NOT NULL DEFAULT 10,

    -- Permissions as JSONB array of strings
    -- Supports wildcards: "*" (all), "staff.*" (all staff), "inventory.read" (specific)
    permissions JSONB NOT NULL DEFAULT '[]'::jsonb,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- User-Role assignments
CREATE TABLE user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,

    -- Assignment metadata
    assigned_by UUID REFERENCES users(id),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT user_roles_unique UNIQUE (user_id, role_id)
);

-- Note: audit_logs table is created in 000003_create_audit_logs.up.sql
-- This avoids duplication and ensures a single canonical audit table

-- Indexes
CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_status ON users(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_last_login ON users(last_login_at DESC) WHERE deleted_at IS NULL;

CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(token_hash);
CREATE INDEX idx_sessions_expires ON sessions(expires_at) WHERE revoked_at IS NULL;

CREATE INDEX idx_roles_name ON roles(name) WHERE deleted_at IS NULL;

CREATE INDEX idx_user_roles_user ON user_roles(user_id);
CREATE INDEX idx_user_roles_role ON user_roles(role_id);

-- Note: audit_logs indexes are created in 000003_create_audit_logs.up.sql

-- Triggers
CREATE TRIGGER users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER roles_updated_at
    BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- Insert default roles with all fields
INSERT INTO roles (id, name, display_name, display_name_de, description, is_system, is_default, is_manager, can_receive_delegation, level, permissions) VALUES
    (gen_random_uuid(), 'admin', 'Administrator', 'Administrator', 'Full access to all features', TRUE, FALSE, TRUE, FALSE, 100, '["*"]'::jsonb),
    (gen_random_uuid(), 'manager', 'Manager', 'Manager', 'Manage staff, inventory, and reports', TRUE, FALSE, TRUE, TRUE, 50,
        '["staff.*", "inventory.*", "reports.*", "users.read"]'::jsonb),
    (gen_random_uuid(), 'staff', 'Staff Member', 'Mitarbeiter', 'Basic access for daily operations', TRUE, TRUE, FALSE, FALSE, 10,
        '["inventory.read", "inventory.adjust", "profile.*"]'::jsonb);
