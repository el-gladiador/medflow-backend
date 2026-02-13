-- MedFlow: BioStoffV (Biostoffverordnung) Register
-- Implements biological agent risk assessment and training tracking
-- per BioStoffV requirements for medical/dental practices.

-- ============================================================================
-- 1. ALTER inventory_items: Add biological agent columns
-- ============================================================================
ALTER TABLE inventory.inventory_items
    ADD COLUMN IF NOT EXISTS is_biological_agent BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE inventory.inventory_items
    ADD COLUMN IF NOT EXISTS bio_risk_group INTEGER;

ALTER TABLE inventory.inventory_items
    ADD CONSTRAINT inventory_items_bio_risk_group_valid
    CHECK (bio_risk_group IS NULL OR (bio_risk_group >= 1 AND bio_risk_group <= 4));

CREATE INDEX IF NOT EXISTS idx_inventory_items_biological
    ON inventory.inventory_items(is_biological_agent)
    WHERE is_biological_agent = TRUE AND deleted_at IS NULL;

-- ============================================================================
-- 2. inventory.bio_risk_assessments
-- Gefährdungsbeurteilung per BioStoffV for biological agents
-- ============================================================================
CREATE TABLE inventory.bio_risk_assessments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id),
    risk_group INTEGER NOT NULL,

    assessment_date DATE NOT NULL,
    assessor_name VARCHAR(255) NOT NULL,

    exposure_routes TEXT,
    protective_measures TEXT,
    operating_instructions_ref VARCHAR(500),
    valid_until DATE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT bio_risk_assessments_risk_group_valid
        CHECK (risk_group >= 1 AND risk_group <= 4)
);

ALTER TABLE inventory.bio_risk_assessments ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.bio_risk_assessments FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.bio_risk_assessments
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_bio_risk_assessments_tenant ON inventory.bio_risk_assessments(tenant_id);
CREATE INDEX idx_bio_risk_assessments_item ON inventory.bio_risk_assessments(item_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_bio_risk_assessments_valid ON inventory.bio_risk_assessments(valid_until)
    WHERE deleted_at IS NULL AND valid_until IS NOT NULL;

CREATE TRIGGER bio_risk_assessments_updated_at
    BEFORE UPDATE ON inventory.bio_risk_assessments
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.bio_risk_assessments TO medflow_app;

-- ============================================================================
-- 3. inventory.bio_trainings
-- BioStoffV training records (Unterweisung)
-- ============================================================================
CREATE TABLE inventory.bio_trainings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    training_type VARCHAR(30) NOT NULL,
    training_date DATE NOT NULL,
    trainer_name VARCHAR(255) NOT NULL,
    attendee_names TEXT NOT NULL,
    topic TEXT,
    next_due_date DATE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT bio_trainings_type_valid CHECK (
        training_type IN ('initial', 'annual_refresh', 'special')
    )
);

ALTER TABLE inventory.bio_trainings ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.bio_trainings FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.bio_trainings
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_bio_trainings_tenant ON inventory.bio_trainings(tenant_id);
CREATE INDEX idx_bio_trainings_date ON inventory.bio_trainings(training_date)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_bio_trainings_next_due ON inventory.bio_trainings(next_due_date)
    WHERE deleted_at IS NULL AND next_due_date IS NOT NULL;

CREATE TRIGGER bio_trainings_updated_at
    BEFORE UPDATE ON inventory.bio_trainings
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.bio_trainings TO medflow_app;
