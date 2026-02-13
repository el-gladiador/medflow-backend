-- MedFlow: IfSG Hygiene Plans & Inspections
-- Implements hygiene plan management and inspection tracking per
-- Infektionsschutzgesetz (IfSG) requirements for medical practices.

-- ============================================================================
-- 1. inventory.hygiene_plans
-- Versioned hygiene plans with approval workflow
-- ============================================================================
CREATE TABLE inventory.hygiene_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    title VARCHAR(500) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    category VARCHAR(50) NOT NULL,

    content TEXT,

    effective_from DATE,
    effective_until DATE,

    approved_by UUID,
    approved_by_name VARCHAR(255),
    approved_at TIMESTAMPTZ,

    status VARCHAR(20) NOT NULL DEFAULT 'draft',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT hygiene_plans_category_valid CHECK (
        category IN (
            'surface_disinfection', 'hand_hygiene',
            'instrument_processing', 'waste_disposal', 'other'
        )
    ),
    CONSTRAINT hygiene_plans_status_valid CHECK (
        status IN ('draft', 'active', 'archived')
    ),
    CONSTRAINT hygiene_plans_title_version_unique UNIQUE (tenant_id, title, version)
);

ALTER TABLE inventory.hygiene_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.hygiene_plans FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.hygiene_plans
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_hygiene_plans_tenant ON inventory.hygiene_plans(tenant_id);
CREATE INDEX idx_hygiene_plans_status ON inventory.hygiene_plans(status)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_hygiene_plans_category ON inventory.hygiene_plans(category)
    WHERE deleted_at IS NULL;

CREATE TRIGGER hygiene_plans_updated_at
    BEFORE UPDATE ON inventory.hygiene_plans
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.hygiene_plans TO medflow_app;

-- ============================================================================
-- 2. inventory.hygiene_inspections
-- Inspection records linked to hygiene plans
-- ============================================================================
CREATE TABLE inventory.hygiene_inspections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    plan_id UUID REFERENCES inventory.hygiene_plans(id),

    inspection_date DATE NOT NULL,
    inspector_name VARCHAR(255) NOT NULL,
    area_inspected VARCHAR(500),

    checklist_results JSONB,
    overall_result VARCHAR(20) NOT NULL,
    corrective_actions TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT hygiene_inspections_result_valid CHECK (
        overall_result IN ('passed', 'failed', 'partial')
    )
);

ALTER TABLE inventory.hygiene_inspections ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.hygiene_inspections FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.hygiene_inspections
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_hygiene_inspections_tenant ON inventory.hygiene_inspections(tenant_id);
CREATE INDEX idx_hygiene_inspections_plan ON inventory.hygiene_inspections(plan_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_hygiene_inspections_date ON inventory.hygiene_inspections(inspection_date DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_hygiene_inspections_result ON inventory.hygiene_inspections(overall_result)
    WHERE deleted_at IS NULL;

CREATE TRIGGER hygiene_inspections_updated_at
    BEFORE UPDATE ON inventory.hygiene_inspections
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.hygiene_inspections TO medflow_app;
