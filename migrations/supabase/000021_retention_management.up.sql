-- MedFlow: Document Retention Management
-- Configurable retention policies per German healthcare regulations.
-- Each entity type has a legally mandated minimum retention period.

-- ============================================================================
-- 1. inventory.retention_policies
-- Per-tenant configurable retention periods with legal basis references
-- ============================================================================
CREATE TABLE inventory.retention_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    entity_type VARCHAR(50) NOT NULL,
    retention_years INTEGER NOT NULL,
    legal_basis TEXT,
    description TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT retention_policies_entity_type_unique UNIQUE (tenant_id, entity_type)
);

ALTER TABLE inventory.retention_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.retention_policies FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.retention_policies
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_retention_policies_tenant ON inventory.retention_policies(tenant_id);
CREATE INDEX idx_retention_policies_entity ON inventory.retention_policies(entity_type)
    WHERE deleted_at IS NULL;

CREATE TRIGGER retention_policies_updated_at
    BEFORE UPDATE ON inventory.retention_policies
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.retention_policies TO medflow_app;

-- ============================================================================
-- 2. Seed function for default retention policies
-- Cannot INSERT directly because RLS requires tenant context.
-- Call this function when onboarding a new tenant.
-- ============================================================================
CREATE OR REPLACE FUNCTION inventory.seed_retention_policies(p_tenant_id UUID)
RETURNS void AS $$
    INSERT INTO inventory.retention_policies
        (tenant_id, entity_type, retention_years, legal_basis, description)
    VALUES
        (p_tenant_id, 'device_book', 20, 'MPBetreibV §12', 'Medizinproduktebuch entries'),
        (p_tenant_id, 'btm_register', 3, 'BtMG §13 Abs. 3', 'Betäubungsmittelbuch entries'),
        (p_tenant_id, 'sterilization', 15, 'RKI/KRINKO', 'Sterilization batch records'),
        (p_tenant_id, 'working_time', 2, 'ArbZG §16', 'Working time records'),
        (p_tenant_id, 'radiation', 30, 'StrlSchV §85', 'Dosimetry and radiation records'),
        (p_tenant_id, 'tax_documents', 10, 'AO §147', 'Tax-relevant inventory documents'),
        (p_tenant_id, 'hazardous_substances', 40, 'GefStoffV §14', 'Gefahrstoffverzeichnis entries')
    ON CONFLICT (tenant_id, entity_type) DO NOTHING;
$$ LANGUAGE sql;
