-- MedFlow Schema-per-Tenant: Tenant Schema
-- Migration: Drop time tracking tables

DROP TRIGGER IF EXISTS time_corrections_updated_at ON time_corrections;
DROP TRIGGER IF EXISTS time_breaks_updated_at ON time_breaks;
DROP TRIGGER IF EXISTS time_entries_updated_at ON time_entries;

DROP TABLE IF EXISTS time_corrections;
DROP TABLE IF EXISTS time_breaks;
DROP TABLE IF EXISTS time_entries;
