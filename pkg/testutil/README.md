# MedFlow Test Utilities

This package provides a comprehensive testing infrastructure for MedFlow backend services with full multi-tenancy support.

## Installation

Dependencies are already added to `go.mod`:
- `github.com/stretchr/testify` - Assertions and mocking
- `github.com/testcontainers/testcontainers-go` - PostgreSQL containers
- `github.com/DATA-DOG/go-sqlmock` - SQL mocking for unit tests

## Quick Start

### Integration Tests (Real Database)

```go
package repository_test

import (
    "context"
    "os"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/medflow/medflow-backend/internal/user/repository"
    "github.com/medflow/medflow-backend/pkg/testutil"
)

var suite *testutil.IntegrationSuite

func TestMain(m *testing.M) {
    ctx := context.Background()

    var err error
    suite, err = testutil.NewIntegrationSuite(ctx)
    if err != nil {
        panic(err)
    }

    code := m.Run()

    suite.Cleanup(ctx)
    testutil.TerminateContainer(ctx)
    os.Exit(code)
}

func TestUserRepository_Create(t *testing.T) {
    ctx := context.Background()

    // Each test gets its own isolated tenant schema
    tenant := suite.SetupUserTenant(t, ctx, "test-create")
    tenantCtx := testutil.WithTestTenant(ctx, tenant)

    repo := repository.NewUserRepository(suite.DB)

    // Test with real database, fully isolated
    user, err := repo.GetByID(tenantCtx, "some-id")
    // ...
}
```

### Unit Tests (Mocked Database)

```go
func TestUserRepository_GetByID_Unit(t *testing.T) {
    mockDB := testutil.NewMockDB(t)
    defer mockDB.Close()

    // Set up expectations for tenant-scoped query
    rows := testutil.MockRows("id", "email").
        AddRow("user-123", "test@example.com")

    mockDB.ExpectTenantQuery("tenant_test",
        "SELECT * FROM users WHERE id = $1",
        rows,
    )

    // ... test logic ...

    mockDB.ExpectationsWereMet(t)
}
```

## Package Components

### Container Management (`container.go`)

- `NewPostgresContainer()` - Starts a real PostgreSQL container
- `PostgresContainer.Connect()` - Get database connection
- `PostgresContainer.CreatePublicSchema()` - Set up tenant registry

### Tenant Management (`tenant.go`)

- `TenantManager` - Creates and manages test tenant schemas
- `CreateTenant()` - Create an isolated tenant schema
- `CreateTenantWithMigrations()` - Create tenant with migrations applied
- `WithTestTenant()` - Add tenant context to context.Context
- Pre-built migrations: `UserMigrations()`, `InventoryMigrations()`, `StaffMigrations()`

### Test Fixtures (`fixtures.go`)

- `FixtureFactory` - Generates test data with unique values
- `User()`, `Role()`, `InventoryItem()`, `Employee()` - Create fixtures
- Functional options: `WithEmail()`, `WithName()`, `WithStatus()`, etc.

### Mocking (`mocks.go`)

- `MockDB` - Wraps sqlmock for easier use
- `ExpectTenantQuery()` - Sets up tenant-scoped query expectations
- `ExpectTenantExec()` - Sets up tenant-scoped exec expectations
- `MockPublisher` - Mock event publisher
- `AnyTime`, `AnyUUID` - Argument matchers

### Test Helpers (`helpers.go`)

- `NewHTTPRequest()` - Create HTTP requests for handler tests
- `WithTenantHeaders()` - Add tenant headers to requests
- `ExecuteRequest()` - Execute and record HTTP response
- `AssertStatus()`, `AssertBodyContains()`, `AssertJSONBody()`
- `PtrString()`, `PtrInt()`, `PtrTime()` - Pointer helpers

### Test Suites (`suite.go`)

- `IntegrationSuite` - Full integration test setup
- `UnitTestSuite` - Unit test setup with mocks
- `SetupUserTenant()`, `SetupInventoryTenant()`, `SetupStaffTenant()`

## Testing Multi-Tenancy Isolation

```go
func TestTenantIsolation(t *testing.T) {
    ctx := context.Background()

    // Create two tenants
    tenant1 := suite.SetupUserTenant(t, ctx, "clinic-a")
    tenant2 := suite.SetupUserTenant(t, ctx, "clinic-b")

    repo := repository.NewUserRepository(suite.DB)

    // Create user in tenant1
    ctx1 := testutil.WithTestTenant(ctx, tenant1)
    // ... create user ...

    // Verify tenant2 cannot see tenant1's user
    ctx2 := testutil.WithTestTenant(ctx, tenant2)
    user, err := repo.GetByID(ctx2, userID)
    assert.Error(t, err) // Should not find user
}
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Skip integration tests (runs only unit tests)
go test -short ./...

# Run specific package tests
go test -v ./internal/user/repository/...

# Run with coverage
go test -cover ./...
```

## Best Practices

1. **Use separate tenants per test** - Ensures complete isolation
2. **Use `t.Cleanup()`** - Automatic cleanup after test
3. **Skip integration tests in CI with `-short`** - Faster CI pipelines
4. **Use fixtures for consistent test data** - `suite.Fixtures.User()`
5. **Test tenant isolation explicitly** - Verify cross-tenant access fails
