-- MedFlow: KRINKO/RKI Reprocessing & Sterilization Tracking
-- Implements instrument reprocessing documentation per KRINKO/RKI guidelines:
--   - Risk categorization (unkritisch through kritisch C)
--   - Sterilization batch tracking with BI/CI/Bowie-Dick indicators
--   - Per-instrument reprocessing cycle history

-- ============================================================================
-- 1. ALTER inventory_items: Add KRINKO risk category
-- ============================================================================
ALTER TABLE inventory.inventory_items
    ADD COLUMN IF NOT EXISTS krinko_risk_category VARCHAR(20);

ALTER TABLE inventory.inventory_items
    ADD CONSTRAINT inventory_items_krinko_category_valid
    CHECK (
        krinko_risk_category IS NULL OR krinko_risk_category IN (
            'unkritisch', 'semikritisch_A', 'semikritisch_B',
            'kritisch_A', 'kritisch_B', 'kritisch_C'
        )
    );

CREATE INDEX IF NOT EXISTS idx_inventory_items_krinko
    ON inventory.inventory_items(krinko_risk_category)
    WHERE krinko_risk_category IS NOT NULL AND deleted_at IS NULL;

-- ============================================================================
-- 2. inventory.sterilization_batches
-- Sterilization load/cycle records with indicator results
-- ============================================================================
CREATE TABLE inventory.sterilization_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    batch_number VARCHAR(100) NOT NULL,
    sterilizer_id UUID,
    sterilizer_name VARCHAR(255) NOT NULL,

    program_number VARCHAR(50),
    cycle_date TIMESTAMPTZ NOT NULL,

    -- Process parameters
    temperature_celsius DECIMAL(5,2),
    pressure_bar DECIMAL(5,2),
    hold_time_minutes INTEGER,

    -- Indicator results
    bi_result VARCHAR(20),
    ci_result VARCHAR(20),
    bowie_dick_result VARCHAR(20),
    overall_result VARCHAR(20) NOT NULL,

    -- Release
    released_by UUID,
    released_by_name VARCHAR(255),
    release_date TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT sterilization_bi_result_valid CHECK (
        bi_result IS NULL OR bi_result IN ('passed', 'failed', 'pending')
    ),
    CONSTRAINT sterilization_ci_result_valid CHECK (
        ci_result IS NULL OR ci_result IN ('passed', 'failed', 'pending')
    ),
    CONSTRAINT sterilization_bowie_dick_valid CHECK (
        bowie_dick_result IS NULL OR bowie_dick_result IN ('passed', 'failed', 'not_applicable')
    ),
    CONSTRAINT sterilization_overall_result_valid CHECK (
        overall_result IN ('passed', 'failed', 'pending')
    )
);

ALTER TABLE inventory.sterilization_batches ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.sterilization_batches FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.sterilization_batches
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_sterilization_batches_tenant ON inventory.sterilization_batches(tenant_id);
CREATE INDEX idx_sterilization_batches_date ON inventory.sterilization_batches(cycle_date DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_sterilization_batches_result ON inventory.sterilization_batches(overall_result)
    WHERE deleted_at IS NULL;

CREATE TRIGGER sterilization_batches_updated_at
    BEFORE UPDATE ON inventory.sterilization_batches
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.sterilization_batches TO medflow_app;

-- ============================================================================
-- 3. inventory.reprocessing_cycles
-- Per-instrument reprocessing history linked to sterilization batches
-- ============================================================================
CREATE TABLE inventory.reprocessing_cycles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id),
    sterilization_batch_id UUID REFERENCES inventory.sterilization_batches(id),

    cycle_number INTEGER NOT NULL,
    cycle_date TIMESTAMPTZ NOT NULL,

    cleaning_method VARCHAR(100),
    disinfection_method VARCHAR(100),
    sterilization_method VARCHAR(100),

    bi_indicator_result VARCHAR(20),
    ci_indicator_result VARCHAR(20),

    released_by UUID,
    released_by_name VARCHAR(255),
    release_date TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT reprocessing_bi_result_valid CHECK (
        bi_indicator_result IS NULL OR bi_indicator_result IN ('passed', 'failed', 'pending')
    ),
    CONSTRAINT reprocessing_ci_result_valid CHECK (
        ci_indicator_result IS NULL OR ci_indicator_result IN ('passed', 'failed', 'pending')
    )
);

ALTER TABLE inventory.reprocessing_cycles ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.reprocessing_cycles FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.reprocessing_cycles
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_reprocessing_cycles_tenant ON inventory.reprocessing_cycles(tenant_id);
CREATE INDEX idx_reprocessing_cycles_item ON inventory.reprocessing_cycles(item_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_reprocessing_cycles_batch ON inventory.reprocessing_cycles(sterilization_batch_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_reprocessing_cycles_date ON inventory.reprocessing_cycles(cycle_date DESC)
    WHERE deleted_at IS NULL;

CREATE TRIGGER reprocessing_cycles_updated_at
    BEFORE UPDATE ON inventory.reprocessing_cycles
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.reprocessing_cycles TO medflow_app;
