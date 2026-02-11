-- MedFlow Schema-per-Tenant: Tenant Schema
-- Migration: Create scheduling tables for shifts, absences, and time tracking

-- ============================================================================
-- SHIFT TEMPLATES (reusable shift definitions)
-- ============================================================================
CREATE TABLE shift_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
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

COMMENT ON TABLE shift_templates IS 'Reusable shift definitions that can be assigned to employees';
COMMENT ON COLUMN shift_templates.shift_type IS 'Type of shift: regular, on_call (Bereitschaft), emergency (Notdienst), night';
COMMENT ON COLUMN shift_templates.break_duration_minutes IS 'Required break duration in minutes (German ArbZG compliance)';

-- ============================================================================
-- SHIFT ASSIGNMENTS (scheduled shifts for employees)
-- ============================================================================
CREATE TABLE shift_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    shift_template_id UUID REFERENCES shift_templates(id),
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

COMMENT ON TABLE shift_assignments IS 'Scheduled shifts assigned to employees';
COMMENT ON COLUMN shift_assignments.has_conflict IS 'Indicates if this shift conflicts with another assignment or absence';
COMMENT ON COLUMN shift_assignments.status IS 'Workflow status: scheduled -> confirmed -> completed, or cancelled';

-- ============================================================================
-- ABSENCES (vacation, sick leave, etc.)
-- ============================================================================
CREATE TABLE absences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
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
            'vacation',       -- Urlaub
            'sick',           -- Krankheit
            'sick_child',     -- Kind krank
            'training',       -- Fortbildung
            'special_leave',  -- Sonderurlaub
            'unpaid_leave',   -- Unbezahlter Urlaub
            'parental_leave', -- Elternzeit
            'comp_time',      -- Zeitausgleich
            'other'
        )
    ),
    CONSTRAINT absences_status_valid CHECK (
        status IN ('pending', 'approved', 'rejected', 'cancelled')
    ),
    CONSTRAINT absences_dates_valid CHECK (end_date >= start_date)
);

COMMENT ON TABLE absences IS 'Employee absence records (vacation, sick leave, etc.)';
COMMENT ON COLUMN absences.absence_type IS 'German absence types: vacation (Urlaub), sick (Krankheit), sick_child (Kind krank), etc.';
COMMENT ON COLUMN absences.vacation_days_used IS 'Number of vacation days consumed (for vacation type absences)';

-- ============================================================================
-- VACATION BALANCES (annual entitlement tracking)
-- ============================================================================
CREATE TABLE vacation_balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    year INTEGER NOT NULL,
    annual_entitlement DECIMAL(5,2) NOT NULL DEFAULT 30,
    carryover_from_previous DECIMAL(5,2) NOT NULL DEFAULT 0,
    additional_granted DECIMAL(5,2) NOT NULL DEFAULT 0,
    taken DECIMAL(5,2) NOT NULL DEFAULT 0,
    planned DECIMAL(5,2) NOT NULL DEFAULT 0,
    pending DECIMAL(5,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT vacation_balances_unique UNIQUE (employee_id, year),
    CONSTRAINT vacation_balances_year_valid CHECK (year >= 2000 AND year <= 2100),
    CONSTRAINT vacation_balances_entitlement_valid CHECK (annual_entitlement >= 0),
    CONSTRAINT vacation_balances_taken_valid CHECK (taken >= 0),
    CONSTRAINT vacation_balances_planned_valid CHECK (planned >= 0),
    CONSTRAINT vacation_balances_pending_valid CHECK (pending >= 0)
);

COMMENT ON TABLE vacation_balances IS 'Annual vacation entitlement and usage tracking per employee';
COMMENT ON COLUMN vacation_balances.annual_entitlement IS 'Base vacation days for the year (German minimum: 20 for 5-day week)';
COMMENT ON COLUMN vacation_balances.carryover_from_previous IS 'Unused days carried over from previous year (expires March 31 per German law)';
COMMENT ON COLUMN vacation_balances.taken IS 'Days already taken (approved and past)';
COMMENT ON COLUMN vacation_balances.planned IS 'Days for approved future absences';
COMMENT ON COLUMN vacation_balances.pending IS 'Days in pending vacation requests';

-- ============================================================================
-- ARBZG COMPLIANCE LOG (German Working Time Act violations)
-- ============================================================================
CREATE TABLE arbzg_compliance_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
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

COMMENT ON TABLE arbzg_compliance_log IS 'German Working Time Act (Arbeitszeitgesetz) compliance violation tracking';
COMMENT ON COLUMN arbzg_compliance_log.violation_type IS 'Type of violation: max_daily_hours, min_rest_period, max_weekly_hours, etc.';
COMMENT ON COLUMN arbzg_compliance_log.severity IS 'warning: approaching limit, violation: limit exceeded, critical: severe breach';
COMMENT ON COLUMN arbzg_compliance_log.details IS 'JSON with violation details (e.g., {"hours_worked": 11, "limit": 10})';

-- ============================================================================
-- INDEXES
-- ============================================================================

-- Shift Templates indexes
CREATE INDEX idx_shift_templates_active ON shift_templates(is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_shift_templates_type ON shift_templates(shift_type) WHERE deleted_at IS NULL;

-- Shift Assignments indexes
CREATE INDEX idx_shift_assignments_employee ON shift_assignments(employee_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_shift_assignments_date ON shift_assignments(shift_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_shift_assignments_status ON shift_assignments(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_shift_assignments_template ON shift_assignments(shift_template_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_shift_assignments_employee_date ON shift_assignments(employee_id, shift_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_shift_assignments_date_range ON shift_assignments(shift_date, start_time, end_time) WHERE deleted_at IS NULL;

-- Absences indexes
CREATE INDEX idx_absences_employee ON absences(employee_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_absences_dates ON absences(start_date, end_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_absences_status ON absences(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_absences_type ON absences(absence_type) WHERE deleted_at IS NULL;
CREATE INDEX idx_absences_pending ON absences(status) WHERE status = 'pending' AND deleted_at IS NULL;
CREATE INDEX idx_absences_employee_dates ON absences(employee_id, start_date, end_date) WHERE deleted_at IS NULL;

-- Vacation Balances indexes
CREATE INDEX idx_vacation_balances_employee ON vacation_balances(employee_id);
CREATE INDEX idx_vacation_balances_year ON vacation_balances(employee_id, year);

-- ArbZG Compliance indexes
CREATE INDEX idx_arbzg_compliance_employee ON arbzg_compliance_log(employee_id);
CREATE INDEX idx_arbzg_compliance_date ON arbzg_compliance_log(violation_date);
CREATE INDEX idx_arbzg_compliance_unack ON arbzg_compliance_log(acknowledged_at) WHERE acknowledged_at IS NULL;
CREATE INDEX idx_arbzg_compliance_severity ON arbzg_compliance_log(severity);

-- ============================================================================
-- TRIGGERS
-- ============================================================================

CREATE TRIGGER shift_templates_updated_at
    BEFORE UPDATE ON shift_templates
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER shift_assignments_updated_at
    BEFORE UPDATE ON shift_assignments
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER absences_updated_at
    BEFORE UPDATE ON absences
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER vacation_balances_updated_at
    BEFORE UPDATE ON vacation_balances
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
