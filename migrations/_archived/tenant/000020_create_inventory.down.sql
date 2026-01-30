-- Rollback: Drop inventory tables

DROP TRIGGER IF EXISTS storage_rooms_updated_at ON storage_rooms;
DROP TRIGGER IF EXISTS storage_cabinets_updated_at ON storage_cabinets;
DROP TRIGGER IF EXISTS storage_shelves_updated_at ON storage_shelves;
DROP TRIGGER IF EXISTS inventory_items_updated_at ON inventory_items;
DROP TRIGGER IF EXISTS inventory_batches_updated_at ON inventory_batches;

DROP TABLE IF EXISTS inventory_alerts;
DROP TABLE IF EXISTS stock_adjustments;
DROP TABLE IF EXISTS inventory_batches;
DROP TABLE IF EXISTS inventory_items;
DROP TABLE IF EXISTS storage_shelves;
DROP TABLE IF EXISTS storage_cabinets;
DROP TABLE IF EXISTS storage_rooms;
