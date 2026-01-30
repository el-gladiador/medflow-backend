package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// InventoryBatch represents an inventory batch
// Actual DB schema: item_id, location_id, batch_number, lot_number, initial_quantity,
// current_quantity, reserved_quantity, manufactured_date, expiry_date, received_date, status
type InventoryBatch struct {
	ID               string     `db:"id" json:"id"`
	ItemID           string     `db:"item_id" json:"item_id"`
	LocationID       *string    `db:"location_id" json:"location_id,omitempty"`
	BatchNumber      string     `db:"batch_number" json:"batch_number"`
	LotNumber        *string    `db:"lot_number" json:"lot_number,omitempty"`
	InitialQuantity  int        `db:"initial_quantity" json:"initial_quantity"`
	CurrentQuantity  int        `db:"current_quantity" json:"current_quantity"`
	ReservedQuantity int        `db:"reserved_quantity" json:"reserved_quantity"`
	ManufacturedDate *time.Time `db:"manufactured_date" json:"manufactured_date,omitempty"`
	ExpiryDate       time.Time  `db:"expiry_date" json:"expiry_date"`
	ReceivedDate     time.Time  `db:"received_date" json:"received_date"`
	Status           string     `db:"status" json:"status"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
	// Computed field for backwards compatibility
	Quantity int `db:"-" json:"quantity"`
}

// StockAdjustment represents a stock adjustment
type StockAdjustment struct {
	ID               string    `db:"id" json:"id"`
	ItemID           string    `db:"item_id" json:"item_id"`
	BatchID          *string   `db:"batch_id" json:"batch_id,omitempty"`
	AdjustmentType   string    `db:"adjustment_type" json:"adjustment_type"`
	Quantity         int       `db:"quantity" json:"quantity"`
	PreviousQuantity int       `db:"previous_quantity" json:"previous_quantity"`
	NewQuantity      int       `db:"new_quantity" json:"new_quantity"`
	Reason           *string   `db:"reason" json:"reason,omitempty"`
	PerformedBy      string    `db:"performed_by" json:"performed_by"`
	PerformedByName  *string   `db:"performed_by_name" json:"performed_by_name,omitempty"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
}

// BatchRepository handles batch persistence
type BatchRepository struct {
	db *database.DB
}

// NewBatchRepository creates a new batch repository
func NewBatchRepository(db *database.DB) *BatchRepository {
	return &BatchRepository{db: db}
}

// Create creates a new batch
// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *BatchRepository) Create(ctx context.Context, batch *InventoryBatch) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	if batch.ID == "" {
		batch.ID = uuid.New().String()
	}

	// Set default status
	if batch.Status == "" {
		batch.Status = "active"
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO inventory_batches (
				id, item_id, location_id, batch_number, lot_number, initial_quantity,
				current_quantity, reserved_quantity, manufactured_date, expiry_date,
				received_date, status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING created_at, updated_at
		`

		return r.db.QueryRowxContext(ctx, query,
			batch.ID, batch.ItemID, batch.LocationID, batch.BatchNumber, batch.LotNumber,
			batch.InitialQuantity, batch.CurrentQuantity, batch.ReservedQuantity,
			batch.ManufacturedDate, batch.ExpiryDate, batch.ReceivedDate, batch.Status,
		).Scan(&batch.CreatedAt, &batch.UpdatedAt)
	})
}

// GetByID gets a batch by ID
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *BatchRepository) GetByID(ctx context.Context, id string) (*InventoryBatch, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var batch InventoryBatch

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, location_id, batch_number, lot_number, initial_quantity,
			       current_quantity, reserved_quantity, manufactured_date, expiry_date,
			       received_date, status, created_at, updated_at
			FROM inventory_batches WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &batch, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("batch")
	}
	if err != nil {
		return nil, err
	}

	// Set computed quantity field
	batch.Quantity = batch.CurrentQuantity

	return &batch, nil
}

// ListByItem lists batches for an item
// TENANT-ISOLATED: Returns only batches from the tenant's schema
func (r *BatchRepository) ListByItem(ctx context.Context, itemID string) ([]*InventoryBatch, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var batches []*InventoryBatch

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, location_id, batch_number, lot_number, initial_quantity,
			       current_quantity, reserved_quantity, manufactured_date, expiry_date,
			       received_date, status, created_at, updated_at
			FROM inventory_batches
			WHERE item_id = $1 AND status = 'active' AND deleted_at IS NULL
			ORDER BY expiry_date
		`
		if err := r.db.SelectContext(ctx, &batches, query, itemID); err != nil {
			return err
		}

		// Set computed quantity for each batch
		for _, b := range batches {
			b.Quantity = b.CurrentQuantity
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return batches, nil
}

// Update updates a batch
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *BatchRepository) Update(ctx context.Context, batch *InventoryBatch) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE inventory_batches SET
				location_id = $2, batch_number = $3, lot_number = $4, initial_quantity = $5,
				current_quantity = $6, reserved_quantity = $7, manufactured_date = $8,
				expiry_date = $9, received_date = $10, status = $11, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`

		result, err := r.db.ExecContext(ctx, query,
			batch.ID, batch.LocationID, batch.BatchNumber, batch.LotNumber, batch.InitialQuantity,
			batch.CurrentQuantity, batch.ReservedQuantity, batch.ManufacturedDate,
			batch.ExpiryDate, batch.ReceivedDate, batch.Status,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("batch")
		}

		return nil
	})
}

// Delete deletes a batch
// TENANT-ISOLATED: Deletes only from the tenant's schema
func (r *BatchRepository) Delete(ctx context.Context, id string) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `DELETE FROM inventory_batches WHERE id = $1`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("batch")
		}

		return nil
	})
}

// GetTotalStock gets the total stock for an item
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *BatchRepository) GetTotalStock(ctx context.Context, itemID string) (int, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return 0, err // Fail-fast if tenant context missing
	}

	var total sql.NullInt64

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `SELECT SUM(current_quantity) FROM inventory_batches WHERE item_id = $1 AND status = 'active' AND deleted_at IS NULL AND current_quantity > 0`
		return r.db.GetContext(ctx, &total, query, itemID)
	})

	if err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}

	return int(total.Int64), nil
}

// GetExpiringBatches gets batches expiring within days
// TENANT-ISOLATED: Returns only batches from the tenant's schema
func (r *BatchRepository) GetExpiringBatches(ctx context.Context, withinDays int) ([]*InventoryBatch, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var batches []*InventoryBatch

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, location_id, batch_number, lot_number, initial_quantity,
			       current_quantity, reserved_quantity, manufactured_date, expiry_date,
			       received_date, status, created_at, updated_at
			FROM inventory_batches
			WHERE status = 'active' AND deleted_at IS NULL AND current_quantity > 0
			AND expiry_date <= NOW() + INTERVAL '1 day' * $1
			ORDER BY expiry_date
		`
		if err := r.db.SelectContext(ctx, &batches, query, withinDays); err != nil {
			return err
		}

		// Set computed quantity for each batch
		for _, b := range batches {
			b.Quantity = b.CurrentQuantity
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return batches, nil
}

// GetExpiredBatches gets expired batches
// TENANT-ISOLATED: Returns only expired batches from the tenant's schema
func (r *BatchRepository) GetExpiredBatches(ctx context.Context) ([]*InventoryBatch, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var batches []*InventoryBatch

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, location_id, batch_number, lot_number, initial_quantity,
			       current_quantity, reserved_quantity, manufactured_date, expiry_date,
			       received_date, status, created_at, updated_at
			FROM inventory_batches
			WHERE status = 'active' AND deleted_at IS NULL AND current_quantity > 0 AND expiry_date < NOW()
			ORDER BY expiry_date
		`
		if err := r.db.SelectContext(ctx, &batches, query); err != nil {
			return err
		}

		// Set computed quantity for each batch
		for _, b := range batches {
			b.Quantity = b.CurrentQuantity
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return batches, nil
}

// AdjustStock adjusts the stock for a batch
// TENANT-ISOLATED: Updates and inserts only in the tenant's schema
func (r *BatchRepository) AdjustStock(ctx context.Context, adj *StockAdjustment) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	if adj.ID == "" {
		adj.ID = uuid.New().String()
	}

	// Calculate new quantity
	var newQty int
	switch adj.AdjustmentType {
	case "add":
		newQty = adj.PreviousQuantity + adj.Quantity
	case "deduct":
		newQty = adj.PreviousQuantity - adj.Quantity
	case "adjust":
		newQty = adj.Quantity
	}
	adj.NewQuantity = newQty

	// Execute queries with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Update batch current_quantity if batch ID provided
		if adj.BatchID != nil {
			query := `UPDATE inventory_batches SET current_quantity = $2, updated_at = NOW() WHERE id = $1`
			if _, err := r.db.ExecContext(ctx, query, *adj.BatchID, newQty); err != nil {
				return err
			}
		}

		// Create adjustment record
		query := `
			INSERT INTO stock_adjustments (
				id, item_id, batch_id, adjustment_type, quantity, previous_quantity,
				new_quantity, reason, performed_by, performed_by_name
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at
		`

		return r.db.QueryRowxContext(ctx, query,
			adj.ID, adj.ItemID, adj.BatchID, adj.AdjustmentType, adj.Quantity,
			adj.PreviousQuantity, adj.NewQuantity, adj.Reason, adj.PerformedBy, adj.PerformedByName,
		).Scan(&adj.CreatedAt)
	})
}

// GetAllActiveBatches gets all active batches
// TENANT-ISOLATED: Returns only active batches from the tenant's schema
func (r *BatchRepository) GetAllActiveBatches(ctx context.Context) ([]*InventoryBatch, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var batches []*InventoryBatch

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, location_id, batch_number, lot_number, initial_quantity,
			       current_quantity, reserved_quantity, manufactured_date, expiry_date,
			       received_date, status, created_at, updated_at
			FROM inventory_batches WHERE status = 'active' AND deleted_at IS NULL ORDER BY expiry_date
		`
		if err := r.db.SelectContext(ctx, &batches, query); err != nil {
			return err
		}

		// Set computed quantity for each batch
		for _, b := range batches {
			b.Quantity = b.CurrentQuantity
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return batches, nil
}
