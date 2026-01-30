# MedFlow Multi-Tenancy Design

**Version**: 2.0
**Architecture**: Schema-per-Tenant
**Status**: Approved
**Last Updated**: 2026-01-30

## Table of Contents

1. [Overview](#overview)
2. [Architecture Pattern](#architecture-pattern)
3. [AWS Infrastructure](#aws-infrastructure)
4. [Public Schema: Tenant Registry](#public-schema-tenant-registry)
5. [Tenant Schema Structure](#tenant-schema-structure)
6. [Go Service Layer Patterns](#go-service-layer-patterns)
7. [Tenant Lifecycle Management](#tenant-lifecycle-management)
8. [Migration Strategy](#migration-strategy)
9. [Analytics Pipeline](#analytics-pipeline)
10. [Security & Compliance](#security--compliance)
11. [Testing Requirements](#testing-requirements)

---

## Overview

MedFlow uses a **Schema-per-Tenant** multi-tenancy architecture optimized for German healthcare compliance. Each customer (dental/medical practice) operates in a completely isolated PostgreSQL schema within a shared Aurora database cluster.

### Design Goals

1. **True Data Isolation**: Database-engine enforced separation (not just application-level)
2. **German Compliance**: GDPR, healthcare regulations, AVV (Auftragsverarbeitungsvertrag)
3. **Future Customization**: Per-tenant schema modifications without affecting others
4. **Analytics Ready**: Centralized analytics via data lake (anonymized)
5. **Cost Efficiency**: Shared compute with Aurora Serverless v2

### Why Schema-per-Tenant?

| Approach | Isolation Level | GDPR Deletion | Custom Logic | Cost |
|----------|-----------------|---------------|--------------|------|
| Discriminator (Row-Level) | Software | Complex | Difficult | Low |
| **Schema-per-Tenant** | **Database Engine** | **DROP SCHEMA** | **Easy** | **Medium** |
| Database-per-Tenant | Hardware | Easy | Easy | Very High |

For German healthcare SaaS, Schema-per-Tenant provides the optimal balance of isolation, compliance, and cost.

---

## Architecture Pattern

### High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           AWS eu-central-1 (Frankfurt)                        │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                               │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────────────────────┐    │
│   │   Request   │────▶│ API Gateway │────▶│      Go Microservices       │    │
│   │  (JWT+Tenant)│    │ (Validate)  │     │  (EKS / Kubernetes)         │    │
│   └─────────────┘     └─────────────┘     └──────────────┬──────────────┘    │
│                                                          │                    │
│                              ┌────────────────────────────┘                   │
│                              │ SET search_path TO tenant_xxx                  │
│                              ▼                                                │
│   ┌─────────────────────────────────────────────────────────────────────┐    │
│   │                  Aurora PostgreSQL Serverless v2                     │    │
│   │  ┌───────────────────────────────────────────────────────────────┐  │    │
│   │  │                    public schema                               │  │    │
│   │  │  ┌─────────────────────────────────────────────────────────┐  │  │    │
│   │  │  │ tenants: id, slug, schema_name, status, kms_key_arn ... │  │  │    │
│   │  │  └─────────────────────────────────────────────────────────┘  │  │    │
│   │  └───────────────────────────────────────────────────────────────┘  │    │
│   │                                                                      │    │
│   │  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐         │    │
│   │  │ tenant_praxis_a│  │ tenant_praxis_b│  │ tenant_praxis_c│  ...    │    │
│   │  │ ├─ users       │  │ ├─ users       │  │ ├─ users       │         │    │
│   │  │ ├─ inventory   │  │ ├─ inventory   │  │ ├─ inventory   │         │    │
│   │  │ ├─ staff       │  │ ├─ staff       │  │ ├─ staff       │         │    │
│   │  │ └─ ...         │  │ └─ ...         │  │ └─ ...         │         │    │
│   │  └────────────────┘  └────────────────┘  └────────────────┘         │    │
│   └─────────────────────────────────────────────────────────────────────┘    │
│                                                                               │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Request Flow

1. **Client Request**: JWT contains `tenant_id` claim
2. **API Gateway**: Validates JWT, extracts tenant info
3. **Microservice**: Receives request with tenant context
4. **Database Connection**: `SET search_path TO tenant_xxx`
5. **Query Execution**: All queries run within tenant's schema
6. **Response**: Data from tenant's isolated namespace

### Isolation Guarantee

Even if application code has a bug and tries to access another tenant's data:
- PostgreSQL rejects the query (table doesn't exist in current search_path)
- No `WHERE organization_id = ?` needed (schema IS the filter)
- Database-level audit logs show exactly which schema was accessed

---

## AWS Infrastructure

### Core Services

| Component | AWS Service | Configuration |
|-----------|-------------|---------------|
| **Database** | Aurora PostgreSQL Serverless v2 | Multi-AZ, eu-central-1 |
| **Compute** | EKS (Kubernetes) | Managed node groups |
| **Secrets** | Secrets Manager | DB credentials, per-tenant if needed |
| **Encryption** | KMS | Per-tenant CMKs for premium tier |
| **Events** | Amazon MQ | Managed RabbitMQ |
| **Analytics** | DMS → S3 → Redshift | CDC pipeline, anonymized |
| **Monitoring** | CloudWatch | Logs, metrics, alarms |

### Aurora Configuration

```yaml
# terraform/aurora.tf (conceptual)
resource "aws_rds_cluster" "medflow" {
  cluster_identifier      = "medflow-production"
  engine                  = "aurora-postgresql"
  engine_mode             = "provisioned"
  engine_version          = "15.4"
  database_name           = "medflow"
  master_username         = "medflow_admin"

  serverlessv2_scaling_configuration {
    min_capacity = 0.5   # Scale to zero at night
    max_capacity = 64    # Scale up during peak hours
  }

  # German data sovereignty
  availability_zones = ["eu-central-1a", "eu-central-1b", "eu-central-1c"]

  # Encryption at rest
  storage_encrypted = true
  kms_key_id        = aws_kms_key.medflow_master.arn

  # Backup for compliance
  backup_retention_period = 35  # 35 days
  preferred_backup_window = "02:00-03:00"
}
```

### Database Users

```sql
-- Admin user (for migrations, schema creation)
CREATE ROLE medflow_admin WITH LOGIN PASSWORD 'xxx' CREATEDB CREATEROLE;

-- Application user (for microservices)
CREATE ROLE medflow_app WITH LOGIN PASSWORD 'yyy';
GRANT CONNECT ON DATABASE medflow TO medflow_app;

-- Per-tenant users (optional, for premium tier)
-- CREATE ROLE tenant_praxis_a_user WITH LOGIN PASSWORD 'zzz';
-- GRANT USAGE ON SCHEMA tenant_praxis_a TO tenant_praxis_a_user;
-- GRANT ALL ON ALL TABLES IN SCHEMA tenant_praxis_a TO tenant_praxis_a_user;
```

---

## Public Schema: Tenant Registry

The `public` schema contains the tenant registry - the only shared data.

### tenants table

```sql
-- Migration: migrations/public/000001_create_tenants.up.sql

CREATE TABLE public.tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Identity
    slug VARCHAR(100) NOT NULL UNIQUE,           -- URL-friendly: "praxis-mueller"
    schema_name VARCHAR(100) NOT NULL UNIQUE,    -- DB schema: "tenant_praxis_mueller"
    name VARCHAR(255) NOT NULL,                  -- Display: "Zahnarztpraxis Dr. Müller"

    -- Contact
    email VARCHAR(255),
    phone VARCHAR(50),

    -- Address (German format)
    street VARCHAR(255),
    city VARCHAR(100),
    postal_code VARCHAR(20),
    country VARCHAR(100) DEFAULT 'Germany',

    -- Subscription & Billing
    subscription_tier VARCHAR(50) NOT NULL DEFAULT 'standard',
    subscription_status VARCHAR(50) NOT NULL DEFAULT 'active',
    trial_ends_at TIMESTAMPTZ,
    billing_email VARCHAR(255),

    -- Security
    kms_key_arn VARCHAR(255),        -- Per-tenant encryption key (premium)
    db_user VARCHAR(100),            -- Dedicated DB user (premium)

    -- Feature Flags
    features JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Limits
    max_users INTEGER NOT NULL DEFAULT 50,
    max_storage_gb INTEGER NOT NULL DEFAULT 10,

    -- Settings
    settings JSONB NOT NULL DEFAULT '{
        "language": "de",
        "timezone": "Europe/Berlin",
        "dateFormat": "DD.MM.YYYY",
        "currency": "EUR"
    }'::jsonb,

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,  -- Soft delete (keep record for audit trail)

    -- Constraints
    CONSTRAINT tenants_slug_format CHECK (slug ~ '^[a-z0-9][a-z0-9-]*[a-z0-9]$'),
    CONSTRAINT tenants_tier_valid CHECK (subscription_tier IN ('free', 'standard', 'premium', 'enterprise')),
    CONSTRAINT tenants_status_valid CHECK (subscription_status IN ('active', 'trial', 'suspended', 'cancelled'))
);

-- Indexes
CREATE INDEX idx_tenants_slug ON public.tenants(slug) WHERE deleted_at IS NULL;
CREATE INDEX idx_tenants_status ON public.tenants(subscription_status) WHERE deleted_at IS NULL;
CREATE INDEX idx_tenants_schema ON public.tenants(schema_name) WHERE deleted_at IS NULL;

-- Updated_at trigger
CREATE OR REPLACE FUNCTION public.update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenants_updated_at
    BEFORE UPDATE ON public.tenants
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
```

### tenant_audit_log table

System-wide audit log for tenant lifecycle events:

```sql
-- Migration: migrations/public/000002_create_tenant_audit_log.up.sql

CREATE TABLE public.tenant_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),

    event_type VARCHAR(50) NOT NULL,  -- created, suspended, deleted, schema_migrated, etc.
    event_data JSONB DEFAULT '{}',

    performed_by UUID,        -- Admin user ID
    performed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    ip_address INET,
    user_agent TEXT
);

CREATE INDEX idx_tenant_audit_tenant ON public.tenant_audit_log(tenant_id);
CREATE INDEX idx_tenant_audit_type ON public.tenant_audit_log(event_type);
CREATE INDEX idx_tenant_audit_time ON public.tenant_audit_log(performed_at DESC);
```

---

## Tenant Schema Structure

Each tenant gets an identical schema with all service tables.

### Schema Naming Convention

```
tenant_{slug}

Examples:
- tenant_praxis_mueller
- tenant_zahnarzt_berlin
- tenant_mvz_hamburg
```

### Template Tables

All tables within a tenant schema follow this pattern:

```sql
-- Template: Every table in tenant schema
CREATE TABLE {table_name} (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Domain-specific columns
    -- ...

    -- Standard timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,  -- Soft delete for GDPR audit trail

    -- Optional: Who created/modified
    created_by UUID,
    updated_by UUID
);

-- Standard updated_at trigger (shared function from public schema)
CREATE TRIGGER {table_name}_updated_at
    BEFORE UPDATE ON {table_name}
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
```

### Example: Inventory Tables in Tenant Schema

```sql
-- Migration: migrations/tenant/000010_create_inventory.up.sql

-- Storage Rooms
CREATE TABLE storage_rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    floor VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Storage Cabinets
CREATE TABLE storage_cabinets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES storage_rooms(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    temperature_controlled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Inventory Items
CREATE TABLE inventory_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100),
    unit VARCHAR(50) NOT NULL,
    min_stock INTEGER NOT NULL DEFAULT 0,
    barcode VARCHAR(100),
    article_number VARCHAR(100),
    use_batch_tracking BOOLEAN DEFAULT FALSE,
    requires_cooling BOOLEAN DEFAULT FALSE,
    default_location_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Indexes (within each tenant schema)
CREATE INDEX idx_inventory_items_name ON inventory_items(name) WHERE deleted_at IS NULL;
CREATE INDEX idx_inventory_items_category ON inventory_items(category) WHERE deleted_at IS NULL;
CREATE INDEX idx_inventory_items_barcode ON inventory_items(barcode) WHERE deleted_at IS NULL;
```

### Note: No organization_id Column

Unlike Row-Level multi-tenancy, tables do **NOT** have an `organization_id` column. The schema itself IS the tenant boundary.

```sql
-- Row-Level approach (NOT used):
SELECT * FROM inventory_items WHERE organization_id = 'xxx';

-- Schema-per-Tenant approach (USED):
SET search_path TO tenant_praxis_mueller;
SELECT * FROM inventory_items;  -- Only sees this tenant's data
```

---

## Go Service Layer Patterns

### Tenant Connection Manager

```go
// pkg/database/tenant.go

package database

import (
    "context"
    "fmt"
    "sync"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

// TenantConnectionManager manages database connections with tenant context
type TenantConnectionManager struct {
    pool   *pgxpool.Pool
    mu     sync.RWMutex
    cache  map[string]*TenantInfo
}

type TenantInfo struct {
    ID         string
    Slug       string
    SchemaName string
    Status     string
}

// NewTenantConnectionManager creates a new connection manager
func NewTenantConnectionManager(pool *pgxpool.Pool) *TenantConnectionManager {
    return &TenantConnectionManager{
        pool:  pool,
        cache: make(map[string]*TenantInfo),
    }
}

// AcquireForTenant gets a connection with search_path set to tenant's schema
func (m *TenantConnectionManager) AcquireForTenant(ctx context.Context, tenantID string) (*pgxpool.Conn, error) {
    // Get tenant info (from cache or DB)
    tenant, err := m.getTenantInfo(ctx, tenantID)
    if err != nil {
        return nil, fmt.Errorf("tenant not found: %w", err)
    }

    if tenant.Status != "active" && tenant.Status != "trial" {
        return nil, fmt.Errorf("tenant suspended or cancelled")
    }

    // Acquire connection from pool
    conn, err := m.pool.Acquire(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to acquire connection: %w", err)
    }

    // Set search_path to tenant's schema
    // This is the KEY isolation mechanism
    _, err = conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", pgx.Identifier{tenant.SchemaName}.Sanitize()))
    if err != nil {
        conn.Release()
        return nil, fmt.Errorf("failed to set search_path: %w", err)
    }

    return conn, nil
}

// getTenantInfo retrieves tenant info with caching
func (m *TenantConnectionManager) getTenantInfo(ctx context.Context, tenantID string) (*TenantInfo, error) {
    // Check cache first
    m.mu.RLock()
    if info, ok := m.cache[tenantID]; ok {
        m.mu.RUnlock()
        return info, nil
    }
    m.mu.RUnlock()

    // Query from public.tenants
    var info TenantInfo
    err := m.pool.QueryRow(ctx, `
        SELECT id, slug, schema_name, subscription_status
        FROM public.tenants
        WHERE id = $1 AND deleted_at IS NULL
    `, tenantID).Scan(&info.ID, &info.Slug, &info.SchemaName, &info.Status)

    if err != nil {
        return nil, err
    }

    // Update cache
    m.mu.Lock()
    m.cache[tenantID] = &info
    m.mu.Unlock()

    return &info, nil
}

// InvalidateCache removes a tenant from the cache (call after updates)
func (m *TenantConnectionManager) InvalidateCache(tenantID string) {
    m.mu.Lock()
    delete(m.cache, tenantID)
    m.mu.Unlock()
}
```

### Context Keys and Helpers

```go
// pkg/tenant/context.go

package tenant

import (
    "context"
    "errors"

    "github.com/jackc/pgx/v5/pgxpool"
)

type contextKey string

const (
    tenantIDKey contextKey = "tenant_id"
    tenantDBKey contextKey = "tenant_db"
)

var (
    ErrNoTenantInContext = errors.New("no tenant in context")
    ErrNoDBInContext     = errors.New("no database connection in context")
)

// WithTenantID adds tenant ID to context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
    return context.WithValue(ctx, tenantIDKey, tenantID)
}

// TenantID extracts tenant ID from context
func TenantID(ctx context.Context) (string, error) {
    id, ok := ctx.Value(tenantIDKey).(string)
    if !ok || id == "" {
        return "", ErrNoTenantInContext
    }
    return id, nil
}

// WithDB adds tenant-scoped database connection to context
func WithDB(ctx context.Context, conn *pgxpool.Conn) context.Context {
    return context.WithValue(ctx, tenantDBKey, conn)
}

// DB extracts tenant-scoped database connection from context
func DB(ctx context.Context) (*pgxpool.Conn, error) {
    conn, ok := ctx.Value(tenantDBKey).(*pgxpool.Conn)
    if !ok || conn == nil {
        return nil, ErrNoDBInContext
    }
    return conn, nil
}
```

### Tenant Middleware

```go
// internal/middleware/tenant.go

package middleware

import (
    "net/http"

    "medflow/pkg/database"
    "medflow/pkg/tenant"
)

// TenantMiddleware extracts tenant from JWT and sets up DB connection
func TenantMiddleware(tcm *database.TenantConnectionManager) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx := r.Context()

            // Extract tenant_id from JWT claims (set by auth middleware)
            tenantID, ok := ctx.Value("jwt_tenant_id").(string)
            if !ok || tenantID == "" {
                http.Error(w, `{"error": "missing tenant context"}`, http.StatusForbidden)
                return
            }

            // Acquire tenant-scoped connection
            conn, err := tcm.AcquireForTenant(ctx, tenantID)
            if err != nil {
                http.Error(w, `{"error": "tenant not accessible"}`, http.StatusForbidden)
                return
            }
            defer conn.Release()

            // Add to context
            ctx = tenant.WithTenantID(ctx, tenantID)
            ctx = tenant.WithDB(ctx, conn)

            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### Repository Pattern (No org_id filtering needed!)

```go
// internal/inventory/repository/inventory_repository.go

package repository

import (
    "context"

    "github.com/google/uuid"
    "medflow/pkg/tenant"
)

type InventoryRepository struct{}

// GetItems retrieves all items - tenant isolation is via search_path
func (r *InventoryRepository) GetItems(ctx context.Context) ([]InventoryItem, error) {
    conn, err := tenant.DB(ctx)
    if err != nil {
        return nil, err
    }

    // No WHERE organization_id needed - search_path restricts to tenant schema
    rows, err := conn.Query(ctx, `
        SELECT id, name, category, unit, min_stock, barcode, article_number,
               use_batch_tracking, requires_cooling, default_location_id,
               created_at, updated_at
        FROM inventory_items
        WHERE deleted_at IS NULL
        ORDER BY name
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var items []InventoryItem
    for rows.Next() {
        var item InventoryItem
        err := rows.Scan(
            &item.ID, &item.Name, &item.Category, &item.Unit, &item.MinStock,
            &item.Barcode, &item.ArticleNumber, &item.UseBatchTracking,
            &item.RequiresCooling, &item.DefaultLocationID,
            &item.CreatedAt, &item.UpdatedAt,
        )
        if err != nil {
            return nil, err
        }
        items = append(items, item)
    }

    return items, nil
}

// CreateItem creates a new item in the tenant's schema
func (r *InventoryRepository) CreateItem(ctx context.Context, item *InventoryItem) error {
    conn, err := tenant.DB(ctx)
    if err != nil {
        return err
    }

    item.ID = uuid.New()

    // No organization_id to set - we're already in the tenant's schema
    _, err = conn.Exec(ctx, `
        INSERT INTO inventory_items (
            id, name, category, unit, min_stock, barcode, article_number,
            use_batch_tracking, requires_cooling, default_location_id
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `,
        item.ID, item.Name, item.Category, item.Unit, item.MinStock,
        item.Barcode, item.ArticleNumber, item.UseBatchTracking,
        item.RequiresCooling, item.DefaultLocationID,
    )

    return err
}
```

---

## Tenant Lifecycle Management

### Tenant Service

```go
// pkg/tenant/service.go

package tenant

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type TenantService struct {
    pool     *pgxpool.Pool
    migrator *Migrator
    kms      KMSClient  // Optional: AWS KMS for premium encryption
}

type CreateTenantInput struct {
    Slug             string
    Name             string
    Email            string
    SubscriptionTier string
}

// CreateTenant provisions a new tenant with their own schema
func (s *TenantService) CreateTenant(ctx context.Context, input CreateTenantInput) (*Tenant, error) {
    schemaName := fmt.Sprintf("tenant_%s", sanitizeSlug(input.Slug))

    tenant := &Tenant{
        ID:               uuid.New(),
        Slug:             input.Slug,
        SchemaName:       schemaName,
        Name:             input.Name,
        Email:            input.Email,
        SubscriptionTier: input.SubscriptionTier,
        Status:           "active",
    }

    // Start transaction
    tx, err := s.pool.Begin(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback(ctx)

    // 1. Create tenant record in public.tenants
    _, err = tx.Exec(ctx, `
        INSERT INTO public.tenants (id, slug, schema_name, name, email, subscription_tier, subscription_status)
        VALUES ($1, $2, $3, $4, $5, $6, 'active')
    `, tenant.ID, tenant.Slug, tenant.SchemaName, tenant.Name, tenant.Email, tenant.SubscriptionTier)
    if err != nil {
        return nil, fmt.Errorf("failed to create tenant record: %w", err)
    }

    // 2. Create schema
    _, err = tx.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", schemaName))
    if err != nil {
        return nil, fmt.Errorf("failed to create schema: %w", err)
    }

    // 3. Grant permissions to app user
    _, err = tx.Exec(ctx, fmt.Sprintf(`
        GRANT USAGE ON SCHEMA %s TO medflow_app;
        GRANT ALL ON ALL TABLES IN SCHEMA %s TO medflow_app;
        GRANT ALL ON ALL SEQUENCES IN SCHEMA %s TO medflow_app;
        ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT ALL ON TABLES TO medflow_app;
        ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT ALL ON SEQUENCES TO medflow_app;
    `, schemaName, schemaName, schemaName, schemaName, schemaName))
    if err != nil {
        return nil, fmt.Errorf("failed to grant permissions: %w", err)
    }

    // Commit transaction
    if err = tx.Commit(ctx); err != nil {
        return nil, err
    }

    // 4. Run migrations on new schema (outside transaction)
    if err = s.migrator.RunForSchema(ctx, schemaName); err != nil {
        // Rollback: drop schema if migrations fail
        s.pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA %s CASCADE", schemaName))
        s.pool.Exec(ctx, "DELETE FROM public.tenants WHERE id = $1", tenant.ID)
        return nil, fmt.Errorf("failed to run migrations: %w", err)
    }

    // 5. Optional: Create KMS key for premium tier
    if input.SubscriptionTier == "premium" || input.SubscriptionTier == "enterprise" {
        keyARN, err := s.kms.CreateKey(ctx, tenant.ID.String())
        if err == nil {
            s.pool.Exec(ctx, "UPDATE public.tenants SET kms_key_arn = $1 WHERE id = $2", keyARN, tenant.ID)
            tenant.KMSKeyARN = keyARN
        }
    }

    // 6. Log audit event
    s.logAuditEvent(ctx, tenant.ID, "created", map[string]interface{}{
        "slug": tenant.Slug,
        "tier": tenant.SubscriptionTier,
    })

    return tenant, nil
}

// DeleteTenant completely removes a tenant (GDPR Right to be Forgotten)
func (s *TenantService) DeleteTenant(ctx context.Context, tenantID uuid.UUID) error {
    // Get tenant info
    var schemaName, kmsKeyARN string
    err := s.pool.QueryRow(ctx, `
        SELECT schema_name, COALESCE(kms_key_arn, '')
        FROM public.tenants WHERE id = $1
    `, tenantID).Scan(&schemaName, &kmsKeyARN)
    if err != nil {
        return fmt.Errorf("tenant not found: %w", err)
    }

    // 1. DROP SCHEMA CASCADE - instantly deletes ALL tenant data
    _, err = s.pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
    if err != nil {
        return fmt.Errorf("failed to drop schema: %w", err)
    }

    // 2. Crypto-shredding: destroy encryption key (data becomes unrecoverable)
    if kmsKeyARN != "" {
        s.kms.ScheduleKeyDeletion(ctx, kmsKeyARN, 7) // 7-day waiting period
    }

    // 3. Soft-delete tenant record (keep for audit trail)
    _, err = s.pool.Exec(ctx, `
        UPDATE public.tenants
        SET deleted_at = NOW(), subscription_status = 'cancelled'
        WHERE id = $1
    `, tenantID)
    if err != nil {
        return fmt.Errorf("failed to mark tenant deleted: %w", err)
    }

    // 4. Log audit event
    s.logAuditEvent(ctx, tenantID, "deleted", map[string]interface{}{
        "schema_dropped":  true,
        "kms_key_deleted": kmsKeyARN != "",
    })

    return nil
}

// SuspendTenant suspends a tenant (keeps data but blocks access)
func (s *TenantService) SuspendTenant(ctx context.Context, tenantID uuid.UUID, reason string) error {
    _, err := s.pool.Exec(ctx, `
        UPDATE public.tenants
        SET subscription_status = 'suspended',
            settings = settings || jsonb_build_object('suspend_reason', $2)
        WHERE id = $1
    `, tenantID, reason)

    if err == nil {
        s.logAuditEvent(ctx, tenantID, "suspended", map[string]interface{}{"reason": reason})
    }

    return err
}
```

---

## Migration Strategy

### Directory Structure

```
medflow-backend/
├── migrations/
│   ├── public/                    # Shared tenant registry (run once)
│   │   ├── 000001_create_tenants.up.sql
│   │   ├── 000001_create_tenants.down.sql
│   │   ├── 000002_create_tenant_audit_log.up.sql
│   │   └── 000002_create_tenant_audit_log.down.sql
│   │
│   └── tenant/                    # Per-tenant schema (run for each tenant)
│       ├── 000001_create_base_tables.up.sql
│       ├── 000001_create_base_tables.down.sql
│       ├── 000010_create_users.up.sql
│       ├── 000010_create_users.down.sql
│       ├── 000020_create_inventory.up.sql
│       ├── 000020_create_inventory.down.sql
│       ├── 000030_create_staff.up.sql
│       └── 000030_create_staff.down.sql
```

### Migration Runner

```go
// pkg/migrate/runner.go

package migrate

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Migrator struct {
    pool           *pgxpool.Pool
    publicMigDir   string
    tenantMigDir   string
}

// RunPublicMigrations runs migrations on public schema (once)
func (m *Migrator) RunPublicMigrations(ctx context.Context) error {
    return m.runMigrations(ctx, m.publicMigDir, "public")
}

// RunForSchema runs tenant migrations on a specific schema
func (m *Migrator) RunForSchema(ctx context.Context, schemaName string) error {
    return m.runMigrations(ctx, m.tenantMigDir, schemaName)
}

// RunForAllTenants runs tenant migrations on all active tenants
func (m *Migrator) RunForAllTenants(ctx context.Context) error {
    rows, err := m.pool.Query(ctx, `
        SELECT schema_name FROM public.tenants
        WHERE deleted_at IS NULL AND subscription_status IN ('active', 'trial')
    `)
    if err != nil {
        return err
    }
    defer rows.Close()

    var schemas []string
    for rows.Next() {
        var schema string
        rows.Scan(&schema)
        schemas = append(schemas, schema)
    }

    for _, schema := range schemas {
        fmt.Printf("Migrating schema: %s\n", schema)
        if err := m.RunForSchema(ctx, schema); err != nil {
            return fmt.Errorf("failed to migrate %s: %w", schema, err)
        }
    }

    return nil
}

func (m *Migrator) runMigrations(ctx context.Context, dir, schemaName string) error {
    // Set search_path
    _, err := m.pool.Exec(ctx, fmt.Sprintf("SET search_path TO %s", schemaName))
    if err != nil {
        return err
    }

    // Create migrations tracking table if not exists
    _, err = m.pool.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version VARCHAR(255) PRIMARY KEY,
            applied_at TIMESTAMPTZ DEFAULT NOW()
        )
    `)
    if err != nil {
        return err
    }

    // Get applied migrations
    applied := make(map[string]bool)
    rows, _ := m.pool.Query(ctx, "SELECT version FROM schema_migrations")
    for rows.Next() {
        var v string
        rows.Scan(&v)
        applied[v] = true
    }
    rows.Close()

    // Get migration files
    files, _ := filepath.Glob(filepath.Join(dir, "*.up.sql"))
    sort.Strings(files)

    // Run pending migrations
    for _, file := range files {
        version := strings.TrimSuffix(filepath.Base(file), ".up.sql")
        if applied[version] {
            continue
        }

        content, err := os.ReadFile(file)
        if err != nil {
            return err
        }

        _, err = m.pool.Exec(ctx, string(content))
        if err != nil {
            return fmt.Errorf("migration %s failed: %w", version, err)
        }

        _, err = m.pool.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version)
        if err != nil {
            return err
        }

        fmt.Printf("  Applied: %s\n", version)
    }

    return nil
}
```

### CLI Commands

```bash
# Run public schema migrations (once)
./medflow-migrate public

# Run migrations for all tenants
./medflow-migrate tenants

# Run migrations for specific tenant
./medflow-migrate tenant --schema=tenant_praxis_mueller

# Create new tenant with migrations
./medflow-admin tenant create --slug=praxis-mueller --name="Zahnarztpraxis Dr. Müller" --tier=standard
```

---

## Analytics Pipeline

### Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Aurora DB      │────▶│  AWS DMS        │────▶│  S3 Data Lake   │
│  (All Schemas)  │     │  (CDC + ETL)    │     │  (Parquet)      │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                                                        │
                              ┌──────────────────────────┘
                              │
                              ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  AWS Glue       │────▶│  Amazon Redshift│────▶│  QuickSight     │
│  (Anonymize)    │     │  (Analytics)    │     │  (Dashboards)   │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

### Anonymization Rules

```python
# glue/anonymize_pii.py

def anonymize_record(record, table_name):
    """Remove/hash PII before loading to analytics warehouse"""

    pii_fields = {
        'users': ['email', 'first_name', 'last_name', 'phone'],
        'staff': ['first_name', 'last_name', 'email', 'phone', 'address'],
        'patients': ['name', 'email', 'phone', 'insurance_number'],
    }

    if table_name in pii_fields:
        for field in pii_fields[table_name]:
            if field in record:
                # Hash for joining, not for identification
                record[field] = hash_field(record[field])

    # Always remove these
    for field in ['password_hash', 'api_key', 'secret']:
        record.pop(field, None)

    return record
```

### Cross-Tenant Aggregations

Analytics warehouse can safely aggregate across tenants:

```sql
-- Example: Average inventory turnover across all practices
SELECT
    DATE_TRUNC('month', adjustment_date) as month,
    COUNT(DISTINCT tenant_schema) as active_practices,
    AVG(total_adjustments) as avg_adjustments
FROM analytics.inventory_summary
GROUP BY 1
ORDER BY 1 DESC;
```

---

## Security & Compliance

### German Healthcare Requirements

| Requirement | Implementation |
|-------------|----------------|
| **GDPR Art. 17** (Right to Erasure) | `DROP SCHEMA CASCADE` + crypto-shredding |
| **GDPR Art. 20** (Data Portability) | Export entire schema to SQL/JSON |
| **Data Sovereignty** | Aurora in eu-central-1 only |
| **AVV** (Auftragsverarbeitungsvertrag) | Schema isolation simplifies DPA |
| **Audit Trail** | `public.tenant_audit_log` + per-tenant audit tables |

### Security Checklist

- [ ] Aurora encryption at rest enabled (KMS)
- [ ] TLS in transit for all connections
- [ ] VPC with private subnets for database
- [ ] Security groups restrict access to EKS only
- [ ] No public endpoints for database
- [ ] IAM roles for service access (not credentials)
- [ ] Secrets Manager for database passwords
- [ ] CloudTrail logging for AWS API calls
- [ ] Per-tenant KMS keys for premium tier

### Data Export (GDPR Art. 20)

```go
// ExportTenantData exports all data for a tenant (portability request)
func (s *TenantService) ExportTenantData(ctx context.Context, tenantID uuid.UUID) (string, error) {
    tenant, _ := s.getTenant(ctx, tenantID)

    // pg_dump the entire schema
    cmd := exec.CommandContext(ctx, "pg_dump",
        "-h", s.dbHost,
        "-U", s.dbUser,
        "-d", s.dbName,
        "-n", tenant.SchemaName,
        "-F", "c",  // Custom format for restore
        "-f", fmt.Sprintf("/exports/%s.dump", tenant.SchemaName),
    )

    if err := cmd.Run(); err != nil {
        return "", err
    }

    // Upload to S3 with presigned URL
    url, err := s.s3.PresignedURL(ctx, fmt.Sprintf("exports/%s.dump", tenant.SchemaName))
    return url, err
}
```

---

## Testing Requirements

### Unit Tests

```go
func TestTenantIsolation(t *testing.T) {
    // Create two test tenants
    tenant1, _ := service.CreateTenant(ctx, CreateTenantInput{Slug: "test-a", Name: "Test A"})
    tenant2, _ := service.CreateTenant(ctx, CreateTenantInput{Slug: "test-b", Name: "Test B"})

    // Insert data into tenant 1
    ctx1 := tenant.WithTenantID(context.Background(), tenant1.ID.String())
    conn1, _ := tcm.AcquireForTenant(ctx1, tenant1.ID.String())
    ctx1 = tenant.WithDB(ctx1, conn1)

    repo.CreateItem(ctx1, &InventoryItem{Name: "Secret Item"})
    conn1.Release()

    // Query from tenant 2 - should NOT see tenant 1's item
    ctx2 := tenant.WithTenantID(context.Background(), tenant2.ID.String())
    conn2, _ := tcm.AcquireForTenant(ctx2, tenant2.ID.String())
    ctx2 = tenant.WithDB(ctx2, conn2)

    items, _ := repo.GetItems(ctx2)
    conn2.Release()

    // Verify isolation
    for _, item := range items {
        assert.NotEqual(t, "Secret Item", item.Name, "Cross-tenant data leak!")
    }
}
```

### Integration Tests

```go
func TestCrossTenantAccessBlocked(t *testing.T) {
    // Attempt to directly query another tenant's schema (should fail)
    conn, _ := pool.Acquire(ctx)
    defer conn.Release()

    // Set search_path to tenant_a
    conn.Exec(ctx, "SET search_path TO tenant_test_a")

    // Try to access tenant_b's table directly (should fail)
    _, err := conn.Query(ctx, "SELECT * FROM tenant_test_b.users")
    assert.Error(t, err, "Should not be able to cross-reference schemas")
}
```

### Load Tests

```go
func BenchmarkConcurrentTenantAccess(b *testing.B) {
    tenants := []string{"tenant_1", "tenant_2", "tenant_3", "tenant_4", "tenant_5"}

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            tenantID := tenants[rand.Intn(len(tenants))]
            ctx := tenant.WithTenantID(context.Background(), tenantID)

            conn, _ := tcm.AcquireForTenant(ctx, tenantID)
            conn.Query(ctx, "SELECT * FROM inventory_items LIMIT 100")
            conn.Release()
        }
    })
}
```

---

## Summary

The Schema-per-Tenant architecture provides:

1. **True Isolation**: Database-engine enforced (not just application code)
2. **GDPR Compliance**: `DROP SCHEMA` = instant, complete data deletion
3. **Customization**: Alter specific tenant's schema without affecting others
4. **German Compliance**: Data stays in Frankfurt, provable separation
5. **Cost Efficiency**: Shared Aurora Serverless compute, scales automatically

All MedFlow microservices must:
- Extract tenant from JWT
- Use `TenantConnectionManager` for connections
- Never hardcode schema names
- Use the migration system for schema changes
- Test cross-tenant isolation
