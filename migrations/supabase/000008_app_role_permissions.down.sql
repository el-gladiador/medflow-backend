-- Rollback: Remove FORCE RLS and revoke medflow_app permissions

-- Remove FORCE RLS from users schema
ALTER TABLE users.users NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users.roles NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users.user_roles NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users.audit_logs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users.sessions NO FORCE ROW LEVEL SECURITY;

-- Remove FORCE RLS from staff schema
ALTER TABLE staff.employees NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.employee_contacts NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.employee_addresses NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.employee_financials NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.employee_social_insurance NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.employee_documents NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.document_processing_audit NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.time_entries NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.time_breaks NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.time_corrections NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.time_correction_requests NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.shift_templates NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.shift_assignments NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.absences NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.vacation_balances NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.compliance_settings NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.compliance_violations NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.compliance_alerts NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.arbzg_compliance_log NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff.user_cache NO FORCE ROW LEVEL SECURITY;

-- Revoke permissions
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM medflow_app;
REVOKE ALL ON ALL TABLES IN SCHEMA users FROM medflow_app;
REVOKE ALL ON ALL TABLES IN SCHEMA staff FROM medflow_app;
REVOKE ALL ON ALL TABLES IN SCHEMA inventory FROM medflow_app;

REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM medflow_app;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA users FROM medflow_app;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA staff FROM medflow_app;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA inventory FROM medflow_app;

REVOKE USAGE ON SCHEMA public FROM medflow_app;
REVOKE USAGE ON SCHEMA users FROM medflow_app;
REVOKE USAGE ON SCHEMA staff FROM medflow_app;
REVOKE USAGE ON SCHEMA inventory FROM medflow_app;

-- Note: We don't DROP the role here as it may be referenced elsewhere
-- DROP ROLE IF EXISTS medflow_app;
