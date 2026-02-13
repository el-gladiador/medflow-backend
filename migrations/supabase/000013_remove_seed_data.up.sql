-- MedFlow: Remove seed/test data from migrations
-- Seed data was previously inserted by 000007 and 000011.
-- Test tenants and demo data should only exist in development via `make seed`.
-- This migration cleans up any seed data that was applied to production.
-- Runs as superuser (bypasses RLS).
--
-- Uses the GDPR delete_tenant_data() function which handles the full FK
-- dependency chain across all schemas, then hard-deletes the tenant records
-- along with their audit trail.

-- Delete all data for test-practice tenant
SELECT public.delete_tenant_data('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11');

-- Delete all data for demo-clinic tenant
SELECT public.delete_tenant_data('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22');

-- Clean up audit trail and hard-delete tenant records
-- (GDPR function only soft-deletes tenants and logs to audit)
DELETE FROM public.tenant_audit_log WHERE tenant_id IN (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22'
);
DELETE FROM public.tenants WHERE id IN (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22'
);
