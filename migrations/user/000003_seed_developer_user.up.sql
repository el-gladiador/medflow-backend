-- Add developer user for testing
-- Email: mohammadamiri.py@gmail.com
-- Password: medflow_test (bcrypt hashed)

INSERT INTO users (
    id,
    email,
    password_hash,
    name,
    avatar,
    role_id,
    is_active,
    is_access_giver,
    created_at,
    updated_at
)
SELECT
    'a0000000-0000-0000-0000-000000000001'::uuid,
    'mohammadamiri.py@gmail.com',
    '$2a$10$K4.SJgxG.XpjlPu7GJCqLew.qmK5Yf2iKOQ0YLs2kCAM4NlLWlXRG',
    'Mohammad Amiri',
    NULL,
    r.id,
    true,
    true,
    NOW(),
    NOW()
FROM roles r
WHERE r.name = 'admin'
ON CONFLICT (email) WHERE deleted_at IS NULL DO NOTHING;

-- Grant access giver scope to all roles for the developer
INSERT INTO access_giver_scope (user_id, role_id)
SELECT
    'a0000000-0000-0000-0000-000000000001'::uuid,
    r.id
FROM roles r
WHERE r.name IN ('admin', 'manager', 'MFA', 'Pflege', 'Praktikant', 'other')
ON CONFLICT DO NOTHING;
