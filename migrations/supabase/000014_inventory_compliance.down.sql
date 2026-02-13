-- Rollback: Inventory Regulatory Compliance

DROP TABLE IF EXISTS inventory.item_documents;
DROP TABLE IF EXISTS inventory.hazardous_substance_details;

ALTER TABLE inventory.inventory_batches DROP COLUMN IF EXISTS opened_at;

ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS manufacturer_address;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS ce_marking_number;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS notified_body_id;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS acquisition_date;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS serial_number;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS udi_di;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS udi_pi;
