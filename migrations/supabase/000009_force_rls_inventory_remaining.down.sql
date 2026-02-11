-- Rollback: Remove FORCE RLS from the 4 inventory tables added in 000009

ALTER TABLE inventory.storage_cabinets NO FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory.storage_shelves NO FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory.stock_adjustments NO FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory.user_cache NO FORCE ROW LEVEL SECURITY;
