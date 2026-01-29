-- Remove developer user
DELETE FROM access_giver_scope WHERE user_id = 'a0000000-0000-0000-0000-000000000001'::uuid;
DELETE FROM users WHERE id = 'a0000000-0000-0000-0000-000000000001'::uuid;
