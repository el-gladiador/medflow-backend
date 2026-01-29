-- Inventory Service Schema
-- Handles items, batches, locations, stock, and alerts

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Enums
CREATE TYPE inventory_category AS ENUM ('Medicine', 'Equipment', 'Supplies');
CREATE TYPE inventory_status AS ENUM ('In Stock', 'Low Stock', 'Critical', 'Out of Stock', 'Expiring');
CREATE TYPE expiry_status AS ENUM ('ok', 'expiring_soon', 'expiring', 'expired');
CREATE TYPE alert_type AS ENUM ('low_stock', 'out_of_stock', 'expiring_soon', 'expiring', 'expired');
CREATE TYPE alert_severity AS ENUM ('warning', 'critical');
CREATE TYPE adjustment_type AS ENUM ('add', 'deduct', 'adjust');

-- Storage rooms
CREATE TABLE storage_rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    code VARCHAR(20) NOT NULL UNIQUE,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Storage cabinets (within rooms)
CREATE TABLE storage_cabinets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES storage_rooms(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(20),
    is_temperature_controlled BOOLEAN DEFAULT FALSE,
    temperature_min DECIMAL(5,2),
    temperature_max DECIMAL(5,2),
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_storage_cabinets_room ON storage_cabinets(room_id);

-- Storage shelves (within cabinets)
CREATE TABLE storage_shelves (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cabinet_id UUID NOT NULL REFERENCES storage_cabinets(id) ON DELETE CASCADE,
    parent_shelf_id UUID REFERENCES storage_shelves(id) ON DELETE CASCADE, -- For sub-shelves
    name VARCHAR(100) NOT NULL,
    code VARCHAR(20),
    position INTEGER NOT NULL,
    capacity VARCHAR(50),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_storage_shelves_cabinet ON storage_shelves(cabinet_id);
CREATE INDEX idx_storage_shelves_parent ON storage_shelves(parent_shelf_id) WHERE parent_shelf_id IS NOT NULL;

-- Inventory items
CREATE TABLE inventory_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    category inventory_category NOT NULL,
    unit VARCHAR(50) NOT NULL,
    price_per_unit DECIMAL(10,2) NOT NULL,
    min_stock INTEGER NOT NULL,
    barcode VARCHAR(100),
    article_number VARCHAR(100),
    supplier VARCHAR(200),
    use_batch_tracking BOOLEAN DEFAULT TRUE,
    requires_cooling BOOLEAN DEFAULT FALSE,
    -- Default location
    default_room_id UUID REFERENCES storage_rooms(id),
    default_cabinet_id UUID REFERENCES storage_cabinets(id),
    default_shelf_id UUID REFERENCES storage_shelves(id),
    -- Metadata
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_inventory_items_category ON inventory_items(category);
CREATE INDEX idx_inventory_items_barcode ON inventory_items(barcode) WHERE barcode IS NOT NULL;
CREATE INDEX idx_inventory_items_article ON inventory_items(article_number) WHERE article_number IS NOT NULL;
CREATE INDEX idx_inventory_items_active ON inventory_items(is_active) WHERE deleted_at IS NULL;

-- Inventory batches (for batch tracking)
CREATE TABLE inventory_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
    batch_number VARCHAR(100) NOT NULL,
    expiry_date DATE NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 0,
    received_date DATE NOT NULL,
    -- Location
    room_id UUID REFERENCES storage_rooms(id),
    cabinet_id UUID REFERENCES storage_cabinets(id),
    shelf_id UUID REFERENCES storage_shelves(id),
    notes TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_inventory_batches_item ON inventory_batches(item_id);
CREATE INDEX idx_inventory_batches_expiry ON inventory_batches(expiry_date);
CREATE INDEX idx_inventory_batches_number ON inventory_batches(item_id, batch_number);
CREATE INDEX idx_inventory_batches_active ON inventory_batches(is_active) WHERE quantity > 0;

-- Stock adjustments (audit trail)
CREATE TABLE stock_adjustments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID NOT NULL REFERENCES inventory_items(id),
    batch_id UUID REFERENCES inventory_batches(id),
    adjustment_type adjustment_type NOT NULL,
    quantity INTEGER NOT NULL, -- Positive for add, negative for deduct
    previous_quantity INTEGER NOT NULL,
    new_quantity INTEGER NOT NULL,
    reason TEXT,
    performed_by UUID NOT NULL, -- user_id from user service
    performed_by_name VARCHAR(255), -- Denormalized for display
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_stock_adjustments_item ON stock_adjustments(item_id);
CREATE INDEX idx_stock_adjustments_batch ON stock_adjustments(batch_id) WHERE batch_id IS NOT NULL;
CREATE INDEX idx_stock_adjustments_user ON stock_adjustments(performed_by);
CREATE INDEX idx_stock_adjustments_created ON stock_adjustments(created_at);

-- Inventory alerts
CREATE TABLE inventory_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_type alert_type NOT NULL,
    item_id UUID NOT NULL REFERENCES inventory_items(id),
    item_name VARCHAR(255) NOT NULL, -- Denormalized
    batch_id UUID REFERENCES inventory_batches(id),
    batch_number VARCHAR(100),
    severity alert_severity NOT NULL,
    message TEXT NOT NULL,
    expiry_date DATE,
    days_until_expiry INTEGER,
    current_stock INTEGER,
    min_stock INTEGER,
    is_acknowledged BOOLEAN DEFAULT FALSE,
    acknowledged_by UUID,
    acknowledged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_inventory_alerts_item ON inventory_alerts(item_id);
CREATE INDEX idx_inventory_alerts_type ON inventory_alerts(alert_type);
CREATE INDEX idx_inventory_alerts_severity ON inventory_alerts(severity);
CREATE INDEX idx_inventory_alerts_unack ON inventory_alerts(is_acknowledged) WHERE is_acknowledged = FALSE;
CREATE INDEX idx_inventory_alerts_created ON inventory_alerts(created_at);

-- Local cache for user data from user-service
CREATE TABLE user_cache (
    user_id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    role_name VARCHAR(50),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Transactional outbox for reliable event publishing
CREATE TABLE outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    routing_key VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    retries INTEGER DEFAULT 0
);

CREATE INDEX idx_outbox_unpublished ON outbox(created_at) WHERE published_at IS NULL;

-- Update timestamp trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_storage_rooms_updated_at
    BEFORE UPDATE ON storage_rooms
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_storage_cabinets_updated_at
    BEFORE UPDATE ON storage_cabinets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_storage_shelves_updated_at
    BEFORE UPDATE ON storage_shelves
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_inventory_items_updated_at
    BEFORE UPDATE ON inventory_items
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_inventory_batches_updated_at
    BEFORE UPDATE ON inventory_batches
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
