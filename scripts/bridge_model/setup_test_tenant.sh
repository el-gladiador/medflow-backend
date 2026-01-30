#!/bin/bash
set -e

echo "========================================="
echo "Bridge Model: Setting Up Test Tenant"
echo "========================================="
echo ""

TENANT="test_practice"
TENANT_ID="a0000000-0000-0000-0000-000000000001"

echo "1. Registering tenant in auth database..."
docker exec -i medflow-db-auth psql -U medflow -d medflow_auth < scripts/bridge_model/create_tenant_registry.sql

echo ""
echo "2. Creating tenant schemas in all service databases..."
make create-tenant TENANT=$TENANT

echo ""
echo "3. Creating dev user in user service..."
docker exec -i medflow-db-users psql -U medflow -d medflow_users < scripts/bridge_model/create_dev_user.sql

echo ""
echo "========================================="
echo "Test Tenant Setup Complete!"
echo "========================================="
echo ""
echo "Tenant Details:"
echo "  Slug: test-practice"
echo "  ID: $TENANT_ID"
echo "  Schema: tenant_$TENANT"
echo ""
echo "Dev User:"
echo "  Email: mohammadamiri.py@gmail.com"
echo "  Password: medflow_test"
echo ""
