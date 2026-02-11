-- Rollback: Remove seeded admin employee records
DELETE FROM staff.employees WHERE id IN (
    'e1000001-0000-0000-0000-000000000001',
    'e2000002-0000-0000-0000-000000000001'
);
