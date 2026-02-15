-- Rollback migration 000026: Remove unique constraints on barcode identifiers and stock check

DROP INDEX IF EXISTS inventory.idx_inventory_items_barcode_unique;
DROP INDEX IF EXISTS inventory.idx_inventory_items_article_number_unique;
DROP INDEX IF EXISTS inventory.idx_inventory_items_pzn_unique;

ALTER TABLE inventory.inventory_batches
    DROP CONSTRAINT IF EXISTS chk_batch_current_quantity_non_negative;
