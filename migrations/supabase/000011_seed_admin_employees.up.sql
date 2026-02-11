-- MedFlow: Create employee records for seeded admin users
-- Admin users exist in users.users but need corresponding staff.employees entries
-- to appear in the staff management list.
-- Runs as superuser 'medflow' which bypasses RLS.

-- Employee record for test-practice admin (Max Müller)
INSERT INTO staff.employees (
    id, tenant_id, user_id, first_name, last_name,
    email, employee_number, job_title, department,
    employment_type, hire_date, status, show_in_staff_list
)
VALUES (
    'e1000001-0000-0000-0000-000000000001',
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'a1000001-0000-0000-0000-000000000001',
    'Max',
    'Müller',
    'admin@praxis-mueller.de',
    'EMP-001',
    'Praxisinhaber',
    'Verwaltung',
    'full_time',
    '2024-01-01',
    'active',
    true
) ON CONFLICT DO NOTHING;

-- Employee record for demo-clinic admin (Lisa Schmidt)
INSERT INTO staff.employees (
    id, tenant_id, user_id, first_name, last_name,
    email, employee_number, job_title, department,
    employment_type, hire_date, status, show_in_staff_list
)
VALUES (
    'e2000002-0000-0000-0000-000000000001',
    'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22',
    'a2000002-0000-0000-0000-000000000001',
    'Lisa',
    'Schmidt',
    'admin@praxis-park.de',
    'EMP-001',
    'Praxisinhaberin',
    'Verwaltung',
    'full_time',
    '2024-01-01',
    'active',
    true
) ON CONFLICT DO NOTHING;
