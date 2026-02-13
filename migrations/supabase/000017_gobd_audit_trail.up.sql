-- MedFlow: GoBD-compliant Audit Trail
-- Append-only, immutable audit log for all inventory entity changes.
-- Satisfies GoBD (Grundsätze ordnungsmäßiger Buchführung) requirements:
--   - No modification or deletion of entries
--   - Full traceability of all changes with field-level diffs

-- ============================================================================
-- inventory.audit_trail (append-only, immutable)
-- ============================================================================
CREATE TABLE inventory.audit_trail (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,

    field_changes JSONB,
    metadata JSONB,

    performed_by UUID,
    performed_by_name VARCHAR(255),
    ip_address VARCHAR(45),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT audit_trail_entity_type_valid CHECK (
        entity_type IN (
            'item', 'batch', 'alert', 'hazardous', 'inspection',
            'training', 'incident', 'temperature', 'document'
        )
    ),
    CONSTRAINT audit_trail_action_valid CHECK (
        action IN (
            'create', 'update', 'delete', 'adjust',
            'open', 'btm_receipt', 'btm_dispense', 'btm_disposal',
            'btm_correction', 'btm_check', 'recall_match', 'recall_resolve'
        )
    )
);

-- NO updated_at column (immutable)
-- NO deleted_at column (immutable)
-- NO update trigger (immutable)

ALTER TABLE inventory.audit_trail ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.audit_trail FORCE ROW LEVEL SECURITY;

-- Separate SELECT and INSERT policies (no UPDATE/DELETE allowed)
CREATE POLICY audit_trail_select ON inventory.audit_trail
    FOR SELECT USING (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE POLICY audit_trail_insert ON inventory.audit_trail
    FOR INSERT WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

-- Indexes
CREATE INDEX idx_audit_trail_tenant ON inventory.audit_trail(tenant_id);
CREATE INDEX idx_audit_trail_entity ON inventory.audit_trail(entity_type, entity_id);
CREATE INDEX idx_audit_trail_created ON inventory.audit_trail(created_at);
CREATE INDEX idx_audit_trail_performed_by ON inventory.audit_trail(performed_by);

-- Append-only: SELECT + INSERT only, NO UPDATE, NO DELETE
GRANT SELECT, INSERT ON inventory.audit_trail TO medflow_app;
