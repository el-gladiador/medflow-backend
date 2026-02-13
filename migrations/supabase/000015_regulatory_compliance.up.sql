-- MedFlow: Regulatory Compliance for German Medical Practice Inventory
-- MPBetreibV §13/§14, AMG medication tracking, cold-chain temperature monitoring
-- Adds: Bestandsverzeichnis fields, device inspections/trainings/incidents,
--        temperature readings, medication opening tracking

-- ============================================================================
-- 1a. ALTER inventory_items: Add MPBetreibV §14 columns + AMG opening tracking
-- ============================================================================
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS is_medical_device BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS device_type VARCHAR(100);
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS device_model VARCHAR(255);
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS authorized_representative TEXT;
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS importer TEXT;
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS operational_id_number VARCHAR(100);
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS location_assignment TEXT;
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS risk_class VARCHAR(20);
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS stk_interval_months INTEGER;
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS mtk_interval_months INTEGER;
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS last_stk_date DATE;
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS next_stk_due DATE;
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS last_mtk_date DATE;
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS next_mtk_due DATE;
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS shelf_life_after_opening_days INTEGER;

CREATE INDEX IF NOT EXISTS idx_inventory_items_medical_device ON inventory.inventory_items(is_medical_device) WHERE is_medical_device = TRUE AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inventory_items_stk_due ON inventory.inventory_items(next_stk_due) WHERE next_stk_due IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inventory_items_mtk_due ON inventory.inventory_items(next_mtk_due) WHERE next_mtk_due IS NOT NULL AND deleted_at IS NULL;

-- ============================================================================
-- 1b. ALTER storage_cabinets: Add temperature monitoring columns
-- ============================================================================
ALTER TABLE inventory.storage_cabinets ADD COLUMN IF NOT EXISTS min_temperature_celsius DECIMAL(5,2);
ALTER TABLE inventory.storage_cabinets ADD COLUMN IF NOT EXISTS max_temperature_celsius DECIMAL(5,2);
ALTER TABLE inventory.storage_cabinets ADD COLUMN IF NOT EXISTS temperature_monitoring_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- ============================================================================
-- 1c. New table: device_inspections (STK/MTK records for Medizinproduktebuch)
-- ============================================================================
CREATE TABLE inventory.device_inspections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE CASCADE,

    inspection_type VARCHAR(10) NOT NULL,
    inspection_date DATE NOT NULL,
    next_due_date DATE,
    result VARCHAR(50) NOT NULL,
    performed_by TEXT NOT NULL,
    report_reference VARCHAR(255),
    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT device_inspections_type_valid CHECK (inspection_type IN ('STK', 'MTK')),
    CONSTRAINT device_inspections_result_valid CHECK (result IN ('passed', 'failed', 'conditional'))
);

ALTER TABLE inventory.device_inspections ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.device_inspections FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.device_inspections
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_device_inspections_tenant ON inventory.device_inspections(tenant_id);
CREATE INDEX idx_device_inspections_item ON inventory.device_inspections(item_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_device_inspections_next_due ON inventory.device_inspections(next_due_date) WHERE deleted_at IS NULL AND next_due_date IS NOT NULL;

CREATE TRIGGER device_inspections_updated_at
    BEFORE UPDATE ON inventory.device_inspections
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.device_inspections TO medflow_app;

-- ============================================================================
-- 1d. New table: device_trainings (Einweisungen for Medizinproduktebuch)
-- ============================================================================
CREATE TABLE inventory.device_trainings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE CASCADE,

    training_date DATE NOT NULL,
    trainer_name TEXT NOT NULL,
    trainer_qualification TEXT,
    attendee_names TEXT NOT NULL,
    topic TEXT,
    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID
);

ALTER TABLE inventory.device_trainings ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.device_trainings FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.device_trainings
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_device_trainings_tenant ON inventory.device_trainings(tenant_id);
CREATE INDEX idx_device_trainings_item ON inventory.device_trainings(item_id) WHERE deleted_at IS NULL;

CREATE TRIGGER device_trainings_updated_at
    BEFORE UPDATE ON inventory.device_trainings
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.device_trainings TO medflow_app;

-- ============================================================================
-- 1e. New table: device_incidents (Vorkommnisse for Medizinproduktebuch)
-- ============================================================================
CREATE TABLE inventory.device_incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE CASCADE,

    incident_date DATE NOT NULL,
    incident_type VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    consequences TEXT,
    corrective_action TEXT,
    reported_to TEXT,
    report_date DATE,
    report_reference VARCHAR(255),
    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT device_incidents_type_valid CHECK (incident_type IN ('malfunction', 'near_miss', 'serious_incident'))
);

ALTER TABLE inventory.device_incidents ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.device_incidents FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.device_incidents
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_device_incidents_tenant ON inventory.device_incidents(tenant_id);
CREATE INDEX idx_device_incidents_item ON inventory.device_incidents(item_id) WHERE deleted_at IS NULL;

CREATE TRIGGER device_incidents_updated_at
    BEFORE UPDATE ON inventory.device_incidents
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.device_incidents TO medflow_app;

-- ============================================================================
-- 1f. New table: temperature_readings (cold-chain monitoring)
-- ============================================================================
CREATE TABLE inventory.temperature_readings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    cabinet_id UUID NOT NULL REFERENCES inventory.storage_cabinets(id) ON DELETE CASCADE,

    temperature_celsius DECIMAL(5,2) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    recorded_by UUID,
    source VARCHAR(20) NOT NULL DEFAULT 'manual',
    is_excursion BOOLEAN NOT NULL DEFAULT FALSE,
    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT temperature_readings_source_valid CHECK (source IN ('manual', 'webhook', 'sensor'))
);

ALTER TABLE inventory.temperature_readings ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.temperature_readings FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.temperature_readings
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_temperature_readings_tenant ON inventory.temperature_readings(tenant_id);
CREATE INDEX idx_temperature_readings_cabinet ON inventory.temperature_readings(cabinet_id, recorded_at DESC);
CREATE INDEX idx_temperature_readings_excursion ON inventory.temperature_readings(is_excursion) WHERE is_excursion = TRUE;

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.temperature_readings TO medflow_app;

-- ============================================================================
-- 1g. Expand alert type constraint to include new regulatory alert types
-- ============================================================================
ALTER TABLE inventory.inventory_alerts DROP CONSTRAINT IF EXISTS alerts_type_valid;
ALTER TABLE inventory.inventory_alerts ADD CONSTRAINT alerts_type_valid CHECK (
    alert_type IN (
        'low_stock', 'expiring_soon', 'expired', 'temperature', 'reorder',
        'out_of_stock',
        'stk_overdue', 'mtk_overdue', 'stk_due_soon', 'mtk_due_soon',
        'temperature_excursion', 'temperature_missing',
        'opening_expiry_soon', 'opening_expired'
    )
);

-- ============================================================================
-- 1h. Expand item document type constraint
-- ============================================================================
ALTER TABLE inventory.item_documents DROP CONSTRAINT IF EXISTS item_documents_type_valid;
ALTER TABLE inventory.item_documents ADD CONSTRAINT item_documents_type_valid CHECK (
    document_type IN ('sdb', 'manual', 'certificate', 'inspection_report', 'training_record')
);
