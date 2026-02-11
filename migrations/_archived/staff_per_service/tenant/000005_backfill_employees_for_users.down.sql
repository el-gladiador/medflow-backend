-- MedFlow Schema-per-Tenant: Tenant Schema
-- Migration Rollback: Remove auto-created employee records
-- Only removes employees that were auto-created (have user_id set and minimal additional data)

DELETE FROM employees
WHERE user_id IS NOT NULL
  AND department IS NULL
  AND employee_number IS NULL
  AND notes IS NULL
  AND date_of_birth IS NULL;
