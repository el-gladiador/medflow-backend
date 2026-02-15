-- Migration 000026: Add unique constraints on barcode identifiers and non-negative stock check
--
-- Prevents duplicate barcodes/PZNs/article numbers within the same tenant.
-- Uses partial unique indexes (WHERE deleted_at IS NULL) so soft-deleted items
-- don't block reuse of identifiers.
-- Also adds a CHECK constraint to prevent negative stock quantities.

-- Unique barcode per tenant (excluding soft-deleted items)
CREATE UNIQUE INDEX IF NOT EXISTS idx_inventory_items_barcode_unique
    ON inventory.inventory_items(tenant_id, barcode)
    WHERE deleted_at IS NULL AND barcode IS NOT NULL AND barcode != '';

-- Unique article_number per tenant (excluding soft-deleted items)
CREATE UNIQUE INDEX IF NOT EXISTS idx_inventory_items_article_number_unique
    ON inventory.inventory_items(tenant_id, article_number)
    WHERE deleted_at IS NULL AND article_number IS NOT NULL AND article_number != '';

-- Unique PZN per tenant (excluding soft-deleted items)
CREATE UNIQUE INDEX IF NOT EXISTS idx_inventory_items_pzn_unique
    ON inventory.inventory_items(tenant_id, pzn)
    WHERE deleted_at IS NULL AND pzn IS NOT NULL AND pzn != '';

-- Prevent negative stock quantities
ALTER TABLE inventory.inventory_batches
    ADD CONSTRAINT chk_batch_current_quantity_non_negative
    CHECK (current_quantity >= 0);

-- Grant permissions to the app role
GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.inventory_items TO medflow_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.inventory_batches TO medflow_app;
