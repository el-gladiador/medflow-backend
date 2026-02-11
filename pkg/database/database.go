package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/medflow/medflow-backend/pkg/config"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// DB wraps sqlx.DB with additional functionality
type DB struct {
	*sqlx.DB
	logger     *logger.Logger
	searchPath string // Service-specific search_path (e.g., "users, public")
}

// New creates a new database connection
func New(cfg *config.DatabaseConfig, log *logger.Logger) (*DB, error) {
	db, err := sqlx.Connect("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return &DB{
		DB:     db,
		logger: log,
	}, nil
}

// NewWithDSN creates a new database connection with a DSN string
func NewWithDSN(dsn string, log *logger.Logger) (*DB, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &DB{
		DB:     db,
		logger: log,
	}, nil
}

// NewWithDSNAndSearchPath creates a new database connection from a DSN string with a search_path.
// Used for test infrastructure where DSN comes from a test container.
func NewWithDSNAndSearchPath(dsn string, searchPath string, log *logger.Logger) (*DB, error) {
	db, err := NewWithDSN(dsn, log)
	if err != nil {
		return nil, err
	}
	db.searchPath = searchPath
	return db, nil
}

// NewWithSearchPath creates a new database connection with a service-specific search_path.
// Used for RLS-based multi-tenancy where each service operates in its own schema.
//
// Example search paths:
//   - "public" (auth-service)
//   - "users, public" (user-service)
//   - "staff, public" (staff-service)
//   - "inventory, public" (inventory-service)
func NewWithSearchPath(cfg *config.DatabaseConfig, searchPath string, log *logger.Logger) (*DB, error) {
	db, err := New(cfg, log)
	if err != nil {
		return nil, err
	}
	db.searchPath = searchPath
	return db, nil
}

// SearchPath returns the configured search path for this DB instance
func (db *DB) SearchPath() string {
	return db.searchPath
}

// Ping checks the database connection
func (db *DB) Ping(ctx context.Context) error {
	return db.PingContext(ctx)
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// Health returns the health status of the database
func (db *DB) Health(ctx context.Context) map[string]string {
	status := map[string]string{
		"status": "up",
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		status["status"] = "down"
		status["error"] = err.Error()
	}

	return status
}

// Transaction executes a function within a transaction
func (db *DB) Transaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			db.logger.Error().Err(rbErr).Msg("failed to rollback transaction")
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetContext gets a single record, using transaction from context if available
func (db *DB) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	if tx := db.getTx(ctx); tx != nil {
		return tx.GetContext(ctx, dest, query, args...)
	}
	return db.DB.GetContext(ctx, dest, query, args...)
}

// SelectContext gets multiple records, using transaction from context if available
func (db *DB) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	if tx := db.getTx(ctx); tx != nil {
		return tx.SelectContext(ctx, dest, query, args...)
	}
	return db.DB.SelectContext(ctx, dest, query, args...)
}

// QueryRowxContext queries a single row, using transaction from context if available
func (db *DB) QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	if tx := db.getTx(ctx); tx != nil {
		return tx.QueryRowxContext(ctx, query, args...)
	}
	return db.DB.QueryRowxContext(ctx, query, args...)
}

// QueryContext executes a query, using transaction from context if available
func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	if tx := db.getTx(ctx); tx != nil {
		return tx.QueryxContext(ctx, query, args...)
	}
	return db.DB.QueryxContext(ctx, query, args...)
}

// QueryxContext executes a query, using transaction from context if available
func (db *DB) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	if tx := db.getTx(ctx); tx != nil {
		return tx.QueryxContext(ctx, query, args...)
	}
	return db.DB.QueryxContext(ctx, query, args...)
}

// QueryRowContext queries a single row, using transaction from context if available
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	if tx := db.getTx(ctx); tx != nil {
		return tx.QueryRowxContext(ctx, query, args...)
	}
	return db.DB.QueryRowxContext(ctx, query, args...)
}

// ExecContext executes a query, using transaction from context if available
func (db *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if tx := db.getTx(ctx); tx != nil {
		return tx.ExecContext(ctx, query, args...)
	}
	return db.DB.ExecContext(ctx, query, args...)
}
