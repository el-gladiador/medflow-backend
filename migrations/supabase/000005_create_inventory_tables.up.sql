-- MedFlow RLS Multi-Tenancy: Inventory Schema Tables
-- All tables have tenant_id + RLS policies for row-level tenant isolation

-- ============================================================================
-- STORAGE ROOMS
-- ============================================================================
CREATE TABLE inventory.storage_rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    name VARCHAR(255) NOT NULL,
    description TEXT,
    floor VARCHAR(50),
    building VARCHAR(100),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID
);

ALTER TABLE inventory.storage_rooms ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON inventory.storage_rooms
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_storage_rooms_tenant ON inventory.storage_rooms(tenant_id);
CREATE INDEX idx_storage_rooms_active ON inventory.storage_rooms(is_active) WHERE deleted_at IS NULL;

CREATE TRIGGER storage_rooms_updated_at
    BEFORE UPDATE ON inventory.storage_rooms
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- STORAGE CABINETS
-- ============================================================================
CREATE TABLE inventory.storage_cabinets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    room_id UUID NOT NULL REFERENCES inventory.storage_rooms(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    description TEXT,
    temperature_controlled BOOLEAN NOT NULL DEFAULT FALSE,
    target_temperature_celsius DECIMAL(5,2),
    requires_key BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID
);

ALTER TABLE inventory.storage_cabinets ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON inventory.storage_cabinets
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_storage_cabinets_tenant ON inventory.storage_cabinets(tenant_id);
CREATE INDEX idx_storage_cabinets_room ON inventory.storage_cabinets(room_id) WHERE deleted_at IS NULL;

CREATE TRIGGER storage_cabinets_updated_at
    BEFORE UPDATE ON inventory.storage_cabinets
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- STORAGE SHELVES
-- ============================================================================
CREATE TABLE inventory.storage_shelves (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    cabinet_id UUID NOT NULL REFERENCES inventory.storage_cabinets(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    position INTEGER,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

ALTER TABLE inventory.storage_shelves ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON inventory.storage_shelves
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_storage_shelves_tenant ON inventory.storage_shelves(tenant_id);
CREATE INDEX idx_storage_shelves_cabinet ON inventory.storage_shelves(cabinet_id) WHERE deleted_at IS NULL;

CREATE TRIGGER storage_shelves_updated_at
    BEFORE UPDATE ON inventory.storage_shelves
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- INVENTORY ITEMS
-- ============================================================================
CREATE TABLE inventory.inventory_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    barcode VARCHAR(100),
    article_number VARCHAR(100),
    manufacturer VARCHAR(255),
    supplier VARCHAR(255),

    unit VARCHAR(50) NOT NULL,
    min_stock INTEGER NOT NULL DEFAULT 0,
    max_stock INTEGER,
    reorder_point INTEGER,
    reorder_quantity INTEGER,

    use_batch_tracking BOOLEAN NOT NULL DEFAULT FALSE,
    requires_cooling BOOLEAN NOT NULL DEFAULT FALSE,
    is_hazardous BOOLEAN NOT NULL DEFAULT FALSE,
    shelf_life_days INTEGER,

    default_location_id UUID,

    unit_price_cents INTEGER,
    currency VARCHAR(3) DEFAULT 'EUR',

    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID
);

ALTER TABLE inventory.inventory_items ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON inventory.inventory_items
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_inventory_items_tenant ON inventory.inventory_items(tenant_id);
CREATE INDEX idx_inventory_items_name ON inventory.inventory_items(name) WHERE deleted_at IS NULL;
CREATE INDEX idx_inventory_items_category ON inventory.inventory_items(tenant_id, category) WHERE deleted_at IS NULL;
CREATE INDEX idx_inventory_items_barcode ON inventory.inventory_items(barcode) WHERE deleted_at IS NULL AND barcode IS NOT NULL;
CREATE INDEX idx_inventory_items_article ON inventory.inventory_items(article_number) WHERE deleted_at IS NULL AND article_number IS NOT NULL;
CREATE INDEX idx_inventory_items_active ON inventory.inventory_items(is_active) WHERE deleted_at IS NULL;

CREATE TRIGGER inventory_items_updated_at
    BEFORE UPDATE ON inventory.inventory_items
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- INVENTORY BATCHES
-- ============================================================================
CREATE TABLE inventory.inventory_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE RESTRICT,
    location_id UUID,

    batch_number VARCHAR(100) NOT NULL,
    lot_number VARCHAR(100),

    initial_quantity INTEGER NOT NULL,
    current_quantity INTEGER NOT NULL,
    reserved_quantity INTEGER NOT NULL DEFAULT 0,

    manufactured_date DATE,
    expiry_date DATE,
    received_date DATE NOT NULL DEFAULT CURRENT_DATE,

    status VARCHAR(50) NOT NULL DEFAULT 'available',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT batches_quantity_valid CHECK (current_quantity >= 0),
    CONSTRAINT batches_reserved_valid CHECK (reserved_quantity >= 0 AND reserved_quantity <= current_quantity),
    CONSTRAINT batches_status_valid CHECK (status IN ('available', 'reserved', 'quarantine', 'expired', 'depleted'))
);

ALTER TABLE inventory.inventory_batches ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON inventory.inventory_batches
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_inventory_batches_tenant ON inventory.inventory_batches(tenant_id);
CREATE INDEX idx_inventory_batches_item ON inventory.inventory_batches(item_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_inventory_batches_expiry ON inventory.inventory_batches(expiry_date) WHERE deleted_at IS NULL AND status = 'available';
CREATE INDEX idx_inventory_batches_status ON inventory.inventory_batches(status) WHERE deleted_at IS NULL;

CREATE TRIGGER inventory_batches_updated_at
    BEFORE UPDATE ON inventory.inventory_batches
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- ============================================================================
-- STOCK ADJUSTMENTS
-- ============================================================================
CREATE TABLE inventory.stock_adjustments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE RESTRICT,
    batch_id UUID REFERENCES inventory.inventory_batches(id),

    adjustment_type VARCHAR(50) NOT NULL,
    quantity INTEGER NOT NULL,

    from_location_id UUID,
    to_location_id UUID,

    reason TEXT,
    reference_type VARCHAR(50),
    reference_id UUID,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID,

    CONSTRAINT adjustments_type_valid CHECK (
        adjustment_type IN ('receipt', 'issue', 'transfer', 'adjustment', 'disposal', 'return', 'count')
    )
);

ALTER TABLE inventory.stock_adjustments ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON inventory.stock_adjustments
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_stock_adjustments_tenant ON inventory.stock_adjustments(tenant_id);
CREATE INDEX idx_stock_adjustments_item ON inventory.stock_adjustments(item_id);
CREATE INDEX idx_stock_adjustments_created ON inventory.stock_adjustments(created_at DESC);

-- ============================================================================
-- INVENTORY ALERTS
-- ============================================================================
CREATE TABLE inventory.inventory_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    item_id UUID NOT NULL REFERENCES inventory.inventory_items(id) ON DELETE CASCADE,
    batch_id UUID REFERENCES inventory.inventory_batches(id),

    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'warning',
    message TEXT NOT NULL,

    status VARCHAR(50) NOT NULL DEFAULT 'active',
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by UUID,
    resolved_at TIMESTAMPTZ,
    resolved_by UUID,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT alerts_type_valid CHECK (
        alert_type IN ('low_stock', 'expiring_soon', 'expired', 'temperature', 'reorder')
    ),
    CONSTRAINT alerts_severity_valid CHECK (severity IN ('info', 'warning', 'critical')),
    CONSTRAINT alerts_status_valid CHECK (status IN ('active', 'acknowledged', 'resolved', 'dismissed'))
);

ALTER TABLE inventory.inventory_alerts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON inventory.inventory_alerts
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_inventory_alerts_tenant ON inventory.inventory_alerts(tenant_id);
CREATE INDEX idx_inventory_alerts_item ON inventory.inventory_alerts(item_id);
CREATE INDEX idx_inventory_alerts_status ON inventory.inventory_alerts(status) WHERE status = 'active';

-- ============================================================================
-- USER CACHE (event-synced from user-service)
-- ============================================================================
CREATE TABLE inventory.user_cache (
    user_id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    email VARCHAR(255),
    role_name VARCHAR(100),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE inventory.user_cache ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON inventory.user_cache
    FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE INDEX idx_inventory_user_cache_tenant ON inventory.user_cache(tenant_id);
