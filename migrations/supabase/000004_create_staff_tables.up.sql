-- MedFlow RLS Multi-Tenancy: Staff Schema Tables
-- All tables have tenant_id + RLS policies for row-level tenant isolation

-- ============================================================================
-- EMPLOYEES
-- ============================================================================
CREATE TABLE staff.employees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    user_id UUID,

    -- Personal info
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    date_of_birth DATE,
    gender VARCHAR(20),
    nationality VARCHAR(100),

    -- Contact
    email VARCHAR(255),
    phone VARCHAR(50),
    mobile VARCHAR(50),

    -- Employment details
    employee_number VARCHAR(50),
    job_title VARCHAR(255),
    department VARCHAR(100),
    employment_type VARCHAR(50) NOT NULL DEFAULT 'full_time',

    -- Dates
    hire_date DATE NOT NULL,
    termination_date DATE,
    probation_end_date DATE,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'active',

    -- Profile
    avatar_url TEXT,
    notes TEXT,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT employees_tenant_number_unique UNIQUE (tenant_id, employee_number),
    CONSTRAINT employees_employment_type_valid CHECK (
        employment_type IN ('full_time', 'part_time', 'contractor', 'intern', 'temporary')
    ),
    CONSTRAINT employees_status_valid CHECK (
        status IN ('active', 'on_leave', 'suspended', 'terminated', 'pending')
    ),
    CONSTRAINT employees_email_format CHECK (
        email IS NULL OR email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'
    )
);

ALTER TABLE staff.employees ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.employees
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_employees_tenant ON staff.employees(tenant_id);
CREATE INDEX idx_employees_user ON staff.employees(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_employees_status ON staff.employees(tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_employees_department ON staff.employees(tenant_id, department) WHERE deleted_at IS NULL;
CREATE INDEX idx_employees_number ON staff.employees(tenant_id, employee_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_employees_name ON staff.employees(last_name, first_name) WHERE deleted_at IS NULL;
CREATE INDEX idx_employees_hire_date ON staff.employees(hire_date) WHERE deleted_at IS NULL;

CREATE TRIGGER employees_updated_at
    BEFORE UPDATE ON staff.employees
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- EMPLOYEE ADDRESSES
-- ============================================================================
CREATE TABLE staff.employee_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,

    address_type VARCHAR(50) NOT NULL DEFAULT 'home',
    street VARCHAR(255) NOT NULL,
    house_number VARCHAR(20),
    address_line2 VARCHAR(255),
    postal_code VARCHAR(20) NOT NULL,
    city VARCHAR(100) NOT NULL,
    state VARCHAR(100),
    country VARCHAR(100) NOT NULL DEFAULT 'Germany',
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT employee_addresses_type_valid CHECK (
        address_type IN ('home', 'mailing', 'emergency')
    )
);

ALTER TABLE staff.employee_addresses ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.employee_addresses
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_employee_addresses_tenant ON staff.employee_addresses(tenant_id);
CREATE INDEX idx_employee_addresses_employee ON staff.employee_addresses(employee_id);

CREATE TRIGGER employee_addresses_updated_at
    BEFORE UPDATE ON staff.employee_addresses
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- EMPLOYEE CONTACTS (emergency)
-- ============================================================================
CREATE TABLE staff.employee_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,

    contact_type VARCHAR(50) NOT NULL DEFAULT 'emergency',
    name VARCHAR(255) NOT NULL,
    relationship VARCHAR(100),
    phone VARCHAR(50) NOT NULL,
    email VARCHAR(255),
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT employee_contacts_type_valid CHECK (
        contact_type IN ('emergency', 'family', 'doctor', 'other')
    )
);

ALTER TABLE staff.employee_contacts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.employee_contacts
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_employee_contacts_tenant ON staff.employee_contacts(tenant_id);
CREATE INDEX idx_employee_contacts_employee ON staff.employee_contacts(employee_id);

CREATE TRIGGER employee_contacts_updated_at
    BEFORE UPDATE ON staff.employee_contacts
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- EMPLOYEE FINANCIALS (German payroll)
-- ============================================================================
CREATE TABLE staff.employee_financials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,

    iban VARCHAR(34),
    bic VARCHAR(11),
    bank_name VARCHAR(255),
    account_holder VARCHAR(255),

    tax_id VARCHAR(20),
    tax_class VARCHAR(10),
    church_tax BOOLEAN DEFAULT FALSE,
    child_allowance DECIMAL(5,2) DEFAULT 0,

    salary_type VARCHAR(50) DEFAULT 'monthly',
    base_salary_cents INTEGER,
    currency VARCHAR(3) DEFAULT 'EUR',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT employee_financials_tenant_unique UNIQUE (tenant_id, employee_id),
    CONSTRAINT employee_financials_salary_type_valid CHECK (
        salary_type IN ('hourly', 'monthly', 'annual')
    )
);

ALTER TABLE staff.employee_financials ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.employee_financials
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_employee_financials_tenant ON staff.employee_financials(tenant_id);

CREATE TRIGGER employee_financials_updated_at
    BEFORE UPDATE ON staff.employee_financials
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- EMPLOYEE SOCIAL INSURANCE (German specific)
-- ============================================================================
CREATE TABLE staff.employee_social_insurance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,

    social_security_number VARCHAR(20),
    health_insurance_provider VARCHAR(255),
    health_insurance_number VARCHAR(50),

    pension_insurance BOOLEAN DEFAULT TRUE,
    unemployment_insurance BOOLEAN DEFAULT TRUE,
    health_insurance BOOLEAN DEFAULT TRUE,
    care_insurance BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT employee_social_insurance_tenant_unique UNIQUE (tenant_id, employee_id)
);

ALTER TABLE staff.employee_social_insurance ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.employee_social_insurance
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_employee_social_insurance_tenant ON staff.employee_social_insurance(tenant_id);

CREATE TRIGGER employee_social_insurance_updated_at
    BEFORE UPDATE ON staff.employee_social_insurance
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- EMPLOYEE DOCUMENTS
-- ============================================================================
CREATE TABLE staff.employee_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,

    document_type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,

    file_path TEXT NOT NULL,
    file_size_bytes INTEGER,
    mime_type VARCHAR(100),

    issue_date DATE,
    expiry_date DATE,

    status VARCHAR(50) NOT NULL DEFAULT 'active',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    uploaded_by UUID,

    CONSTRAINT employee_documents_type_valid CHECK (
        document_type IN (
            'contract', 'id_card', 'passport', 'work_permit',
            'certificate', 'qualification', 'training',
            'medical', 'other'
        )
    ),
    CONSTRAINT employee_documents_status_valid CHECK (
        status IN ('active', 'expired', 'superseded', 'archived')
    )
);

ALTER TABLE staff.employee_documents ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.employee_documents
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_employee_documents_tenant ON staff.employee_documents(tenant_id);
CREATE INDEX idx_employee_documents_employee ON staff.employee_documents(employee_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_employee_documents_type ON staff.employee_documents(document_type) WHERE deleted_at IS NULL;
CREATE INDEX idx_employee_documents_expiry ON staff.employee_documents(expiry_date) WHERE deleted_at IS NULL AND expiry_date IS NOT NULL;

CREATE TRIGGER employee_documents_updated_at
    BEFORE UPDATE ON staff.employee_documents
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- SHIFT TEMPLATES
-- ============================================================================
CREATE TABLE staff.shift_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    name VARCHAR(100) NOT NULL,
    description TEXT,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    break_duration_minutes INTEGER NOT NULL DEFAULT 0,
    shift_type VARCHAR(50) NOT NULL DEFAULT 'regular',
    color VARCHAR(20) DEFAULT '#22c55e',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,

    CONSTRAINT shift_templates_type_valid CHECK (
        shift_type IN ('regular', 'on_call', 'emergency', 'night')
    )
);

ALTER TABLE staff.shift_templates ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.shift_templates
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_shift_templates_tenant ON staff.shift_templates(tenant_id);
CREATE INDEX idx_shift_templates_active ON staff.shift_templates(is_active) WHERE deleted_at IS NULL;

CREATE TRIGGER shift_templates_updated_at
    BEFORE UPDATE ON staff.shift_templates
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- SHIFT ASSIGNMENTS
-- ============================================================================
CREATE TABLE staff.shift_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,
    shift_template_id UUID REFERENCES staff.shift_templates(id),

    shift_date DATE NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    break_duration_minutes INTEGER NOT NULL DEFAULT 0,
    shift_type VARCHAR(50) NOT NULL DEFAULT 'regular',
    status VARCHAR(50) NOT NULL DEFAULT 'scheduled',
    has_conflict BOOLEAN NOT NULL DEFAULT FALSE,
    conflict_reason TEXT,
    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT shift_assignments_type_valid CHECK (
        shift_type IN ('regular', 'on_call', 'emergency', 'night')
    ),
    CONSTRAINT shift_assignments_status_valid CHECK (
        status IN ('scheduled', 'confirmed', 'completed', 'cancelled')
    )
);

ALTER TABLE staff.shift_assignments ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.shift_assignments
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_shift_assignments_tenant ON staff.shift_assignments(tenant_id);
CREATE INDEX idx_shift_assignments_employee ON staff.shift_assignments(employee_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_shift_assignments_date ON staff.shift_assignments(shift_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_shift_assignments_employee_date ON staff.shift_assignments(employee_id, shift_date) WHERE deleted_at IS NULL;

CREATE TRIGGER shift_assignments_updated_at
    BEFORE UPDATE ON staff.shift_assignments
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- ABSENCES (vacation, sick leave, etc.)
-- ============================================================================
CREATE TABLE staff.absences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,

    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    absence_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_by UUID,
    reviewed_at TIMESTAMPTZ,
    rejection_reason TEXT,
    vacation_days_used DECIMAL(4,2),
    employee_note TEXT,
    manager_note TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT absences_type_valid CHECK (
        absence_type IN (
            'vacation', 'sick', 'sick_child', 'training', 'special_leave',
            'unpaid_leave', 'parental_leave', 'comp_time', 'other'
        )
    ),
    CONSTRAINT absences_status_valid CHECK (
        status IN ('pending', 'approved', 'rejected', 'cancelled')
    ),
    CONSTRAINT absences_dates_valid CHECK (end_date >= start_date)
);

ALTER TABLE staff.absences ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.absences
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_absences_tenant ON staff.absences(tenant_id);
CREATE INDEX idx_absences_employee ON staff.absences(employee_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_absences_dates ON staff.absences(start_date, end_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_absences_status ON staff.absences(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_absences_pending ON staff.absences(status) WHERE status = 'pending' AND deleted_at IS NULL;

CREATE TRIGGER absences_updated_at
    BEFORE UPDATE ON staff.absences
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- VACATION BALANCES
-- ============================================================================
CREATE TABLE staff.vacation_balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,

    year INTEGER NOT NULL,
    annual_entitlement DECIMAL(5,2) NOT NULL DEFAULT 30,
    carryover_from_previous DECIMAL(5,2) NOT NULL DEFAULT 0,
    additional_granted DECIMAL(5,2) NOT NULL DEFAULT 0,
    taken DECIMAL(5,2) NOT NULL DEFAULT 0,
    planned DECIMAL(5,2) NOT NULL DEFAULT 0,
    pending DECIMAL(5,2) NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT vacation_balances_tenant_unique UNIQUE (tenant_id, employee_id, year),
    CONSTRAINT vacation_balances_year_valid CHECK (year >= 2000 AND year <= 2100),
    CONSTRAINT vacation_balances_entitlement_valid CHECK (annual_entitlement >= 0),
    CONSTRAINT vacation_balances_taken_valid CHECK (taken >= 0),
    CONSTRAINT vacation_balances_planned_valid CHECK (planned >= 0),
    CONSTRAINT vacation_balances_pending_valid CHECK (pending >= 0)
);

ALTER TABLE staff.vacation_balances ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.vacation_balances
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_vacation_balances_tenant ON staff.vacation_balances(tenant_id);
CREATE INDEX idx_vacation_balances_employee ON staff.vacation_balances(employee_id);

CREATE TRIGGER vacation_balances_updated_at
    BEFORE UPDATE ON staff.vacation_balances
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- TIME ENTRIES
-- ============================================================================
CREATE TABLE staff.time_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,

    entry_date DATE NOT NULL,
    clock_in TIMESTAMPTZ NOT NULL,
    clock_out TIMESTAMPTZ,
    total_work_minutes INTEGER NOT NULL DEFAULT 0,
    total_break_minutes INTEGER NOT NULL DEFAULT 0,
    notes TEXT,
    is_manual_entry BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID
);

ALTER TABLE staff.time_entries ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.time_entries
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_time_entries_tenant ON staff.time_entries(tenant_id);
CREATE INDEX idx_time_entries_employee_date ON staff.time_entries(employee_id, entry_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_time_entries_date ON staff.time_entries(entry_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_time_entries_active ON staff.time_entries(employee_id, clock_out) WHERE deleted_at IS NULL AND clock_out IS NULL;

CREATE TRIGGER time_entries_updated_at
    BEFORE UPDATE ON staff.time_entries
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- TIME BREAKS
-- ============================================================================
CREATE TABLE staff.time_breaks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    time_entry_id UUID NOT NULL REFERENCES staff.time_entries(id) ON DELETE CASCADE,

    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE staff.time_breaks ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.time_breaks
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_time_breaks_tenant ON staff.time_breaks(tenant_id);
CREATE INDEX idx_time_breaks_entry ON staff.time_breaks(time_entry_id);
CREATE INDEX idx_time_breaks_active ON staff.time_breaks(time_entry_id, end_time) WHERE end_time IS NULL;

CREATE TRIGGER time_breaks_updated_at
    BEFORE UPDATE ON staff.time_breaks
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- TIME CORRECTIONS
-- ============================================================================
CREATE TABLE staff.time_corrections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,
    time_entry_id UUID REFERENCES staff.time_entries(id) ON DELETE SET NULL,

    correction_date DATE NOT NULL,
    original_clock_in TIMESTAMPTZ,
    original_clock_out TIMESTAMPTZ,
    corrected_clock_in TIMESTAMPTZ,
    corrected_clock_out TIMESTAMPTZ,
    reason TEXT NOT NULL,
    corrected_by UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

ALTER TABLE staff.time_corrections ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.time_corrections
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_time_corrections_tenant ON staff.time_corrections(tenant_id);
CREATE INDEX idx_time_corrections_employee ON staff.time_corrections(employee_id, correction_date) WHERE deleted_at IS NULL;

CREATE TRIGGER time_corrections_updated_at
    BEFORE UPDATE ON staff.time_corrections
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- ARBZG COMPLIANCE LOG
-- ============================================================================
CREATE TABLE staff.arbzg_compliance_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id) ON DELETE CASCADE,
    time_entry_id UUID,

    violation_date DATE NOT NULL,
    violation_type VARCHAR(100) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'warning',
    description TEXT NOT NULL,
    details JSONB,
    acknowledged_by UUID,
    acknowledged_at TIMESTAMPTZ,
    resolution_note TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT arbzg_compliance_severity_valid CHECK (
        severity IN ('warning', 'violation', 'critical')
    )
);

ALTER TABLE staff.arbzg_compliance_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.arbzg_compliance_log
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_arbzg_compliance_tenant ON staff.arbzg_compliance_log(tenant_id);
CREATE INDEX idx_arbzg_compliance_employee ON staff.arbzg_compliance_log(employee_id);
CREATE INDEX idx_arbzg_compliance_date ON staff.arbzg_compliance_log(violation_date);
CREATE INDEX idx_arbzg_compliance_unack ON staff.arbzg_compliance_log(acknowledged_at) WHERE acknowledged_at IS NULL;

-- ============================================================================
-- COMPLIANCE VIOLATIONS
-- ============================================================================
CREATE TABLE staff.compliance_violations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id),

    violation_type VARCHAR(50) NOT NULL,
    violation_date DATE NOT NULL,
    time_entry_id UUID REFERENCES staff.time_entries(id),
    shift_assignment_id UUID REFERENCES staff.shift_assignments(id),

    expected_value VARCHAR(100),
    actual_value VARCHAR(100),
    description TEXT,

    status VARCHAR(20) NOT NULL DEFAULT 'open',
    acknowledged_by UUID REFERENCES staff.employees(id),
    acknowledged_at TIMESTAMPTZ,
    resolution_notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE staff.compliance_violations ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.compliance_violations
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_compliance_violations_tenant ON staff.compliance_violations(tenant_id);
CREATE INDEX idx_compliance_violations_employee ON staff.compliance_violations(employee_id);
CREATE INDEX idx_compliance_violations_status ON staff.compliance_violations(status);

CREATE TRIGGER compliance_violations_updated_at
    BEFORE UPDATE ON staff.compliance_violations
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- COMPLIANCE SETTINGS
-- ============================================================================
CREATE TABLE staff.compliance_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    min_break_6h_minutes INTEGER NOT NULL DEFAULT 30,
    min_break_9h_minutes INTEGER NOT NULL DEFAULT 45,
    min_break_segment_minutes INTEGER NOT NULL DEFAULT 15,
    max_daily_hours INTEGER NOT NULL DEFAULT 10,
    target_daily_hours INTEGER NOT NULL DEFAULT 8,
    max_weekly_hours INTEGER NOT NULL DEFAULT 48,
    min_rest_between_shifts_hours INTEGER NOT NULL DEFAULT 11,
    alert_no_break_after_minutes INTEGER NOT NULL DEFAULT 360,
    alert_break_too_long_minutes INTEGER NOT NULL DEFAULT 60,
    alert_approaching_max_hours_minutes INTEGER NOT NULL DEFAULT 30,
    notify_employee_violations BOOLEAN NOT NULL DEFAULT true,
    notify_manager_violations BOOLEAN NOT NULL DEFAULT true,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT compliance_settings_tenant_unique UNIQUE (tenant_id)
);

ALTER TABLE staff.compliance_settings ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.compliance_settings
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE TRIGGER compliance_settings_updated_at
    BEFORE UPDATE ON staff.compliance_settings
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- COMPLIANCE ALERTS
-- ============================================================================
CREATE TABLE staff.compliance_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id),

    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'warning',
    message TEXT NOT NULL,
    action_label VARCHAR(100),

    is_active BOOLEAN NOT NULL DEFAULT true,
    dismissed_by UUID REFERENCES staff.employees(id),
    dismissed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE staff.compliance_alerts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.compliance_alerts
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_compliance_alerts_tenant ON staff.compliance_alerts(tenant_id);
CREATE INDEX idx_compliance_alerts_active ON staff.compliance_alerts(is_active) WHERE is_active = true;

CREATE TRIGGER compliance_alerts_updated_at
    BEFORE UPDATE ON staff.compliance_alerts
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- TIME CORRECTION REQUESTS
-- ============================================================================
CREATE TABLE staff.time_correction_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    employee_id UUID NOT NULL REFERENCES staff.employees(id),
    time_entry_id UUID REFERENCES staff.time_entries(id),

    requested_date DATE NOT NULL,
    requested_clock_in TIMESTAMPTZ,
    requested_clock_out TIMESTAMPTZ,
    request_type VARCHAR(50) NOT NULL,
    reason TEXT NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    reviewed_by UUID REFERENCES staff.employees(id),
    reviewed_at TIMESTAMPTZ,
    rejection_reason TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

ALTER TABLE staff.time_correction_requests ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.time_correction_requests
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_correction_requests_tenant ON staff.time_correction_requests(tenant_id);
CREATE INDEX idx_correction_requests_employee ON staff.time_correction_requests(employee_id);
CREATE INDEX idx_correction_requests_status ON staff.time_correction_requests(status);

CREATE TRIGGER time_correction_requests_updated_at
    BEFORE UPDATE ON staff.time_correction_requests
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- USER CACHE (event-synced from user-service)
-- ============================================================================
CREATE TABLE staff.user_cache (
    user_id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    email VARCHAR(255),
    role_name VARCHAR(100),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE staff.user_cache ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.user_cache
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_staff_user_cache_tenant ON staff.user_cache(tenant_id);

-- ============================================================================
-- DOCUMENT PROCESSING AUDIT (DSGVO compliance)
-- ============================================================================
CREATE TABLE staff.document_processing_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    document_type VARCHAR(50) NOT NULL,
    consent_timestamp TIMESTAMPTZ NOT NULL,
    consent_given_by UUID NOT NULL,
    fields_extracted TEXT[] NOT NULL DEFAULT '{}',
    processing_duration_ms INTEGER,
    image_deleted_at TIMESTAMPTZ NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE staff.document_processing_audit ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff.document_processing_audit
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_document_processing_audit_tenant ON staff.document_processing_audit(tenant_id);
CREATE INDEX idx_document_processing_audit_created ON staff.document_processing_audit(created_at DESC);
