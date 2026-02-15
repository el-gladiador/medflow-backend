-- Rollback migration 000025: Remove PZN column from inventory_items

DROP INDEX IF EXISTS inventory.idx_inventory_items_pzn;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS pzn;
