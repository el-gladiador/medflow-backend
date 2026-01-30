-- MedFlow Schema-per-Tenant: Tenant Schema
-- Migration: Create inventory management tables

-- Storage Rooms
CREATE TABLE storage_rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    name VARCHAR(255) NOT NULL,
    description TEXT,
    floor VARCHAR(50),
    building VARCHAR(100),

    -- Status
    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id)
);

-- Storage Cabinets
CREATE TABLE storage_cabinets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES storage_rooms(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    description TEXT,

    -- Properties
    temperature_controlled BOOLEAN NOT NULL DEFAULT FALSE,
    target_temperature_celsius DECIMAL(5,2),
    requires_key BOOLEAN NOT NULL DEFAULT FALSE,

    -- Status
    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id)
);

-- Storage Shelves
CREATE TABLE storage_shelves (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cabinet_id UUID NOT NULL REFERENCES storage_cabinets(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    position INTEGER,  -- Shelf number within cabinet

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Inventory Items (master data)
CREATE TABLE inventory_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Identity
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),

    -- Product info
    barcode VARCHAR(100),
    article_number VARCHAR(100),
    manufacturer VARCHAR(255),
    supplier VARCHAR(255),

    -- Stock settings
    unit VARCHAR(50) NOT NULL,  -- Stück, Packung, ml, etc.
    min_stock INTEGER NOT NULL DEFAULT 0,
    max_stock INTEGER,
    reorder_point INTEGER,
    reorder_quantity INTEGER,

    -- Properties
    use_batch_tracking BOOLEAN NOT NULL DEFAULT FALSE,
    requires_cooling BOOLEAN NOT NULL DEFAULT FALSE,
    is_hazardous BOOLEAN NOT NULL DEFAULT FALSE,
    shelf_life_days INTEGER,

    -- Default location
    default_location_id UUID REFERENCES storage_shelves(id),

    -- Pricing
    unit_price_cents INTEGER,
    currency VARCHAR(3) DEFAULT 'EUR',

    -- Status
    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id)
);

-- Inventory Batches (for batch tracking)
CREATE TABLE inventory_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE RESTRICT,
    location_id UUID REFERENCES storage_shelves(id),

    -- Batch info
    batch_number VARCHAR(100) NOT NULL,
    lot_number VARCHAR(100),

    -- Quantities
    initial_quantity INTEGER NOT NULL,
    current_quantity INTEGER NOT NULL,
    reserved_quantity INTEGER NOT NULL DEFAULT 0,

    -- Dates
    manufactured_date DATE,
    expiry_date DATE,
    received_date DATE NOT NULL DEFAULT CURRENT_DATE,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'available',

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- Constraints
    CONSTRAINT batches_quantity_valid CHECK (current_quantity >= 0),
    CONSTRAINT batches_reserved_valid CHECK (reserved_quantity >= 0 AND reserved_quantity <= current_quantity),
    CONSTRAINT batches_status_valid CHECK (status IN ('available', 'reserved', 'quarantine', 'expired', 'depleted'))
);

-- Stock Adjustments (all inventory movements)
CREATE TABLE stock_adjustments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE RESTRICT,
    batch_id UUID REFERENCES inventory_batches(id),

    -- Adjustment details
    adjustment_type VARCHAR(50) NOT NULL,
    quantity INTEGER NOT NULL,  -- Positive for additions, negative for reductions

    -- Location
    from_location_id UUID REFERENCES storage_shelves(id),
    to_location_id UUID REFERENCES storage_shelves(id),

    -- Context
    reason TEXT,
    reference_type VARCHAR(50),  -- order, return, disposal, transfer, etc.
    reference_id UUID,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID REFERENCES users(id),

    -- Constraints
    CONSTRAINT adjustments_type_valid CHECK (
        adjustment_type IN ('receipt', 'issue', 'transfer', 'adjustment', 'disposal', 'return', 'count')
    )
);

-- Inventory Alerts
CREATE TABLE inventory_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
    batch_id UUID REFERENCES inventory_batches(id),

    -- Alert details
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'warning',
    message TEXT NOT NULL,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by UUID REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    resolved_by UUID REFERENCES users(id),

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT alerts_type_valid CHECK (
        alert_type IN ('low_stock', 'expiring_soon', 'expired', 'temperature', 'reorder')
    ),
    CONSTRAINT alerts_severity_valid CHECK (severity IN ('info', 'warning', 'critical')),
    CONSTRAINT alerts_status_valid CHECK (status IN ('active', 'acknowledged', 'resolved', 'dismissed'))
);

-- Indexes
CREATE INDEX idx_storage_rooms_active ON storage_rooms(is_active) WHERE deleted_at IS NULL;

CREATE INDEX idx_storage_cabinets_room ON storage_cabinets(room_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_storage_cabinets_temp ON storage_cabinets(temperature_controlled) WHERE deleted_at IS NULL;

CREATE INDEX idx_storage_shelves_cabinet ON storage_shelves(cabinet_id) WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_items_name ON inventory_items(name) WHERE deleted_at IS NULL;
CREATE INDEX idx_inventory_items_category ON inventory_items(category) WHERE deleted_at IS NULL;
CREATE INDEX idx_inventory_items_barcode ON inventory_items(barcode) WHERE deleted_at IS NULL AND barcode IS NOT NULL;
CREATE INDEX idx_inventory_items_article ON inventory_items(article_number) WHERE deleted_at IS NULL AND article_number IS NOT NULL;
CREATE INDEX idx_inventory_items_active ON inventory_items(is_active) WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_batches_item ON inventory_batches(item_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_inventory_batches_location ON inventory_batches(location_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_inventory_batches_expiry ON inventory_batches(expiry_date) WHERE deleted_at IS NULL AND status = 'available';
CREATE INDEX idx_inventory_batches_status ON inventory_batches(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_inventory_batches_batch_num ON inventory_batches(batch_number) WHERE deleted_at IS NULL;

CREATE INDEX idx_stock_adjustments_item ON stock_adjustments(item_id);
CREATE INDEX idx_stock_adjustments_batch ON stock_adjustments(batch_id) WHERE batch_id IS NOT NULL;
CREATE INDEX idx_stock_adjustments_type ON stock_adjustments(adjustment_type);
CREATE INDEX idx_stock_adjustments_created ON stock_adjustments(created_at DESC);
CREATE INDEX idx_stock_adjustments_ref ON stock_adjustments(reference_type, reference_id) WHERE reference_id IS NOT NULL;

CREATE INDEX idx_inventory_alerts_item ON inventory_alerts(item_id);
CREATE INDEX idx_inventory_alerts_status ON inventory_alerts(status) WHERE status = 'active';
CREATE INDEX idx_inventory_alerts_type ON inventory_alerts(alert_type);

-- Triggers
CREATE TRIGGER storage_rooms_updated_at
    BEFORE UPDATE ON storage_rooms
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER storage_cabinets_updated_at
    BEFORE UPDATE ON storage_cabinets
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER storage_shelves_updated_at
    BEFORE UPDATE ON storage_shelves
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER inventory_items_updated_at
    BEFORE UPDATE ON inventory_items
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER inventory_batches_updated_at
    BEFORE UPDATE ON inventory_batches
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
