-- MedFlow: FORCE RLS on remaining inventory tables
--
-- Migration 000008 missed 4 inventory tables when applying FORCE ROW LEVEL SECURITY.
-- Without FORCE RLS, the table owner could bypass RLS policies.
-- This migration closes that security gap.

ALTER TABLE inventory.storage_cabinets FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory.storage_shelves FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory.stock_adjustments FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory.user_cache FORCE ROW LEVEL SECURITY;
