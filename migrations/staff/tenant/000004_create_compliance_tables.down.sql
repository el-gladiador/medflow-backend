-- Drop triggers
DROP TRIGGER IF EXISTS time_correction_requests_updated_at ON time_correction_requests;
DROP TRIGGER IF EXISTS compliance_alerts_updated_at ON compliance_alerts;
DROP TRIGGER IF EXISTS compliance_settings_updated_at ON compliance_settings;
DROP TRIGGER IF EXISTS compliance_violations_updated_at ON compliance_violations;

-- Drop function
DROP FUNCTION IF EXISTS update_compliance_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_correction_requests_date;
DROP INDEX IF EXISTS idx_correction_requests_status;
DROP INDEX IF EXISTS idx_correction_requests_employee;

DROP INDEX IF EXISTS idx_compliance_alerts_type;
DROP INDEX IF EXISTS idx_compliance_alerts_active;
DROP INDEX IF EXISTS idx_compliance_alerts_employee;

DROP INDEX IF EXISTS idx_compliance_violations_type;
DROP INDEX IF EXISTS idx_compliance_violations_status;
DROP INDEX IF EXISTS idx_compliance_violations_date;
DROP INDEX IF EXISTS idx_compliance_violations_employee;

-- Drop tables
DROP TABLE IF EXISTS time_correction_requests;
DROP TABLE IF EXISTS compliance_alerts;
DROP TABLE IF EXISTS compliance_settings;
DROP TABLE IF EXISTS compliance_violations;
