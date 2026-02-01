# Deprecated Migrations

This directory contains migrations from the old shared-table architecture that has been superseded by the Schema-per-Tenant architecture.

## Why These Are Deprecated

MedFlow originally used a mixed architecture with both shared tables (in each service's public schema) and tenant-scoped tables. This created:

1. **Inconsistency** - Two places to look for data models
2. **Complexity** - Permission tables vs JSONB permissions
3. **Naming conflicts** - German vs English column names
4. **Maintenance burden** - Keeping two systems in sync

## New Architecture

The new pure Schema-per-Tenant architecture:

- **All tenant data** lives in isolated schemas (e.g., `tenant_praxis_mueller`)
- **Only shared infrastructure** lives in public schema (auth DB: tenants, sessions, etc.)
- **Each service** manages its own tenant migrations
- **Permissions** use simple JSONB arrays on the roles table

## Migration Path

These files are kept for:
1. Reference during transition
2. Understanding historical data models
3. Potential rollback scenarios

## File Mapping

| Old (Deprecated) | New Location |
|------------------|--------------|
| `user/000001_init_users.up.sql` | `user/tenant/000001_create_tenant_schema.up.sql` |
| `inventory/000001_init_inventory.up.sql` | `inventory/tenant/000001_create_tenant_schema.up.sql` |
| `staff/000001_init_staff.up.sql` | `staff/tenant/000001_create_tenant_schema.up.sql` |

## Do Not Use

These migrations should NOT be run on new deployments. They exist only for documentation and potential data migration scripts.
