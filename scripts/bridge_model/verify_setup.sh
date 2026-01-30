#!/bin/bash

echo "========================================="
echo "Bridge Model Verification"
echo "========================================="
echo ""

echo "1. Auth DB - Tenant Registry"
docker exec -i medflow-db-auth psql -U medflow -d medflow_auth -c "
SELECT id, slug, name, subscription_status
FROM public.tenants
WHERE slug = 'test-practice';
"

echo ""
echo "2. User DB - Tenant Schema"
docker exec -i medflow-db-users psql -U medflow -d medflow_users -c "
SELECT nspname FROM pg_namespace WHERE nspname = 'tenant_test_practice';
"

docker exec -i medflow-db-users psql -U medflow -d medflow_users -c "
SET search_path TO tenant_test_practice;
SELECT email, first_name, last_name, status FROM users WHERE email = 'mohammadamiri.py@gmail.com';
"

echo ""
echo "3. Staff DB - Tenant Schema"
docker exec -i medflow-db-staff psql -U medflow -d medflow_staff -c "
SELECT nspname FROM pg_namespace WHERE nspname = 'tenant_test_practice';
"

echo ""
echo "4. Inventory DB - Tenant Schema"
docker exec -i medflow-db-inventory psql -U medflow -d medflow_inventory -c "
SELECT nspname FROM pg_namespace WHERE nspname = 'tenant_test_practice';
"

echo ""
echo "========================================="
echo "Verification Complete!"
echo "========================================="
