// Package testutil provides testing utilities for MedFlow backend services.
// It includes testcontainers for PostgreSQL, tenant context helpers,
// mock factories, and common test fixtures.
package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer wraps a testcontainers PostgreSQL instance
type PostgresContainer struct {
	*postgres.PostgresContainer
	DSN string
}

// PostgresContainerConfig configures the test PostgreSQL container
type PostgresContainerConfig struct {
	Database string
	Username string
	Password string
	Image    string // Optional: defaults to postgres:15-alpine
}

// DefaultPostgresConfig returns sensible defaults for test containers
func DefaultPostgresConfig() PostgresContainerConfig {
	return PostgresContainerConfig{
		Database: "medflow_test",
		Username: "test",
		Password: "test",
		Image:    "postgres:15-alpine",
	}
}

// NewPostgresContainer creates a new PostgreSQL test container.
// The container is automatically configured for testing with schema-per-tenant.
//
// Usage:
//
//	func TestMain(m *testing.M) {
//	    ctx := context.Background()
//	    container, err := testutil.NewPostgresContainer(ctx, testutil.DefaultPostgresConfig())
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer container.Terminate(ctx)
//
//	    // Run tests
//	    code := m.Run()
//	    os.Exit(code)
//	}
func NewPostgresContainer(ctx context.Context, cfg PostgresContainerConfig) (*PostgresContainer, error) {
	if cfg.Image == "" {
		cfg.Image = "postgres:15-alpine"
	}
	if cfg.Database == "" {
		cfg.Database = "medflow_test"
	}
	if cfg.Username == "" {
		cfg.Username = "test"
	}
	if cfg.Password == "" {
		cfg.Password = "test"
	}

	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage(cfg.Image),
		postgres.WithDatabase(cfg.Database),
		postgres.WithUsername(cfg.Username),
		postgres.WithPassword(cfg.Password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	return &PostgresContainer{
		PostgresContainer: container,
		DSN:               dsn,
	}, nil
}

// Connect returns a sqlx.DB connection to the container
func (c *PostgresContainer) Connect(ctx context.Context) (*sqlx.DB, error) {
	db, err := sqlx.ConnectContext(ctx, "postgres", c.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}
	return db, nil
}

// Terminate stops and removes the container
func (c *PostgresContainer) Terminate(ctx context.Context) error {
	return c.PostgresContainer.Terminate(ctx)
}

// CreatePublicSchema creates the public schema with tenants table
// This mirrors the auth service's public schema setup
func (c *PostgresContainer) CreatePublicSchema(ctx context.Context, db *sqlx.DB) error {
	schema := `
		-- Tenants registry (public schema)
		CREATE TABLE IF NOT EXISTS public.tenants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(100) UNIQUE NOT NULL,
			schema_name VARCHAR(100) UNIQUE NOT NULL,
			subscription_status VARCHAR(50) DEFAULT 'trial',
			kms_key_id VARCHAR(255),
			settings JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);

		-- Tenant audit log (public schema)
		CREATE TABLE IF NOT EXISTS public.tenant_audit_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID REFERENCES public.tenants(id),
			action VARCHAR(100) NOT NULL,
			actor_id UUID,
			actor_name VARCHAR(255),
			details JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- Updated at trigger function
		CREATE OR REPLACE FUNCTION update_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`

	_, err := db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to create public schema: %w", err)
	}

	return nil
}
