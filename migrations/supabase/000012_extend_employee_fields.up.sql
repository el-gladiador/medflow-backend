-- MedFlow: Extend employee fields to close frontend/backend/DB gaps
-- Adds missing columns that the frontend collects but the DB doesn't store.
-- Expands employment_type to include German-specific employment forms.

-- ============================================================================
-- 1. ADD MISSING COLUMNS TO staff.employees
-- ============================================================================

-- Place of birth (Geburtsort)
ALTER TABLE staff.employees ADD COLUMN IF NOT EXISTS birth_place VARCHAR(100);

-- Marital status (Familienstand) - German civil status categories
ALTER TABLE staff.employees ADD COLUMN IF NOT EXISTS marital_status VARCHAR(50);
ALTER TABLE staff.employees ADD CONSTRAINT employees_marital_status_valid CHECK (
    marital_status IS NULL OR marital_status IN (
        'single', 'married', 'divorced', 'widowed', 'civil_partnership'
    )
);

-- Contract type (Vertragsart)
ALTER TABLE staff.employees ADD COLUMN IF NOT EXISTS contract_type VARCHAR(50);
ALTER TABLE staff.employees ADD CONSTRAINT employees_contract_type_valid CHECK (
    contract_type IS NULL OR contract_type IN ('permanent', 'fixed_term')
);

-- Weekly working hours (Wochenstunden)
ALTER TABLE staff.employees ADD COLUMN IF NOT EXISTS weekly_hours DECIMAL(5,2);

-- Annual vacation days (Urlaubstage)
ALTER TABLE staff.employees ADD COLUMN IF NOT EXISTS vacation_days INTEGER;

-- Working time model (Arbeitszeitmodell)
ALTER TABLE staff.employees ADD COLUMN IF NOT EXISTS work_time_model VARCHAR(50);
ALTER TABLE staff.employees ADD CONSTRAINT employees_work_time_model_valid CHECK (
    work_time_model IS NULL OR work_time_model IN ('fixed', 'flex', 'trust', 'shift')
);

-- ============================================================================
-- 2. EXPAND employment_type CHECK CONSTRAINT
-- Adds German-specific employment forms: minijob, working_student, auxiliary
-- ============================================================================

ALTER TABLE staff.employees DROP CONSTRAINT IF EXISTS employees_employment_type_valid;
ALTER TABLE staff.employees ADD CONSTRAINT employees_employment_type_valid CHECK (
    employment_type IN (
        'full_time', 'part_time', 'contractor', 'intern', 'temporary',
        'minijob', 'working_student', 'auxiliary'
    )
);

-- ============================================================================
-- 3. GRANT permissions to medflow_app role for new columns
-- (Existing table grants cover new columns automatically in PostgreSQL,
--  but be explicit for clarity)
-- ============================================================================

-- No additional GRANT needed - ALTER TABLE ADD COLUMN inherits existing table-level grants.

-- ============================================================================
-- 4. Create employee_files table (referenced by Go code but missing from migrations)
-- This is a simplified file metadata table for uploaded employee files.
-- Note: employee_documents already exists for formal HR documents (contracts, IDs, etc.)
-- employee_files is for ad-hoc file uploads via the staff UI.
-- ============================================================================

CREATE TABLE IF NOT EXISTS staff.employee_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    file_type VARCHAR(100) NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER,
    mime_type VARCHAR(100),
    category VARCHAR(100),

    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    uploaded_by UUID,

    CONSTRAINT employee_files_category_valid CHECK (
        category IS NULL OR category IN (
            'contract', 'id_document', 'certificate', 'qualification',
            'medical', 'payroll', 'other'
        )
    )
);

ALTER TABLE staff.employee_files ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.employee_files
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX IF NOT EXISTS idx_employee_files_tenant ON staff.employee_files(tenant_id);
CREATE INDEX IF NOT EXISTS idx_employee_files_employee ON staff.employee_files(employee_id);

-- Grant to medflow_app
GRANT SELECT, INSERT, UPDATE, DELETE ON staff.employee_files TO medflow_app;

-- ============================================================================
-- 5. FORCE RLS on employee_files for medflow_app
-- ============================================================================
ALTER TABLE staff.employee_files FORCE ROW LEVEL SECURITY;
