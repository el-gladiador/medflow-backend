# Employee Backfill Scripts

This directory contains scripts for backfilling employee records for existing users.

## Problem

After implementing automatic employee creation for new users, existing users (created before this feature) don't have employee records. This means:
- Admin/tenant can't see themselves in the employee list
- Can't plan vacation/shifts for themselves
- Missing employee records for all pre-existing users

## Solution

The backfill migration (`000005_backfill_employees_for_users.up.sql`) creates employee records for all existing users who don't have them.

## Scripts

### 1. `backfill_employees.sh`

Runs the backfill migration for all tenant schemas.

**Usage:**
```bash
# Using Make (recommended)
cd medflow-backend
make backfill-employees

# Or directly
./scripts/backfill_employees.sh
```

**What it does:**
1. Connects to the staff database
2. Finds all tenant schemas (`tenant_*`)
3. Runs migration `000005_backfill_employees_for_users.up.sql` for each
4. Skips tenants that already have the migration applied
5. Shows summary of successes/failures

**Environment Variables:**
```bash
export DB_USER=medflow           # Default: medflow
export DB_PASSWORD=devpassword   # Default: devpassword
export DB_HOST=localhost         # Default: localhost
export DB_PORT=5435              # Default: 5435
export DB_NAME=medflow_staff     # Default: medflow_staff
export DB_SSL_MODE=disable       # Default: disable
```

### 2. `verify_employee_backfill.sh`

Verifies that all users have employee records.

**Usage:**
```bash
# Using Make (recommended)
cd medflow-backend
make verify-employees

# Or directly
./scripts/verify_employee_backfill.sh
```

**What it does:**
1. Checks each tenant schema
2. Counts users in `user_cache` table
3. Counts employees with `user_id` (linked to users)
4. Counts employees without `user_id` (manual entries)
5. Lists any users missing employee records
6. Shows overall summary

**Sample Output:**
```
======================================
Employee Backfill Verification
======================================

Found 2 tenant schema(s)

tenant_praxis_mueller
----------------------------------------
Users in user_cache:         3
Total employees:             3
  - With user_id (linked):   3
  - Without user_id:         0
✓ All users have employee records

tenant_zahnarzt_berlin
----------------------------------------
Users in user_cache:         5
Total employees:             6
  - With user_id (linked):   5
  - Without user_id:         1
✓ All users have employee records

======================================
Overall Summary
======================================
Total users across all tenants:           8
Total employees across all tenants:       9
  - Employees linked to users:            8
  - Employees without user accounts:      1

✓ SUCCESS: All users have employee records!
```

## Workflow

### First Time Setup (for existing deployments)

```bash
cd medflow-backend

# 1. Verify current state (optional)
make verify-employees

# 2. Run backfill migration
make backfill-employees

# 3. Verify it worked
make verify-employees
```

### For New Tenants

No action needed! Automatic employee creation is enabled for all new users via the `user.created` event consumer.

## Migration Details

**File:** `migrations/staff/tenant/000005_backfill_employees_for_users.up.sql`

**What it does:**
```sql
-- Creates employee records for users who don't have them
INSERT INTO employees (user_id, first_name, last_name, email, employment_type, hire_date, status, job_title)
SELECT
    u.user_id,
    u.first_name,
    u.last_name,
    u.email,
    'full_time',  -- Default employment type
    CURRENT_DATE, -- Hire date = today (can be updated manually)
    'active',     -- Status matches user
    CASE
        WHEN u.role_name = 'admin' THEN 'Administrator'
        WHEN u.role_name = 'manager' THEN 'Manager'
        WHEN u.role_name = 'staff' THEN 'Staff Member'
        WHEN u.role_name = 'viewer' THEN 'Viewer'
        ELSE INITCAP(u.role_name)
    END
FROM user_cache u
WHERE NOT EXISTS (SELECT 1 FROM employees e WHERE e.user_id = u.user_id);
```

**Default values:**
- `employment_type`: `full_time`
- `hire_date`: `CURRENT_DATE` (can be updated manually later)
- `status`: `active`
- `job_title`: Derived from user's role
- `department`: `NULL` (must be set manually)
- `employee_number`: `NULL` (generated separately if needed)

## Troubleshooting

### Script fails with "migrate: command not found"

Install the migrate tool:
```bash
make tools
# Or manually:
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### Script can't connect to database

Check your database configuration:
```bash
# Test connection
psql "postgres://medflow:devpassword@localhost:5435/medflow_staff?sslmode=disable" -c "\dt"

# If using Docker
docker ps | grep medflow-db-staff

# Check if database is running
docker logs medflow-db-staff
```

### Migration already applied

The script will skip tenants that already have the migration applied. This is safe and expected for:
- Running the script multiple times
- Tenants that were already migrated manually

### Some users still missing employees

Check the verification output for details:
```bash
make verify-employees
```

If specific users are missing, the verification script will list them. You can then:
1. Re-run the backfill script
2. Manually create employee records for those users
3. Check if the users exist in `user_cache` table

## Related Files

- Migration: [migrations/staff/tenant/000005_backfill_employees_for_users.up.sql](../migrations/staff/tenant/000005_backfill_employees_for_users.up.sql)
- Consumer: [internal/staff/consumers/user_consumer.go](../internal/staff/consumers/user_consumer.go) (automatic creation for new users)
- Handler: [internal/staff/handler/employee.go](../internal/staff/handler/employee.go) (employee creation with credentials)
- User Client: [internal/staff/client/user_client.go](../internal/staff/client/user_client.go) (service-to-service calls)
