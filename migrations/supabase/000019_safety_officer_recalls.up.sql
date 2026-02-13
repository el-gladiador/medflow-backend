-- MedFlow: Safety Officer & Field Safety Notice (Recall) Management
-- Implements MPBetreibV safety officer designation and BfArM field safety
-- notice tracking with automated inventory matching.

-- ============================================================================
-- 1. inventory.safety_officers
-- Designated safety officers (Sicherheitsbeauftragte) per MPBetreibV
-- ============================================================================
CREATE TABLE inventory.safety_officers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    user_id UUID NOT NULL,
    user_name VARCHAR(255) NOT NULL,
    qualification TEXT,
    designation_date DATE NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID
);

ALTER TABLE inventory.safety_officers ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.safety_officers FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.safety_officers
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_safety_officers_tenant ON inventory.safety_officers(tenant_id);
CREATE INDEX idx_safety_officers_user ON inventory.safety_officers(user_id);
CREATE INDEX idx_safety_officers_active ON inventory.safety_officers(is_active)
    WHERE deleted_at IS NULL AND is_active = TRUE;

CREATE TRIGGER safety_officers_updated_at
    BEFORE UPDATE ON inventory.safety_officers
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.safety_officers TO medflow_app;

-- ============================================================================
-- 2. inventory.field_safety_notices
-- BfArM field safety notices, manufacturer recalls, urgent safety notices
-- ============================================================================
CREATE TABLE inventory.field_safety_notices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    notice_number VARCHAR(100) NOT NULL,
    notice_type VARCHAR(30) NOT NULL,
    severity VARCHAR(20) NOT NULL,

    title VARCHAR(500) NOT NULL,
    description TEXT,

    manufacturer VARCHAR(255),
    affected_product VARCHAR(500),
    affected_batch_numbers TEXT,
    affected_udi_dis TEXT,
    affected_serial_numbers TEXT,

    source VARCHAR(50),
    source_url TEXT,
    notice_date DATE,
    received_date DATE NOT NULL,

    status VARCHAR(30) NOT NULL DEFAULT 'open',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT field_safety_notices_type_valid CHECK (
        notice_type IN ('recall', 'safety_notice', 'urgent_field_safety')
    ),
    CONSTRAINT field_safety_notices_severity_valid CHECK (
        severity IN ('class_I', 'class_II', 'class_III')
    ),
    CONSTRAINT field_safety_notices_source_valid CHECK (
        source IN ('bfarm', 'manufacturer', 'distributor', 'other')
    ),
    CONSTRAINT field_safety_notices_status_valid CHECK (
        status IN ('open', 'in_progress', 'resolved', 'not_applicable')
    )
);

ALTER TABLE inventory.field_safety_notices ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.field_safety_notices FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.field_safety_notices
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_field_safety_notices_tenant ON inventory.field_safety_notices(tenant_id);
CREATE INDEX idx_field_safety_notices_status ON inventory.field_safety_notices(status)
    WHERE deleted_at IS NULL AND status IN ('open', 'in_progress');
CREATE INDEX idx_field_safety_notices_notice_number ON inventory.field_safety_notices(tenant_id, notice_number);

CREATE TRIGGER field_safety_notices_updated_at
    BEFORE UPDATE ON inventory.field_safety_notices
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.field_safety_notices TO medflow_app;

-- ============================================================================
-- 3. inventory.recall_matches
-- Links field safety notices to affected inventory items/batches
-- ============================================================================
CREATE TABLE inventory.recall_matches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    notice_id UUID NOT NULL REFERENCES inventory.field_safety_notices(id),
    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id),
    batch_id UUID REFERENCES inventory.inventory_batches(id),

    match_type VARCHAR(30) NOT NULL,
    matched_value VARCHAR(500),

    action_taken TEXT,
    action_date TIMESTAMPTZ,
    action_by UUID,

    status VARCHAR(30) NOT NULL DEFAULT 'pending',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT recall_matches_match_type_valid CHECK (
        match_type IN ('batch_number', 'udi_di', 'serial_number', 'product_name')
    ),
    CONSTRAINT recall_matches_status_valid CHECK (
        status IN ('pending', 'quarantined', 'returned', 'disposed', 'resolved')
    )
);

ALTER TABLE inventory.recall_matches ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.recall_matches FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.recall_matches
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_recall_matches_tenant ON inventory.recall_matches(tenant_id);
CREATE INDEX idx_recall_matches_notice ON inventory.recall_matches(notice_id);
CREATE INDEX idx_recall_matches_item ON inventory.recall_matches(item_id);
CREATE INDEX idx_recall_matches_status ON inventory.recall_matches(status)
    WHERE deleted_at IS NULL AND status IN ('pending', 'quarantined');

CREATE TRIGGER recall_matches_updated_at
    BEFORE UPDATE ON inventory.recall_matches
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.recall_matches TO medflow_app;
