DROP TRIGGER IF EXISTS update_inventory_batches_updated_at ON inventory_batches;
DROP TRIGGER IF EXISTS update_inventory_items_updated_at ON inventory_items;
DROP TRIGGER IF EXISTS update_storage_shelves_updated_at ON storage_shelves;
DROP TRIGGER IF EXISTS update_storage_cabinets_updated_at ON storage_cabinets;
DROP TRIGGER IF EXISTS update_storage_rooms_updated_at ON storage_rooms;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS user_cache;
DROP TABLE IF EXISTS inventory_alerts;
DROP TABLE IF EXISTS stock_adjustments;
DROP TABLE IF EXISTS inventory_batches;
DROP TABLE IF EXISTS inventory_items;
DROP TABLE IF EXISTS storage_shelves;
DROP TABLE IF EXISTS storage_cabinets;
DROP TABLE IF EXISTS storage_rooms;

DROP TYPE IF EXISTS adjustment_type;
DROP TYPE IF EXISTS alert_severity;
DROP TYPE IF EXISTS alert_type;
DROP TYPE IF EXISTS expiry_status;
DROP TYPE IF EXISTS inventory_status;
DROP TYPE IF EXISTS inventory_category;
