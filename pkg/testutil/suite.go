package testutil

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/logger"
)

var (
	// Global test container (shared across all integration tests)
	globalContainer *PostgresContainer
	globalDB        *sqlx.DB
	containerOnce   sync.Once
	containerErr    error
)

// IntegrationSuite provides a base for integration tests with real PostgreSQL
type IntegrationSuite struct {
	Container     *PostgresContainer
	RawDB         *sqlx.DB
	DB            *database.DB
	TenantManager *TenantManager
	Fixtures      *FixtureFactory
	Logger        *logger.Logger
	t             *testing.T
}

// NewIntegrationSuite creates a new integration test suite.
// Call this in TestMain to set up shared test infrastructure.
//
// Usage:
//
//	var suite *testutil.IntegrationSuite
//
//	func TestMain(m *testing.M) {
//	    ctx := context.Background()
//	    var code int
//
//	    suite, err := testutil.NewIntegrationSuite(ctx)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer suite.Cleanup(ctx)
//
//	    code = m.Run()
//	    os.Exit(code)
//	}
//
//	func TestSomething(t *testing.T) {
//	    ctx := context.Background()
//	    tenant := suite.SetupTenant(t, ctx, "test-tenant", testutil.UserMigrations())
//	    // ... run tests with tenant context
//	}
func NewIntegrationSuite(ctx context.Context) (*IntegrationSuite, error) {
	container, db, err := getOrCreateContainer(ctx)
	if err != nil {
		return nil, err
	}

	// Create wrapped database using DSN
	log := logger.New("test", "test")
	wrappedDB, err := database.NewWithDSN(container.DSN, log)
	if err != nil {
		return nil, err
	}

	// Create public schema
	if err := container.CreatePublicSchema(ctx, db); err != nil {
		return nil, err
	}

	return &IntegrationSuite{
		Container:     container,
		RawDB:         db,
		DB:            wrappedDB,
		TenantManager: NewTenantManager(db),
		Fixtures:      NewFixtureFactory(),
		Logger:        log,
	}, nil
}

// getOrCreateContainer returns the shared test container
func getOrCreateContainer(ctx context.Context) (*PostgresContainer, *sqlx.DB, error) {
	containerOnce.Do(func() {
		globalContainer, containerErr = NewPostgresContainer(ctx, DefaultPostgresConfig())
		if containerErr != nil {
			return
		}
		globalDB, containerErr = globalContainer.Connect(ctx)
	})

	return globalContainer, globalDB, containerErr
}

// SetupTenant creates a tenant with migrations for a specific test.
// Each test should use its own tenant for isolation.
func (s *IntegrationSuite) SetupTenant(t *testing.T, ctx context.Context, name string, migrations []string) *TestTenant {
	t.Helper()

	tenant, err := s.TenantManager.CreateTenantWithMigrations(ctx, name, migrations)
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		if err := s.TenantManager.DropTenant(ctx, tenant); err != nil {
			t.Logf("warning: failed to drop tenant %s: %v", tenant.SchemaName, err)
		}
	})

	return tenant
}

// SetupUserTenant creates a tenant with user service migrations
func (s *IntegrationSuite) SetupUserTenant(t *testing.T, ctx context.Context, name string) *TestTenant {
	return s.SetupTenant(t, ctx, name, UserMigrations())
}

// SetupInventoryTenant creates a tenant with inventory service migrations
func (s *IntegrationSuite) SetupInventoryTenant(t *testing.T, ctx context.Context, name string) *TestTenant {
	return s.SetupTenant(t, ctx, name, InventoryMigrations())
}

// SetupStaffTenant creates a tenant with staff service migrations
func (s *IntegrationSuite) SetupStaffTenant(t *testing.T, ctx context.Context, name string) *TestTenant {
	return s.SetupTenant(t, ctx, name, StaffMigrations())
}

// SetupFullTenant creates a tenant with all service migrations
func (s *IntegrationSuite) SetupFullTenant(t *testing.T, ctx context.Context, name string) *TestTenant {
	migrations := make([]string, 0)
	migrations = append(migrations, UserMigrations()...)
	migrations = append(migrations, InventoryMigrations()...)
	migrations = append(migrations, StaffMigrations()...)
	return s.SetupTenant(t, ctx, name, migrations)
}

// TenantContext returns a context with the tenant set
func (s *IntegrationSuite) TenantContext(tenant *TestTenant) context.Context {
	return WithTestTenant(context.Background(), tenant)
}

// Cleanup cleans up all test resources
func (s *IntegrationSuite) Cleanup(ctx context.Context) error {
	if err := s.TenantManager.Cleanup(ctx); err != nil {
		return err
	}
	// Note: We don't terminate the container here since it's shared
	return nil
}

// TerminateContainer terminates the shared container.
// Only call this in TestMain after all tests have completed.
func TerminateContainer(ctx context.Context) {
	if globalContainer != nil {
		globalContainer.Terminate(ctx)
	}
}

// UnitTestSuite provides a base for unit tests with mocked dependencies
type UnitTestSuite struct {
	MockDB   *MockDB
	Fixtures *FixtureFactory
	t        *testing.T
}

// NewUnitTestSuite creates a new unit test suite
func NewUnitTestSuite(t *testing.T) *UnitTestSuite {
	return &UnitTestSuite{
		MockDB:   NewMockDB(t),
		Fixtures: NewFixtureFactory(),
		t:        t,
	}
}

// Cleanup verifies expectations and cleans up
func (s *UnitTestSuite) Cleanup() {
	s.MockDB.ExpectationsWereMet(s.t)
	s.MockDB.Close()
}

// GetEnvOrDefault returns environment variable or default value
func GetEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// IsCI returns true if running in CI environment
func IsCI() bool {
	ciVars := []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL"}
	for _, v := range ciVars {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return false
}
