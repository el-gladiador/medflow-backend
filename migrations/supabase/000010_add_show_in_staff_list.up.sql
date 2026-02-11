-- Add show_in_staff_list column to control employee visibility in the staff list.
-- Defaults to TRUE so existing employees remain visible.
ALTER TABLE staff.employees ADD COLUMN show_in_staff_list BOOLEAN NOT NULL DEFAULT TRUE;
