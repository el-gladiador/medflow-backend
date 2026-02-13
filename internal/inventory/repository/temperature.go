package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// TemperatureReading represents a temperature reading for a cabinet
type TemperatureReading struct {
	ID                 string    `db:"id" json:"id"`
	CabinetID          string    `db:"cabinet_id" json:"cabinet_id"`
	TemperatureCelsius float64   `db:"temperature_celsius" json:"temperature_celsius"`
	RecordedAt         time.Time `db:"recorded_at" json:"recorded_at"`
	RecordedBy         *string   `db:"recorded_by" json:"recorded_by,omitempty"`
	Source             string    `db:"source" json:"source"`
	IsExcursion        bool      `db:"is_excursion" json:"is_excursion"`
	Notes              *string   `db:"notes" json:"notes,omitempty"`
	CreatedAt          time.Time `db:"created_at" json:"created_at"`
}

// TemperatureRepository handles temperature reading persistence
type TemperatureRepository struct {
	db *database.DB
}

// NewTemperatureRepository creates a new temperature repository
func NewTemperatureRepository(db *database.DB) *TemperatureRepository {
	return &TemperatureRepository{db: db}
}

// Create creates a new temperature reading
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *TemperatureRepository) Create(ctx context.Context, reading *TemperatureReading) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if reading.ID == "" {
		reading.ID = uuid.New().String()
	}
	if reading.Source == "" {
		reading.Source = "manual"
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO temperature_readings (
				id, tenant_id, cabinet_id, temperature_celsius, recorded_at, recorded_by,
				source, is_excursion, notes
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING created_at
		`
		return r.db.QueryRowxContext(ctx, query,
			reading.ID, tenantID, reading.CabinetID, reading.TemperatureCelsius,
			reading.RecordedAt, reading.RecordedBy, reading.Source,
			reading.IsExcursion, reading.Notes,
		).Scan(&reading.CreatedAt)
	})
}

// ListByCabinet lists temperature readings for a cabinet with pagination
// TENANT-ISOLATED: Returns only readings via RLS
func (r *TemperatureRepository) ListByCabinet(ctx context.Context, cabinetID string, from, to *time.Time, page, perPage int) ([]*TemperatureReading, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var readings []*TemperatureReading

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		args := []interface{}{cabinetID}
		argIdx := 2

		countQuery := `SELECT COUNT(*) FROM temperature_readings WHERE cabinet_id = $1`
		query := `SELECT id, cabinet_id, temperature_celsius, recorded_at, recorded_by, source, is_excursion, notes, created_at
			FROM temperature_readings WHERE cabinet_id = $1`

		if from != nil {
			countQuery += ` AND recorded_at >= $` + itoa(argIdx)
			query += ` AND recorded_at >= $` + itoa(argIdx)
			args = append(args, *from)
			argIdx++
		}
		if to != nil {
			countQuery += ` AND recorded_at <= $` + itoa(argIdx)
			query += ` AND recorded_at <= $` + itoa(argIdx)
			args = append(args, *to)
			argIdx++
		}

		if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
			return err
		}

		query += ` ORDER BY recorded_at DESC`
		offset := (page - 1) * perPage
		query += ` LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)
		args = append(args, perPage, offset)

		return r.db.SelectContext(ctx, &readings, query, args...)
	})

	if err != nil {
		return nil, 0, err
	}

	return readings, total, nil
}

// GetLatestByCabinet gets the most recent reading for a cabinet
// TENANT-ISOLATED: Queries via RLS
func (r *TemperatureRepository) GetLatestByCabinet(ctx context.Context, cabinetID string) (*TemperatureReading, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var reading TemperatureReading
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, cabinet_id, temperature_celsius, recorded_at, recorded_by, source, is_excursion, notes, created_at
			FROM temperature_readings WHERE cabinet_id = $1
			ORDER BY recorded_at DESC LIMIT 1
		`
		return r.db.GetContext(ctx, &reading, query, cabinetID)
	})

	if err != nil {
		return nil, err
	}

	return &reading, nil
}

// GetExcursions gets excursion readings for a cabinet in a time range
// TENANT-ISOLATED: Returns only readings via RLS
func (r *TemperatureRepository) GetExcursions(ctx context.Context, cabinetID string, from, to time.Time) ([]*TemperatureReading, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var readings []*TemperatureReading
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, cabinet_id, temperature_celsius, recorded_at, recorded_by, source, is_excursion, notes, created_at
			FROM temperature_readings
			WHERE cabinet_id = $1 AND is_excursion = TRUE AND recorded_at >= $2 AND recorded_at <= $3
			ORDER BY recorded_at DESC
		`
		return r.db.SelectContext(ctx, &readings, query, cabinetID, from, to)
	})

	if err != nil {
		return nil, err
	}

	return readings, nil
}

// GetMonitoredCabinetsWithoutReading returns cabinet IDs that have monitoring enabled
// but no reading since the given time
// TENANT-ISOLATED: Returns only cabinets via RLS
func (r *TemperatureRepository) GetMonitoredCabinetsWithoutReading(ctx context.Context, since time.Time) ([]string, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var cabinetIDs []string
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT sc.id FROM storage_cabinets sc
			WHERE sc.temperature_monitoring_enabled = TRUE AND sc.deleted_at IS NULL AND sc.is_active = TRUE
			AND NOT EXISTS (
				SELECT 1 FROM temperature_readings tr
				WHERE tr.cabinet_id = sc.id AND tr.recorded_at >= $1
			)
		`
		return r.db.SelectContext(ctx, &cabinetIDs, query, since)
	})

	if err != nil {
		return nil, err
	}

	return cabinetIDs, nil
}
