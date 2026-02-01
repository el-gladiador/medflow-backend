#!/bin/bash
set -e

echo "========================================="
echo "Bridge Model: Setting Up SECOND Test Tenant"
echo "========================================="
echo ""
echo "This creates a second tenant for multi-tenancy isolation testing."
echo ""

TENANT="demo_clinic"
TENANT_ID="b0000000-0000-0000-0000-000000000001"

echo "1. Registering second tenant in auth database..."
docker exec -i medflow-db-auth psql -U medflow -d medflow_auth < scripts/bridge_model/create_tenant2_registry.sql

echo ""
echo "2. Creating tenant schemas in all service databases..."
make create-tenant TENANT=$TENANT

echo ""
echo "3. Creating demo user in user service..."
docker exec -i medflow-db-users psql -U medflow -d medflow_users < scripts/bridge_model/create_tenant2_user.sql

echo ""
echo "4. Seeding user-tenant lookup..."
docker exec -i medflow-db-auth psql -U medflow -d medflow_auth < scripts/bridge_model/seed_tenant2_lookup.sql

echo ""
echo "========================================="
echo "Second Test Tenant Setup Complete!"
echo "========================================="
echo ""
echo "Tenant Details:"
echo "  Slug: demo-clinic"
echo "  ID: $TENANT_ID"
echo "  Schema: tenant_$TENANT"
echo ""
echo "Demo User:"
echo "  Email: demo@medflow.de"
echo "  Password: medflow_test"
echo ""
echo "----------------------------------------"
echo "Multi-Tenancy Test Instructions:"
echo "----------------------------------------"
echo "1. Login with mohammadamiri.py@gmail.com (Test Dental Practice)"
echo "2. Logout"
echo "3. Login with demo@medflow.de (Demo Dental Clinic)"
echo "4. Verify that data is isolated between tenants"
echo ""
