#!/bin/bash

# Backfill Employee Records for Existing Users
# This script runs migration 000005_backfill_employees_for_users.up.sql
# for all existing tenant schemas in the staff database.
#
# Usage:
#   ./scripts/backfill_employees.sh
#
# Environment Variables (optional):
#   DB_USER      - PostgreSQL user (default: medflow)
#   DB_PASSWORD  - PostgreSQL password (default: devpassword)
#   DB_HOST      - PostgreSQL host (default: localhost)
#   DB_PORT      - PostgreSQL port (default: 5435 for staff service)
#   DB_NAME      - Database name (default: medflow_staff)
#   DB_SSL_MODE  - SSL mode (default: disable)

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default database configuration
DB_USER=${DB_USER:-medflow}
DB_PASSWORD=${DB_PASSWORD:-devpassword}
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5435}
DB_NAME=${DB_NAME:-medflow_staff}
DB_SSL_MODE=${DB_SSL_MODE:-disable}

# Migration path
MIGRATION_PATH="migrations/staff/tenant"
MIGRATION_FILE="000005_backfill_employees_for_users.up.sql"

# Database URL
DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}"

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}Employee Backfill Migration Runner${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""
echo -e "Database: ${GREEN}${DB_NAME}@${DB_HOST}:${DB_PORT}${NC}"
echo -e "Migration: ${GREEN}${MIGRATION_FILE}${NC}"
echo ""

# Check if migrate tool is installed
MIGRATE_CMD=""
if command -v migrate &> /dev/null; then
    MIGRATE_CMD="migrate"
elif [ -f "$HOME/go/bin/migrate" ]; then
    MIGRATE_CMD="$HOME/go/bin/migrate"
else
    echo -e "${RED}Error: 'migrate' tool not found${NC}"
    echo "Install with: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
    exit 1
fi

# Check if migration file exists
if [ ! -f "${MIGRATION_PATH}/${MIGRATION_FILE}" ]; then
    echo -e "${RED}Error: Migration file not found: ${MIGRATION_PATH}/${MIGRATION_FILE}${NC}"
    exit 1
fi

# Get list of all tenant schemas
echo -e "${YELLOW}Fetching list of tenant schemas...${NC}"
TENANT_SCHEMAS=$(psql "${DATABASE_URL}" -t -c "SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'tenant_%' ORDER BY schema_name;" 2>&1)

if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Failed to connect to database${NC}"
    echo "${TENANT_SCHEMAS}"
    exit 1
fi

# Convert to array and remove whitespace
TENANT_SCHEMAS_ARRAY=()
while IFS= read -r schema; do
    schema=$(echo "$schema" | xargs)  # Trim whitespace
    if [ -n "$schema" ]; then
        TENANT_SCHEMAS_ARRAY+=("$schema")
    fi
done <<< "$TENANT_SCHEMAS"

TENANT_COUNT=${#TENANT_SCHEMAS_ARRAY[@]}

if [ $TENANT_COUNT -eq 0 ]; then
    echo -e "${YELLOW}No tenant schemas found. Nothing to do.${NC}"
    exit 0
fi

echo -e "${GREEN}Found ${TENANT_COUNT} tenant schema(s)${NC}"
echo ""

# Confirm before proceeding
echo -e "${YELLOW}This will run the backfill migration for the following tenants:${NC}"
for schema in "${TENANT_SCHEMAS_ARRAY[@]}"; do
    echo -e "  - ${schema}"
done
echo ""
read -p "Continue? (y/N): " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Aborted by user${NC}"
    exit 0
fi

# Run migration for each tenant schema
SUCCESS_COUNT=0
FAIL_COUNT=0
SKIPPED_COUNT=0

for schema in "${TENANT_SCHEMAS_ARRAY[@]}"; do
    echo ""
    echo -e "${BLUE}Processing: ${schema}${NC}"
    echo "----------------------------------------"

    # Check if migration was already applied
    TENANT_DB_URL="${DATABASE_URL}&search_path=${schema}"
    CURRENT_VERSION=$($MIGRATE_CMD -path "${MIGRATION_PATH}" -database "${TENANT_DB_URL}" version 2>&1 | grep -oP '\d+' || echo "0")

    # Migration 000005 means version 5
    if [ "$CURRENT_VERSION" -ge 5 ]; then
        echo -e "${GREEN}✓ Migration already applied (version: ${CURRENT_VERSION})${NC}"
        ((SKIPPED_COUNT++))
        continue
    fi

    # Run migration
    if $MIGRATE_CMD -path "${MIGRATION_PATH}" -database "${TENANT_DB_URL}" up; then
        echo -e "${GREEN}✓ Migration applied successfully${NC}"
        ((SUCCESS_COUNT++))

        # Count backfilled employees
        BACKFILLED_COUNT=$(psql "${DATABASE_URL}" -t -c "SET search_path TO ${schema}; SELECT COUNT(*) FROM employees WHERE user_id IS NOT NULL;" 2>/dev/null | xargs)
        if [ -n "$BACKFILLED_COUNT" ] && [ "$BACKFILLED_COUNT" -gt 0 ]; then
            echo -e "${GREEN}  ${BACKFILLED_COUNT} employee record(s) now exist${NC}"
        fi
    else
        echo -e "${RED}✗ Migration failed${NC}"
        ((FAIL_COUNT++))
    fi
done

# Summary
echo ""
echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}Summary${NC}"
echo -e "${BLUE}======================================${NC}"
echo -e "Total tenants: ${TENANT_COUNT}"
echo -e "${GREEN}Successfully migrated: ${SUCCESS_COUNT}${NC}"
echo -e "${YELLOW}Already up-to-date: ${SKIPPED_COUNT}${NC}"
if [ $FAIL_COUNT -gt 0 ]; then
    echo -e "${RED}Failed: ${FAIL_COUNT}${NC}"
fi
echo ""

if [ $FAIL_COUNT -gt 0 ]; then
    echo -e "${RED}Some migrations failed. Please check the errors above.${NC}"
    exit 1
else
    echo -e "${GREEN}All migrations completed successfully!${NC}"
    echo ""
    echo -e "${YELLOW}Next steps:${NC}"
    echo "1. Verify employees were created: make run-staff-service"
    echo "2. Check Staff Dashboard in the frontend"
    echo "3. Admin/tenant users should now appear in the employee list"
fi
