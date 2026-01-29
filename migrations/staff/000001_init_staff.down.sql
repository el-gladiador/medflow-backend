DROP TRIGGER IF EXISTS update_employee_documents_updated_at ON employee_documents;
DROP TRIGGER IF EXISTS update_employee_social_updated_at ON employee_social_insurance;
DROP TRIGGER IF EXISTS update_employee_financials_updated_at ON employee_financials;
DROP TRIGGER IF EXISTS update_employee_contacts_updated_at ON employee_contacts;
DROP TRIGGER IF EXISTS update_employee_addresses_updated_at ON employee_addresses;
DROP TRIGGER IF EXISTS update_employees_updated_at ON employees;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS user_cache;
DROP TABLE IF EXISTS employee_files;
DROP TABLE IF EXISTS employee_documents;
DROP TABLE IF EXISTS employee_social_insurance;
DROP TABLE IF EXISTS employee_financials;
DROP TABLE IF EXISTS employee_contacts;
DROP TABLE IF EXISTS employee_addresses;
DROP TABLE IF EXISTS employees;

DROP TYPE IF EXISTS ausweistyp;
DROP TYPE IF EXISTS konfession;
DROP TYPE IF EXISTS steuerklasse;
DROP TYPE IF EXISTS krankenversicherungstyp;
DROP TYPE IF EXISTS arbeitszeitmodell;
DROP TYPE IF EXISTS vertragsart;
DROP TYPE IF EXISTS anstellungsart;
DROP TYPE IF EXISTS familienstand;
DROP TYPE IF EXISTS geschlecht;
