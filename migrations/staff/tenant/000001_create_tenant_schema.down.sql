-- Rollback: Drop staff/employee tables

DROP TRIGGER IF EXISTS employees_updated_at ON employees;
DROP TRIGGER IF EXISTS employee_addresses_updated_at ON employee_addresses;
DROP TRIGGER IF EXISTS employee_contacts_updated_at ON employee_contacts;
DROP TRIGGER IF EXISTS employee_financials_updated_at ON employee_financials;
DROP TRIGGER IF EXISTS employee_social_insurance_updated_at ON employee_social_insurance;
DROP TRIGGER IF EXISTS employee_documents_updated_at ON employee_documents;

DROP TABLE IF EXISTS employee_documents;
DROP TABLE IF EXISTS employee_social_insurance;
DROP TABLE IF EXISTS employee_financials;
DROP TABLE IF EXISTS employee_contacts;
DROP TABLE IF EXISTS employee_addresses;
DROP TABLE IF EXISTS employees;
