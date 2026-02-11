-- Restore FK constraints to storage_shelves
ALTER TABLE inventory_items
    ADD CONSTRAINT inventory_items_default_location_id_fkey
    FOREIGN KEY (default_location_id) REFERENCES storage_shelves(id);

ALTER TABLE inventory_batches
    ADD CONSTRAINT inventory_batches_location_id_fkey
    FOREIGN KEY (location_id) REFERENCES storage_shelves(id);
