-- MedFlow: BtM Controlled Substances (Betäubungsmittelgesetz)
-- Implements BtMG-compliant controlled substance tracking:
--   - Append-only BtM register (Betäubungsmittelbuch) with gap-free entry numbers
--   - Authorization management for BtM handling personnel
--   - Inventory item flags for controlled substance identification

-- ============================================================================
-- 1. ALTER inventory_items: Add controlled substance columns
-- ============================================================================
ALTER TABLE inventory.inventory_items
    ADD COLUMN IF NOT EXISTS is_controlled_substance BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE inventory.inventory_items
    ADD COLUMN IF NOT EXISTS btm_schedule VARCHAR(20);
ALTER TABLE inventory.inventory_items
    ADD COLUMN IF NOT EXISTS btm_max_stock_days INTEGER DEFAULT 14;

CREATE INDEX IF NOT EXISTS idx_inventory_items_controlled
    ON inventory.inventory_items(is_controlled_substance)
    WHERE is_controlled_substance = TRUE AND deleted_at IS NULL;

-- ============================================================================
-- 2. inventory.btm_register (append-only ledger)
-- Gap-free entry numbering per tenant+item, immutable records
-- ============================================================================
CREATE TABLE inventory.btm_register (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id),

    -- Gap-free sequential number per tenant+item
    entry_number INTEGER NOT NULL,
    entry_type VARCHAR(30) NOT NULL,

    -- Quantities
    quantity DECIMAL(12,4) NOT NULL,
    running_balance DECIMAL(12,4) NOT NULL,
    unit VARCHAR(50) NOT NULL,

    -- Receipt fields
    supplier_name VARCHAR(255),
    delivery_note_number VARCHAR(100),

    -- Dispense fields
    patient_identifier VARCHAR(255),
    prescribing_doctor VARCHAR(255),
    purpose TEXT,

    -- Disposal fields
    disposal_method VARCHAR(100),
    disposal_witness VARCHAR(255),

    -- Correction fields
    correction_reason TEXT,
    corrects_entry_id UUID REFERENCES inventory.btm_register(id),

    -- Accountability
    performed_by UUID NOT NULL,
    performed_by_name VARCHAR(255) NOT NULL,
    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- NO updated_at (immutable)
    -- NO deleted_at (immutable)

    CONSTRAINT btm_register_entry_type_valid CHECK (
        entry_type IN ('receipt', 'dispense', 'disposal', 'correction', 'inventory_check')
    ),
    CONSTRAINT btm_register_entry_number_unique UNIQUE (tenant_id, item_id, entry_number)
);

ALTER TABLE inventory.btm_register ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.btm_register FORCE ROW LEVEL SECURITY;

-- Separate SELECT and INSERT policies (append-only)
CREATE POLICY btm_register_select ON inventory.btm_register
    FOR SELECT USING (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE POLICY btm_register_insert ON inventory.btm_register
    FOR INSERT WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

-- Indexes
CREATE INDEX idx_btm_register_tenant ON inventory.btm_register(tenant_id);
CREATE INDEX idx_btm_register_item ON inventory.btm_register(item_id);
CREATE INDEX idx_btm_register_created ON inventory.btm_register(created_at);
CREATE INDEX idx_btm_register_entry_type ON inventory.btm_register(entry_type);

-- Append-only: SELECT + INSERT only, NO UPDATE, NO DELETE
GRANT SELECT, INSERT ON inventory.btm_register TO medflow_app;

-- ============================================================================
-- 3. inventory.btm_authorized_personnel (standard CRUD table)
-- Tracks who is authorized to handle controlled substances
-- ============================================================================
CREATE TABLE inventory.btm_authorized_personnel (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    user_id UUID NOT NULL,
    user_name VARCHAR(255) NOT NULL,

    authorization_type VARCHAR(30) NOT NULL,

    authorized_by UUID,
    authorized_by_name VARCHAR(255),
    authorized_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    revoked_at TIMESTAMPTZ,
    revoked_by UUID,
    revoked_by_name VARCHAR(255),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,

    CONSTRAINT btm_auth_type_valid CHECK (
        authorization_type IN ('full', 'dispense_only', 'view_only')
    )
);

-- Only one active (non-revoked) authorization per user per tenant
CREATE UNIQUE INDEX idx_btm_authorized_active
    ON inventory.btm_authorized_personnel(tenant_id, user_id)
    WHERE revoked_at IS NULL;

ALTER TABLE inventory.btm_authorized_personnel ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.btm_authorized_personnel FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON inventory.btm_authorized_personnel
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_btm_authorized_tenant ON inventory.btm_authorized_personnel(tenant_id);
CREATE INDEX idx_btm_authorized_user ON inventory.btm_authorized_personnel(user_id);

CREATE TRIGGER btm_authorized_personnel_updated_at
    BEFORE UPDATE ON inventory.btm_authorized_personnel
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.btm_authorized_personnel TO medflow_app;
