-- MedFlow RLS Multi-Tenancy: GDPR Tenant Data Deletion
-- SECURITY DEFINER function that bypasses RLS to delete all data for a tenant.
-- Replaces DROP SCHEMA CASCADE from schema-per-tenant architecture.

CREATE OR REPLACE FUNCTION public.delete_tenant_data(p_tenant_id UUID)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_tenant_exists BOOLEAN;
BEGIN
    -- Verify tenant exists
    SELECT EXISTS(SELECT 1 FROM public.tenants WHERE id = p_tenant_id) INTO v_tenant_exists;
    IF NOT v_tenant_exists THEN
        RAISE EXCEPTION 'Tenant % does not exist', p_tenant_id;
    END IF;

    -- Log the deletion event BEFORE deleting data
    INSERT INTO public.tenant_audit_log (tenant_id, event_type, event_data, performed_at)
    VALUES (p_tenant_id, 'deleted', jsonb_build_object('reason', 'GDPR Right to Erasure'), NOW());

    -- ========================================================================
    -- DELETE INVENTORY SCHEMA DATA (order respects FK constraints)
    -- ========================================================================
    DELETE FROM inventory.inventory_alerts WHERE tenant_id = p_tenant_id;
    DELETE FROM inventory.stock_adjustments WHERE tenant_id = p_tenant_id;
    DELETE FROM inventory.inventory_batches WHERE tenant_id = p_tenant_id;
    DELETE FROM inventory.inventory_items WHERE tenant_id = p_tenant_id;
    DELETE FROM inventory.storage_shelves WHERE tenant_id = p_tenant_id;
    DELETE FROM inventory.storage_cabinets WHERE tenant_id = p_tenant_id;
    DELETE FROM inventory.storage_rooms WHERE tenant_id = p_tenant_id;
    DELETE FROM inventory.user_cache WHERE tenant_id = p_tenant_id;

    -- ========================================================================
    -- DELETE STAFF SCHEMA DATA
    -- ========================================================================
    DELETE FROM staff.document_processing_audit WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.time_correction_requests WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.compliance_alerts WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.compliance_violations WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.compliance_settings WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.arbzg_compliance_log WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.time_corrections WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.time_breaks WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.time_entries WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.vacation_balances WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.absences WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.shift_assignments WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.shift_templates WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.employee_documents WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.employee_social_insurance WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.employee_financials WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.employee_contacts WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.employee_addresses WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.employees WHERE tenant_id = p_tenant_id;
    DELETE FROM staff.user_cache WHERE tenant_id = p_tenant_id;

    -- ========================================================================
    -- DELETE USERS SCHEMA DATA
    -- ========================================================================
    DELETE FROM users.audit_logs WHERE tenant_id = p_tenant_id;
    DELETE FROM users.user_roles WHERE tenant_id = p_tenant_id;
    DELETE FROM users.sessions WHERE tenant_id = p_tenant_id;
    DELETE FROM users.roles WHERE tenant_id = p_tenant_id;
    DELETE FROM users.users WHERE tenant_id = p_tenant_id;

    -- ========================================================================
    -- DELETE PUBLIC SCHEMA DATA
    -- ========================================================================
    DELETE FROM public.user_tenant_lookup WHERE tenant_id = p_tenant_id;

    -- Soft-delete the tenant record (keep for audit trail)
    UPDATE public.tenants SET deleted_at = NOW(), subscription_status = 'cancelled' WHERE id = p_tenant_id;
END;
$$;

COMMENT ON FUNCTION public.delete_tenant_data IS
    'GDPR Right to Erasure: Deletes ALL data for a tenant across all schemas. SECURITY DEFINER bypasses RLS.';
