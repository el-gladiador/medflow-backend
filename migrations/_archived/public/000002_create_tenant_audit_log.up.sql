-- MedFlow Schema-per-Tenant: Public Schema
-- Migration: Create tenant_audit_log table (system-wide audit of tenant lifecycle)

CREATE TABLE public.tenant_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    -- Event details
    event_type VARCHAR(50) NOT NULL,
    event_data JSONB NOT NULL DEFAULT '{}',

    -- Who performed the action
    performed_by UUID,
    performed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Request context
    ip_address INET,
    user_agent TEXT,

    -- Constraints
    CONSTRAINT tenant_audit_event_type_valid CHECK (
        event_type IN (
            'created',
            'updated',
            'suspended',
            'reactivated',
            'deleted',
            'schema_migrated',
            'tier_changed',
            'settings_updated',
            'kms_key_created',
            'kms_key_deleted',
            'data_exported',
            'user_invited',
            'user_removed'
        )
    )
);

-- Indexes
CREATE INDEX idx_tenant_audit_tenant ON public.tenant_audit_log(tenant_id);
CREATE INDEX idx_tenant_audit_type ON public.tenant_audit_log(event_type);
CREATE INDEX idx_tenant_audit_time ON public.tenant_audit_log(performed_at DESC);
CREATE INDEX idx_tenant_audit_performed_by ON public.tenant_audit_log(performed_by) WHERE performed_by IS NOT NULL;

-- Comments
COMMENT ON TABLE public.tenant_audit_log IS 'System-wide audit log for tenant lifecycle events (GDPR compliance)';
COMMENT ON COLUMN public.tenant_audit_log.event_data IS 'JSON containing event-specific details';
COMMENT ON COLUMN public.tenant_audit_log.performed_by IS 'Admin user ID who performed the action';
