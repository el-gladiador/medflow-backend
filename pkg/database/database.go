package database

import (
	"context"
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
	logger *logger.Logger
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
