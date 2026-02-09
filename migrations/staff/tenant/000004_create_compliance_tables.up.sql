-- ============================================================================
-- ArbZG (Arbeitszeitgesetz) Compliance Tables
-- German Labor Law compliance tracking for time tracking and shift planning
-- ============================================================================

-- Compliance Violations Log
-- Records all violations of German labor law for audit purposes
CREATE TABLE IF NOT EXISTS compliance_violations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id),

    -- Violation details
    violation_type VARCHAR(50) NOT NULL,
    -- Types:
    -- 'missing_break' - No break taken when required
    -- 'insufficient_break' - Break duration less than legally required
    -- 'break_too_short_6h' - Break < 30min for 6-9h work
    -- 'break_too_short_9h' - Break < 45min for >9h work
    -- 'max_daily_hours_exceeded' - Worked more than 10 hours in a day
    -- 'max_weekly_hours_exceeded' - Worked more than 48 hours in a week
    -- 'rest_period_violated' - Less than 11h rest between shifts
    -- 'no_break_after_6h' - Worked 6+ hours without taking a break

    violation_date DATE NOT NULL,
    time_entry_id UUID REFERENCES time_entries(id),
    shift_assignment_id UUID REFERENCES shift_assignments(id),

    -- Details
    expected_value VARCHAR(100),  -- e.g., "30 minutes break", "11 hours rest"
    actual_value VARCHAR(100),    -- e.g., "15 minutes break", "8 hours rest"
    description TEXT,

    -- Resolution workflow
    status VARCHAR(20) NOT NULL DEFAULT 'open',  -- 'open', 'acknowledged', 'resolved', 'waived'
    acknowledged_by UUID REFERENCES employees(id),
    acknowledged_at TIMESTAMPTZ,
    resolution_notes TEXT,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_compliance_violations_employee ON compliance_violations(employee_id);
CREATE INDEX idx_compliance_violations_date ON compliance_violations(violation_date);
CREATE INDEX idx_compliance_violations_status ON compliance_violations(status);
CREATE INDEX idx_compliance_violations_type ON compliance_violations(violation_type);

-- Compliance Settings
-- Tenant-configurable compliance settings (cannot be less than legal minimum)
CREATE TABLE IF NOT EXISTS compliance_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Break settings (cannot be less than legal minimum)
    -- Legal minimum: 30min for 6-9h, 45min for >9h
    min_break_6h_minutes INTEGER NOT NULL DEFAULT 30,  -- Minutes, legal min: 30
    min_break_9h_minutes INTEGER NOT NULL DEFAULT 45,  -- Minutes, legal min: 45
    min_break_segment_minutes INTEGER NOT NULL DEFAULT 15,  -- Minutes, legal min: 15 (breaks can be split)

    -- Work hour settings
    -- Legal maximum: 8h/day (can extend to 10h with averaging), 48h/week
    max_daily_hours INTEGER NOT NULL DEFAULT 10,      -- Legal max: 10
    target_daily_hours INTEGER NOT NULL DEFAULT 8,    -- For averaging calculations
    max_weekly_hours INTEGER NOT NULL DEFAULT 48,     -- Legal max: 48

    -- Rest period settings
    -- Legal minimum: 11h between shifts (10h for healthcare with compensation)
    min_rest_between_shifts_hours INTEGER NOT NULL DEFAULT 11,  -- Hours, legal min: 11 (10 for healthcare)

    -- Alert thresholds
    alert_no_break_after_minutes INTEGER NOT NULL DEFAULT 360,       -- 6 hours = 360 minutes
    alert_break_too_long_minutes INTEGER NOT NULL DEFAULT 60,        -- Alert manager if break > 60min
    alert_approaching_max_hours_minutes INTEGER NOT NULL DEFAULT 30, -- Alert 30min before max hours

    -- Notification preferences
    notify_employee_violations BOOLEAN NOT NULL DEFAULT true,
    notify_manager_violations BOOLEAN NOT NULL DEFAULT true,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert default compliance settings (one row per tenant)
INSERT INTO compliance_settings (id)
VALUES (gen_random_uuid())
ON CONFLICT DO NOTHING;

-- Compliance Alerts
-- Real-time alerts for managers about ongoing compliance issues
CREATE TABLE IF NOT EXISTS compliance_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id),

    -- Alert details
    alert_type VARCHAR(50) NOT NULL,
    -- Types:
    -- 'no_break_warning' - Working 6+ hours without break
    -- 'break_too_long' - On break for extended time
    -- 'max_hours_approaching' - Close to daily max hours
    -- 'max_hours_exceeded' - Exceeded daily max hours

    severity VARCHAR(20) NOT NULL DEFAULT 'warning',  -- 'info', 'warning', 'critical'
    message TEXT NOT NULL,
    action_label VARCHAR(100),  -- e.g., "Take Break", "Clock Out"

    -- Status
    is_active BOOLEAN NOT NULL DEFAULT true,
    dismissed_by UUID REFERENCES employees(id),
    dismissed_at TIMESTAMPTZ,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_compliance_alerts_employee ON compliance_alerts(employee_id);
CREATE INDEX idx_compliance_alerts_active ON compliance_alerts(is_active) WHERE is_active = true;
CREATE INDEX idx_compliance_alerts_type ON compliance_alerts(alert_type);

-- Time Correction Requests
-- Employee-initiated requests for time corrections (manager approval workflow)
CREATE TABLE IF NOT EXISTS time_correction_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id),
    time_entry_id UUID REFERENCES time_entries(id),  -- NULL for entries not yet created

    -- Requested changes
    requested_date DATE NOT NULL,
    requested_clock_in TIMESTAMPTZ,
    requested_clock_out TIMESTAMPTZ,

    -- Request metadata
    request_type VARCHAR(50) NOT NULL,
    -- Types:
    -- 'clock_in_correction' - Correct the clock in time
    -- 'clock_out_correction' - Correct the clock out time
    -- 'missed_entry' - Add a missed time entry
    -- 'delete_entry' - Request deletion of incorrect entry

    reason TEXT NOT NULL,  -- Employee's explanation (required for audit)

    -- Approval workflow
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- 'pending', 'approved', 'rejected'
    reviewed_by UUID REFERENCES employees(id),      -- Manager who reviewed
    reviewed_at TIMESTAMPTZ,
    rejection_reason TEXT,                          -- If rejected, why

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_correction_requests_employee ON time_correction_requests(employee_id);
CREATE INDEX idx_correction_requests_status ON time_correction_requests(status);
CREATE INDEX idx_correction_requests_date ON time_correction_requests(requested_date);

-- Add trigger for updated_at timestamps
CREATE OR REPLACE FUNCTION update_compliance_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER compliance_violations_updated_at
    BEFORE UPDATE ON compliance_violations
    FOR EACH ROW EXECUTE FUNCTION update_compliance_updated_at();

CREATE TRIGGER compliance_settings_updated_at
    BEFORE UPDATE ON compliance_settings
    FOR EACH ROW EXECUTE FUNCTION update_compliance_updated_at();

CREATE TRIGGER compliance_alerts_updated_at
    BEFORE UPDATE ON compliance_alerts
    FOR EACH ROW EXECUTE FUNCTION update_compliance_updated_at();

CREATE TRIGGER time_correction_requests_updated_at
    BEFORE UPDATE ON time_correction_requests
    FOR EACH ROW EXECUTE FUNCTION update_compliance_updated_at();
