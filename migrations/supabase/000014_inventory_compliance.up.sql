-- MedFlow: Inventory Regulatory Compliance
-- Adds compliance fields for CE/UDI tracking, medication opening dates,
-- hazardous substance details (GefStoffV §6), and item document management.

-- ============================================================================
-- ALTER inventory_items: Add compliance fields
-- ============================================================================
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS manufacturer_address TEXT;
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS ce_marking_number VARCHAR(100);
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS notified_body_id VARCHAR(100);
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS acquisition_date DATE;
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS serial_number VARCHAR(100);
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS udi_di VARCHAR(255);
ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS udi_pi VARCHAR(255);

-- ============================================================================
-- ALTER inventory_batches: Add opened_at for medication tracking (AMG)
-- ============================================================================
ALTER TABLE inventory.inventory_batches ADD COLUMN IF NOT EXISTS opened_at TIMESTAMPTZ;

-- ============================================================================
-- HAZARDOUS SUBSTANCE DETAILS (GefStoffV §6)
-- One-to-one with inventory_items where is_hazardous = true
-- ============================================================================
CREATE TABLE inventory.hazardous_substance_details (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE CASCADE,

    ghs_pictogram_codes TEXT,        -- Comma-separated GHS01-GHS09
    h_statements TEXT,               -- Hazard statements (H200, H301, etc.)
    p_statements TEXT,               -- Precautionary statements (P101, P202, etc.)
    signal_word VARCHAR(50),         -- 'Danger' or 'Warning'
    usage_area TEXT,                 -- Where used in practice
    storage_instructions TEXT,
    emergency_procedures TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT uq_hazardous_tenant_item UNIQUE(tenant_id, item_id)
);

ALTER TABLE inventory.hazardous_substance_details ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.hazardous_substance_details FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.hazardous_substance_details
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_hazardous_details_tenant ON inventory.hazardous_substance_details(tenant_id);
CREATE INDEX idx_hazardous_details_item ON inventory.hazardous_substance_details(item_id);

CREATE TRIGGER hazardous_substance_details_updated_at
    BEFORE UPDATE ON inventory.hazardous_substance_details
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- ITEM DOCUMENTS (SDB uploads, certificates, manuals)
-- ============================================================================
CREATE TABLE inventory.item_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE CASCADE,

    document_type VARCHAR(50) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_path TEXT NOT NULL,
    file_size_bytes INTEGER,
    mime_type VARCHAR(100),

    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    uploaded_by UUID,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT item_documents_type_valid CHECK (
        document_type IN ('sdb', 'manual', 'certificate')
    )
);

ALTER TABLE inventory.item_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.item_documents FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.item_documents
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_item_documents_tenant ON inventory.item_documents(tenant_id);
CREATE INDEX idx_item_documents_item ON inventory.item_documents(item_id);

CREATE TRIGGER item_documents_updated_at
    BEFORE UPDATE ON inventory.item_documents
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
