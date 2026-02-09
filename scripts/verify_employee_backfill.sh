#!/bin/bash

# Verify Employee Backfill
# This script checks if employees were successfully created for all users
# and displays a summary for each tenant.
#
# Usage:
#   ./scripts/verify_employee_backfill.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Default database configuration
DB_USER=${DB_USER:-medflow}
DB_PASSWORD=${DB_PASSWORD:-devpassword}
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5435}
DB_NAME=${DB_NAME:-medflow_staff}
DB_SSL_MODE=${DB_SSL_MODE:-disable}

DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}"

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}Employee Backfill Verification${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""

# Get list of all tenant schemas
TENANT_SCHEMAS=$(psql "${DATABASE_URL}" -t -c "SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'tenant_%' ORDER BY schema_name;")

if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Failed to connect to database${NC}"
    exit 1
fi

# Convert to array
TENANT_SCHEMAS_ARRAY=()
while IFS= read -r schema; do
    schema=$(echo "$schema" | xargs)
    if [ -n "$schema" ]; then
        TENANT_SCHEMAS_ARRAY+=("$schema")
    fi
done <<< "$TENANT_SCHEMAS"

if [ ${#TENANT_SCHEMAS_ARRAY[@]} -eq 0 ]; then
    echo -e "${YELLOW}No tenant schemas found${NC}"
    exit 0
fi

echo -e "${GREEN}Found ${#TENANT_SCHEMAS_ARRAY[@]} tenant schema(s)${NC}"
echo ""

TOTAL_USERS=0
TOTAL_EMPLOYEES=0
TOTAL_WITH_USER_ID=0
TOTAL_WITHOUT_USER_ID=0

for schema in "${TENANT_SCHEMAS_ARRAY[@]}"; do
    echo -e "${BLUE}${schema}${NC}"
    echo "----------------------------------------"

    # Get user count from user_cache
    USER_COUNT=$(psql "${DATABASE_URL}" -t -c "SET search_path TO ${schema}; SELECT COUNT(*) FROM user_cache;" 2>/dev/null | xargs || echo "0")

    # Get employee counts
    EMPLOYEE_COUNT=$(psql "${DATABASE_URL}" -t -c "SET search_path TO ${schema}; SELECT COUNT(*) FROM employees;" 2>/dev/null | xargs || echo "0")
    WITH_USER_ID=$(psql "${DATABASE_URL}" -t -c "SET search_path TO ${schema}; SELECT COUNT(*) FROM employees WHERE user_id IS NOT NULL;" 2>/dev/null | xargs || echo "0")
    WITHOUT_USER_ID=$(psql "${DATABASE_URL}" -t -c "SET search_path TO ${schema}; SELECT COUNT(*) FROM employees WHERE user_id IS NULL;" 2>/dev/null | xargs || echo "0")

    echo "Users in user_cache:         ${USER_COUNT}"
    echo "Total employees:             ${EMPLOYEE_COUNT}"
    echo "  - With user_id (linked):   ${WITH_USER_ID}"
    echo "  - Without user_id:         ${WITHOUT_USER_ID}"

    # Check if all users have employees
    if [ "$USER_COUNT" -eq "$WITH_USER_ID" ]; then
        echo -e "${GREEN}✓ All users have employee records${NC}"
    elif [ "$USER_COUNT" -gt "$WITH_USER_ID" ]; then
        MISSING=$((USER_COUNT - WITH_USER_ID))
        echo -e "${YELLOW}⚠ ${MISSING} user(s) missing employee records${NC}"

        # Show missing users
        echo -e "${YELLOW}Missing employee records for:${NC}"
        psql "${DATABASE_URL}" -c "SET search_path TO ${schema}; \
            SELECT u.email, u.first_name, u.last_name, u.role_name \
            FROM user_cache u \
            WHERE NOT EXISTS (SELECT 1 FROM employees e WHERE e.user_id = u.user_id) \
            ORDER BY u.email;" 2>/dev/null || true
    fi

    echo ""

    TOTAL_USERS=$((TOTAL_USERS + USER_COUNT))
    TOTAL_EMPLOYEES=$((TOTAL_EMPLOYEES + EMPLOYEE_COUNT))
    TOTAL_WITH_USER_ID=$((TOTAL_WITH_USER_ID + WITH_USER_ID))
    TOTAL_WITHOUT_USER_ID=$((TOTAL_WITHOUT_USER_ID + WITHOUT_USER_ID))
done

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}Overall Summary${NC}"
echo -e "${BLUE}======================================${NC}"
echo "Total users across all tenants:           ${TOTAL_USERS}"
echo "Total employees across all tenants:       ${TOTAL_EMPLOYEES}"
echo "  - Employees linked to users:            ${TOTAL_WITH_USER_ID}"
echo "  - Employees without user accounts:      ${TOTAL_WITHOUT_USER_ID}"
echo ""

if [ "$TOTAL_USERS" -eq "$TOTAL_WITH_USER_ID" ]; then
    echo -e "${GREEN}✓ SUCCESS: All users have employee records!${NC}"
    exit 0
elif [ "$TOTAL_USERS" -gt "$TOTAL_WITH_USER_ID" ]; then
    MISSING=$((TOTAL_USERS - TOTAL_WITH_USER_ID))
    echo -e "${YELLOW}⚠ WARNING: ${MISSING} user(s) are missing employee records${NC}"
    echo -e "${YELLOW}Run: make backfill-employees${NC}"
    exit 1
else
    echo -e "${GREEN}✓ All users accounted for (some employees may not have user accounts)${NC}"
    exit 0
fi
