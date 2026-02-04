-- MedFlow Schema-per-Tenant: Tenant Schema
-- Migration: Drop scheduling tables (rollback)

-- Drop triggers first
DROP TRIGGER IF EXISTS vacation_balances_updated_at ON vacation_balances;
DROP TRIGGER IF EXISTS absences_updated_at ON absences;
DROP TRIGGER IF EXISTS shift_assignments_updated_at ON shift_assignments;
DROP TRIGGER IF EXISTS shift_templates_updated_at ON shift_templates;

-- Drop tables in reverse order of creation (respecting foreign key constraints)
DROP TABLE IF EXISTS arbzg_compliance_log;
DROP TABLE IF EXISTS vacation_balances;
DROP TABLE IF EXISTS absences;
DROP TABLE IF EXISTS shift_assignments;
DROP TABLE IF EXISTS shift_templates;
