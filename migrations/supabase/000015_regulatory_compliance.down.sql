-- Rollback: Regulatory Compliance migration

-- Drop new tables (reverse order of creation)
DROP TABLE IF EXISTS inventory.temperature_readings;
DROP TABLE IF EXISTS inventory.device_incidents;
DROP TABLE IF EXISTS inventory.device_trainings;
DROP TABLE IF EXISTS inventory.device_inspections;

-- Remove new columns from inventory_items
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS is_medical_device;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS device_type;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS device_model;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS authorized_representative;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS importer;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS operational_id_number;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS location_assignment;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS risk_class;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS stk_interval_months;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS mtk_interval_months;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS last_stk_date;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS next_stk_due;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS last_mtk_date;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS next_mtk_due;
ALTER TABLE inventory.inventory_items DROP COLUMN IF EXISTS shelf_life_after_opening_days;

-- Remove new columns from storage_cabinets
ALTER TABLE inventory.storage_cabinets DROP COLUMN IF EXISTS min_temperature_celsius;
ALTER TABLE inventory.storage_cabinets DROP COLUMN IF EXISTS max_temperature_celsius;
ALTER TABLE inventory.storage_cabinets DROP COLUMN IF EXISTS temperature_monitoring_enabled;

-- Restore original alert type constraint
ALTER TABLE inventory.inventory_alerts DROP CONSTRAINT IF EXISTS alerts_type_valid;
ALTER TABLE inventory.inventory_alerts ADD CONSTRAINT alerts_type_valid CHECK (
    alert_type IN ('low_stock', 'expiring_soon', 'expired', 'temperature', 'reorder')
);

-- Restore original document type constraint
ALTER TABLE inventory.item_documents DROP CONSTRAINT IF EXISTS item_documents_type_valid;
ALTER TABLE inventory.item_documents ADD CONSTRAINT item_documents_type_valid CHECK (
    document_type IN ('sdb', 'manual', 'certificate')
);
