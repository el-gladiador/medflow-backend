-- MedFlow RLS Multi-Tenancy: Users Schema Tables
-- All tables have tenant_id + RLS policies for row-level tenant isolation

-- ============================================================================
-- USERS
-- ============================================================================
CREATE TABLE users.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    -- Identity
    email VARCHAR(255) NOT NULL,
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

    -- Constraints (compound uniqueness with tenant_id)
    CONSTRAINT users_tenant_email_unique UNIQUE (tenant_id, email),
    CONSTRAINT users_status_valid CHECK (status IN ('active', 'inactive', 'suspended', 'pending')),
    CONSTRAINT users_email_format CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$')
);

-- RLS
ALTER TABLE users.users ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON users.users
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

-- Indexes
CREATE INDEX idx_users_tenant ON users.users(tenant_id);
CREATE INDEX idx_users_email ON users.users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_status ON users.users(tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_last_login ON users.users(last_login_at DESC) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_users_username ON users.users(tenant_id, username)
    WHERE username IS NOT NULL AND deleted_at IS NULL;

CREATE TRIGGER users_updated_at
    BEFORE UPDATE ON users.users
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- ROLES
-- ============================================================================
CREATE TABLE users.roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    -- Identity
    name VARCHAR(100) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    display_name_de VARCHAR(255),
    description TEXT,

    -- Role type flags
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    is_manager BOOLEAN NOT NULL DEFAULT FALSE,
    can_receive_delegation BOOLEAN NOT NULL DEFAULT FALSE,

    -- Hierarchy
    level INTEGER NOT NULL DEFAULT 10,

    -- Permissions as JSONB array
    permissions JSONB NOT NULL DEFAULT '[]'::jsonb,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT roles_tenant_name_unique UNIQUE (tenant_id, name)
);

ALTER TABLE users.roles ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON users.roles
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_roles_tenant ON users.roles(tenant_id);
CREATE INDEX idx_roles_name ON users.roles(tenant_id, name) WHERE deleted_at IS NULL;

CREATE TRIGGER roles_updated_at
    BEFORE UPDATE ON users.roles
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- USER ROLES (assignments)
-- ============================================================================
CREATE TABLE users.user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    user_id UUID NOT NULL REFERENCES users.users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES users.roles(id) ON DELETE CASCADE,

    assigned_by UUID REFERENCES users.users(id),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT user_roles_tenant_unique UNIQUE (tenant_id, user_id, role_id)
);

ALTER TABLE users.user_roles ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON users.user_roles
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_user_roles_tenant ON users.user_roles(tenant_id);
CREATE INDEX idx_user_roles_user ON users.user_roles(user_id);
CREATE INDEX idx_user_roles_role ON users.user_roles(role_id);

-- ============================================================================
-- AUDIT LOGS
-- ============================================================================
CREATE TABLE users.audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    actor_id UUID,
    actor_name VARCHAR(255) NOT NULL,
    action VARCHAR(100) NOT NULL,

    resource_type VARCHAR(100),
    resource_id UUID,

    target_user_id UUID,
    target_user_name VARCHAR(255),

    old_values JSONB,
    new_values JSONB,
    details JSONB,

    ip_address INET,
    user_agent TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE users.audit_logs ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON users.audit_logs
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_audit_logs_tenant ON users.audit_logs(tenant_id);
CREATE INDEX idx_audit_logs_actor ON users.audit_logs(actor_id);
CREATE INDEX idx_audit_logs_action ON users.audit_logs(action);
CREATE INDEX idx_audit_logs_created ON users.audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_resource ON users.audit_logs(resource_type, resource_id);

-- ============================================================================
-- SESSIONS (per-tenant sessions)
-- ============================================================================
CREATE TABLE users.sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    user_id UUID NOT NULL REFERENCES users.users(id) ON DELETE CASCADE,

    token_hash VARCHAR(255) NOT NULL UNIQUE,
    refresh_token_hash VARCHAR(255),

    device_name VARCHAR(255),
    device_type VARCHAR(50),
    ip_address INET,
    user_agent TEXT,

    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE users.sessions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON users.sessions
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_user_sessions_tenant ON users.sessions(tenant_id);
CREATE INDEX idx_user_sessions_user ON users.sessions(user_id);
CREATE INDEX idx_user_sessions_token ON users.sessions(token_hash);
CREATE INDEX idx_user_sessions_expires ON users.sessions(expires_at) WHERE revoked_at IS NULL;
