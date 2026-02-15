-- Migration 000025: Add PZN (Pharmazentralnummer) column to inventory_items
-- PZN is an 8-digit German pharmaceutical identification number used for medication lookup.

ALTER TABLE inventory.inventory_items ADD COLUMN IF NOT EXISTS pzn VARCHAR(8);

-- Partial index for fast PZN lookups (only non-deleted, non-null PZN)
CREATE INDEX IF NOT EXISTS idx_inventory_items_pzn
    ON inventory.inventory_items(pzn)
    WHERE deleted_at IS NULL AND pzn IS NOT NULL;

-- Grant access to medflow_app role
GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.inventory_items TO medflow_app;
