-- MedFlow: Radiation Protection (Strahlenschutzverordnung)
-- Implements StrlSchV-compliant radiation device management:
--   - Radiation device registry
--   - Constancy tests (Konstanzprüfungen) per DIN 6868
--   - Expert inspections (Sachverständigenprüfungen)
--   - Staff radiation certifications (Fachkunde)
--   - Dosimetry records (30-year retention per StrlSchV §85)

-- ============================================================================
-- 1. inventory.radiation_devices
-- Registry of radiation-emitting devices (e.g., X-ray units)
-- ============================================================================
CREATE TABLE inventory.radiation_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id),
    device_category VARCHAR(100) NOT NULL,
    approval_number VARCHAR(100),
    location VARCHAR(500),

    responsible_person VARCHAR(255) NOT NULL,
    responsible_person_id UUID,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID
);

ALTER TABLE inventory.radiation_devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.radiation_devices FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.radiation_devices
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_radiation_devices_tenant ON inventory.radiation_devices(tenant_id);
CREATE INDEX idx_radiation_devices_item ON inventory.radiation_devices(item_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_radiation_devices_category ON inventory.radiation_devices(device_category)
    WHERE deleted_at IS NULL;

CREATE TRIGGER radiation_devices_updated_at
    BEFORE UPDATE ON inventory.radiation_devices
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.radiation_devices TO medflow_app;

-- ============================================================================
-- 2. inventory.constancy_tests
-- Konstanzprüfungen per DIN 6868 (daily, monthly, quarterly, annual)
-- ============================================================================
CREATE TABLE inventory.constancy_tests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    device_id UUID NOT NULL REFERENCES inventory.radiation_devices(id),

    test_date DATE NOT NULL,
    test_type VARCHAR(50) NOT NULL,
    result VARCHAR(20) NOT NULL,

    performed_by VARCHAR(255) NOT NULL,
    performed_by_id UUID,

    next_due_date DATE,
    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT constancy_tests_type_valid CHECK (
        test_type IN ('daily', 'monthly', 'quarterly', 'annual')
    ),
    CONSTRAINT constancy_tests_result_valid CHECK (
        result IN ('passed', 'failed', 'conditional')
    )
);

ALTER TABLE inventory.constancy_tests ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.constancy_tests FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.constancy_tests
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_constancy_tests_tenant ON inventory.constancy_tests(tenant_id);
CREATE INDEX idx_constancy_tests_device ON inventory.constancy_tests(device_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_constancy_tests_date ON inventory.constancy_tests(test_date DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_constancy_tests_next_due ON inventory.constancy_tests(next_due_date)
    WHERE deleted_at IS NULL AND next_due_date IS NOT NULL;

CREATE TRIGGER constancy_tests_updated_at
    BEFORE UPDATE ON inventory.constancy_tests
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.constancy_tests TO medflow_app;

-- ============================================================================
-- 3. inventory.expert_inspections
-- Sachverständigenprüfungen (external expert inspections)
-- ============================================================================
CREATE TABLE inventory.expert_inspections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    device_id UUID NOT NULL REFERENCES inventory.radiation_devices(id),

    inspection_date DATE NOT NULL,
    inspector_name VARCHAR(255) NOT NULL,
    inspector_organization VARCHAR(255),

    result VARCHAR(20) NOT NULL,
    report_reference VARCHAR(255),
    next_due_date DATE,
    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT expert_inspections_result_valid CHECK (
        result IN ('passed', 'failed', 'conditional')
    )
);

ALTER TABLE inventory.expert_inspections ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.expert_inspections FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.expert_inspections
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_expert_inspections_tenant ON inventory.expert_inspections(tenant_id);
CREATE INDEX idx_expert_inspections_device ON inventory.expert_inspections(device_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_expert_inspections_date ON inventory.expert_inspections(inspection_date DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_expert_inspections_next_due ON inventory.expert_inspections(next_due_date)
    WHERE deleted_at IS NULL AND next_due_date IS NOT NULL;

CREATE TRIGGER expert_inspections_updated_at
    BEFORE UPDATE ON inventory.expert_inspections
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.expert_inspections TO medflow_app;

-- ============================================================================
-- 4. inventory.staff_radiation_certifications
-- Fachkunde im Strahlenschutz (radiation protection qualifications)
-- ============================================================================
CREATE TABLE inventory.staff_radiation_certifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    employee_id UUID NOT NULL,
    employee_name VARCHAR(255) NOT NULL,

    certification_type VARCHAR(100) NOT NULL,
    issued_date DATE NOT NULL,
    expiry_date DATE NOT NULL,
    issuing_authority VARCHAR(255),
    certificate_number VARCHAR(100),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID
);

ALTER TABLE inventory.staff_radiation_certifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.staff_radiation_certifications FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.staff_radiation_certifications
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_staff_radiation_certs_tenant ON inventory.staff_radiation_certifications(tenant_id);
CREATE INDEX idx_staff_radiation_certs_employee ON inventory.staff_radiation_certifications(employee_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_staff_radiation_certs_expiry ON inventory.staff_radiation_certifications(expiry_date)
    WHERE deleted_at IS NULL;

CREATE TRIGGER staff_radiation_certifications_updated_at
    BEFORE UPDATE ON inventory.staff_radiation_certifications
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.staff_radiation_certifications TO medflow_app;

-- ============================================================================
-- 5. inventory.dosimetry_records
-- Personendosimetrie (StrlSchV §85: 30-year retention required)
-- ============================================================================
CREATE TABLE inventory.dosimetry_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    employee_id UUID NOT NULL,
    employee_name VARCHAR(255) NOT NULL,

    measurement_period_start DATE NOT NULL,
    measurement_period_end DATE NOT NULL,
    dosimeter_type VARCHAR(100) NOT NULL,

    dose_msv DECIMAL(10,4) NOT NULL,
    body_region VARCHAR(100) DEFAULT 'whole_body',

    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID
);

COMMENT ON TABLE inventory.dosimetry_records IS 'StrlSchV §85: 30-year retention required';

ALTER TABLE inventory.dosimetry_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.dosimetry_records FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.dosimetry_records
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_dosimetry_records_tenant ON inventory.dosimetry_records(tenant_id);
CREATE INDEX idx_dosimetry_records_employee ON inventory.dosimetry_records(employee_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_dosimetry_records_period ON inventory.dosimetry_records(measurement_period_start, measurement_period_end)
    WHERE deleted_at IS NULL;

CREATE TRIGGER dosimetry_records_updated_at
    BEFORE UPDATE ON inventory.dosimetry_records
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.dosimetry_records TO medflow_app;
