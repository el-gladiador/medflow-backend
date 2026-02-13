-- MedFlow: Alert Scheduler Support
-- Adds deduplication index for alert upsert operations, expands alert_type
-- constraint to include 'expiring', and adds scanner lookup index.

-- ============================================================================
-- 1. Deduplication index for alert scanner upsert
-- Ensures only one active/acknowledged alert per (tenant, item, batch, type)
-- Uses sentinel UUID for NULL batch_id to enable unique constraint
-- ============================================================================
CREATE UNIQUE INDEX idx_inventory_alerts_dedup
    ON inventory.inventory_alerts(
        tenant_id,
        item_id,
        COALESCE(batch_id, '00000000-0000-0000-0000-000000000000'),
        alert_type
    )
    WHERE status IN ('open', 'acknowledged');

-- ============================================================================
-- 2. Expand alert_type constraint to include 'expiring'
-- Used by batch expiry scanner code but missing from prior constraint
-- ============================================================================
ALTER TABLE inventory.inventory_alerts DROP CONSTRAINT IF EXISTS alerts_type_valid;
ALTER TABLE inventory.inventory_alerts ADD CONSTRAINT alerts_type_valid CHECK (
    alert_type IN (
        'low_stock', 'expiring_soon', 'expiring', 'expired', 'temperature',
        'reorder', 'out_of_stock',
        'stk_overdue', 'mtk_overdue', 'stk_due_soon', 'mtk_due_soon',
        'temperature_excursion', 'temperature_missing',
        'opening_expiry_soon', 'opening_expired'
    )
);

-- ============================================================================
-- 3. Index for alert scanner batch-expiry lookups
-- ============================================================================
CREATE INDEX IF NOT EXISTS idx_inventory_alerts_scanner_lookup
    ON inventory.inventory_alerts(tenant_id, item_id, alert_type, status);
