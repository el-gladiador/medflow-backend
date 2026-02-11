-- Relax location FK constraints to accept any location UUID (room, cabinet, or shelf)
-- Previously these only accepted storage_shelves(id), preventing room/cabinet-level assignments.

-- Drop the FK on inventory_items.default_location_id
ALTER TABLE inventory_items DROP CONSTRAINT IF EXISTS inventory_items_default_location_id_fkey;

-- Drop the FK on inventory_batches.location_id
ALTER TABLE inventory_batches DROP CONSTRAINT IF EXISTS inventory_batches_location_id_fkey;
