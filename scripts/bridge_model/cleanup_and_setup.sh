#!/bin/bash
set -e

echo "========================================"
echo "Bridge Model: Complete Setup (Clean)"
echo "========================================"
echo ""

# 1. Clean up existing tenant schemas
echo "1. Cleaning up existing tenant schemas..."
docker exec -i medflow-db-users psql -U medflow -d medflow_users -c "DROP SCHEMA IF EXISTS tenant_test_practice CASCADE;" 2>/dev/null || true
docker exec -i medflow-db-staff psql -U medflow -d medflow_staff -c "DROP SCHEMA IF EXISTS tenant_test_practice CASCADE;" 2>/dev/null || true
docker exec -i medflow-db-inventory psql -U medflow -d medflow_inventory -c "DROP SCHEMA IF EXISTS tenant_test_practice CASCADE;" 2>/dev/null || true

# 2. Fix dirty migration versions
echo "2. Fixing dirty migration versions..."
~/go/bin/migrate -path migrations/user -database "postgresql://medflow:devpassword@localhost:5434/medflow_users?sslmode=disable" force 6 2>/dev/null || true
~/go/bin/migrate -path migrations/staff -database "postgresql://medflow:devpassword@localhost:5435/medflow_staff?sslmode=disable" force 2 2>/dev/null || true
~/go/bin/migrate -path migrations/inventory -database "postgresql://medflow:devpassword@localhost:5436/medflow_inventory?sslmode=disable" force 2 2>/dev/null || true

# 3. Run public schema migrations (ensure function exists)
echo "3. Ensuring public schema migrations are current..."
~/go/bin/migrate -path migrations/user -database "postgresql://medflow:devpassword@localhost:5434/medflow_users?sslmode=disable" up
~/go/bin/migrate -path migrations/staff -database "postgresql://medflow:devpassword@localhost:5435/medflow_staff?sslmode=disable" up
~/go/bin/migrate -path migrations/inventory -database "postgresql://medflow:devpassword@localhost:5436/medflow_inventory?sslmode=disable" up

# 4. Register tenant in auth database
echo ""
echo "4. Registering tenant in auth database..."
docker exec -i medflow-db-auth psql -U medflow -d medflow_auth < scripts/bridge_model/create_tenant_registry.sql

# 5. Create tenant schemas in all services
echo ""
echo "5. Creating tenant schemas..."
make create-tenant TENANT=test_practice

# 6. Create dev user
echo ""
echo "6. Creating dev user..."
docker exec -i medflow-db-users psql -U medflow -d medflow_users < scripts/bridge_model/create_dev_user.sql

echo ""
echo "========================================"
echo "Setup Complete!"
echo "========================================"
