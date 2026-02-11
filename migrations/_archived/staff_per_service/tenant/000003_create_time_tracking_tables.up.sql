-- MedFlow Schema-per-Tenant: Tenant Schema
-- Migration: Create time tracking tables for clock in/out, breaks, and corrections

-- ============================================================================
-- TIME ENTRIES (daily clock in/out records)
-- ============================================================================
CREATE TABLE time_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
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

COMMENT ON TABLE time_entries IS 'Daily time tracking entries for employee clock in/out';
COMMENT ON COLUMN time_entries.entry_date IS 'The date this time entry is for (local date)';
COMMENT ON COLUMN time_entries.clock_in IS 'Timestamp when employee clocked in';
COMMENT ON COLUMN time_entries.clock_out IS 'Timestamp when employee clocked out (NULL if still clocked in)';
COMMENT ON COLUMN time_entries.total_work_minutes IS 'Total work minutes (clock_out - clock_in - breaks), calculated on clock out';
COMMENT ON COLUMN time_entries.total_break_minutes IS 'Total break minutes taken, calculated from time_breaks table';
COMMENT ON COLUMN time_entries.is_manual_entry IS 'TRUE if this entry was manually created/corrected by a manager';

-- ============================================================================
-- TIME BREAKS (breaks within a time entry)
-- ============================================================================
CREATE TABLE time_breaks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    time_entry_id UUID NOT NULL REFERENCES time_entries(id) ON DELETE CASCADE,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE time_breaks IS 'Break periods within a time entry';
COMMENT ON COLUMN time_breaks.start_time IS 'Timestamp when break started';
COMMENT ON COLUMN time_breaks.end_time IS 'Timestamp when break ended (NULL if currently on break)';

-- ============================================================================
-- TIME CORRECTIONS (audit trail for manager corrections)
-- ============================================================================
CREATE TABLE time_corrections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    time_entry_id UUID REFERENCES time_entries(id) ON DELETE SET NULL,
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

COMMENT ON TABLE time_corrections IS 'Audit trail for time entry corrections made by managers';
COMMENT ON COLUMN time_corrections.original_clock_in IS 'Original clock in time before correction';
COMMENT ON COLUMN time_corrections.original_clock_out IS 'Original clock out time before correction';
COMMENT ON COLUMN time_corrections.corrected_clock_in IS 'New clock in time after correction';
COMMENT ON COLUMN time_corrections.corrected_clock_out IS 'New clock out time after correction';
COMMENT ON COLUMN time_corrections.reason IS 'Reason for the correction (required)';
COMMENT ON COLUMN time_corrections.corrected_by IS 'User ID of the manager who made the correction';

-- ============================================================================
-- INDEXES
-- ============================================================================

-- Time Entries indexes
CREATE INDEX idx_time_entries_employee_date ON time_entries(employee_id, entry_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_time_entries_date ON time_entries(entry_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_time_entries_employee ON time_entries(employee_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_time_entries_active ON time_entries(employee_id, clock_out) WHERE deleted_at IS NULL AND clock_out IS NULL;

-- Time Breaks indexes
CREATE INDEX idx_time_breaks_entry ON time_breaks(time_entry_id);
CREATE INDEX idx_time_breaks_active ON time_breaks(time_entry_id, end_time) WHERE end_time IS NULL;

-- Time Corrections indexes
CREATE INDEX idx_time_corrections_employee ON time_corrections(employee_id, correction_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_time_corrections_entry ON time_corrections(time_entry_id) WHERE deleted_at IS NULL;

-- ============================================================================
-- TRIGGERS
-- ============================================================================

CREATE TRIGGER time_entries_updated_at
    BEFORE UPDATE ON time_entries
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER time_breaks_updated_at
    BEFORE UPDATE ON time_breaks
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER time_corrections_updated_at
    BEFORE UPDATE ON time_corrections
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
