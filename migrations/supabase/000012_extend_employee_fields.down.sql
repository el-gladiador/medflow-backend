-- Revert: Extend employee fields

-- Drop employee_files table
DROP TABLE IF EXISTS staff.employee_files;

-- Remove new columns
ALTER TABLE staff.employees DROP COLUMN IF EXISTS birth_place;
ALTER TABLE staff.employees DROP COLUMN IF EXISTS marital_status;
ALTER TABLE staff.employees DROP COLUMN IF EXISTS contract_type;
ALTER TABLE staff.employees DROP COLUMN IF EXISTS weekly_hours;
ALTER TABLE staff.employees DROP COLUMN IF EXISTS vacation_days;
ALTER TABLE staff.employees DROP COLUMN IF EXISTS work_time_model;

-- Drop new constraints (they go with the columns, but be explicit)
ALTER TABLE staff.employees DROP CONSTRAINT IF EXISTS employees_marital_status_valid;
ALTER TABLE staff.employees DROP CONSTRAINT IF EXISTS employees_contract_type_valid;
ALTER TABLE staff.employees DROP CONSTRAINT IF EXISTS employees_work_time_model_valid;

-- Restore original employment_type constraint
ALTER TABLE staff.employees DROP CONSTRAINT IF EXISTS employees_employment_type_valid;
ALTER TABLE staff.employees ADD CONSTRAINT employees_employment_type_valid CHECK (
    employment_type IN ('full_time', 'part_time', 'contractor', 'intern', 'temporary')
);
