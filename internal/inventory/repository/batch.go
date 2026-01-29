package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
)

// InventoryBatch represents an inventory batch
type InventoryBatch struct {
	ID           string     `db:"id" json:"id"`
	ItemID       string     `db:"item_id" json:"item_id"`
	BatchNumber  string     `db:"batch_number" json:"batch_number"`
	ExpiryDate   time.Time  `db:"expiry_date" json:"expiry_date"`
	Quantity     int        `db:"quantity" json:"quantity"`
	ReceivedDate time.Time  `db:"received_date" json:"received_date"`
	RoomID       *string    `db:"room_id" json:"room_id,omitempty"`
	CabinetID    *string    `db:"cabinet_id" json:"cabinet_id,omitempty"`
	ShelfID      *string    `db:"shelf_id" json:"shelf_id,omitempty"`
	Notes        *string    `db:"notes" json:"notes,omitempty"`
	IsActive     bool       `db:"is_active" json:"is_active"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
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
func (r *BatchRepository) Create(ctx context.Context, batch *InventoryBatch) error {
	if batch.ID == "" {
		batch.ID = uuid.New().String()
	}

	query := `
		INSERT INTO inventory_batches (
			id, item_id, batch_number, expiry_date, quantity, received_date,
			room_id, cabinet_id, shelf_id, notes, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at
	`

	return r.db.QueryRowxContext(ctx, query,
		batch.ID, batch.ItemID, batch.BatchNumber, batch.ExpiryDate, batch.Quantity,
		batch.ReceivedDate, batch.RoomID, batch.CabinetID, batch.ShelfID,
		batch.Notes, batch.IsActive,
	).Scan(&batch.CreatedAt, &batch.UpdatedAt)
}

// GetByID gets a batch by ID
func (r *BatchRepository) GetByID(ctx context.Context, id string) (*InventoryBatch, error) {
	var batch InventoryBatch
	query := `SELECT * FROM inventory_batches WHERE id = $1`
	if err := r.db.GetContext(ctx, &batch, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("batch")
		}
		return nil, err
	}
	return &batch, nil
}

// ListByItem lists batches for an item
func (r *BatchRepository) ListByItem(ctx context.Context, itemID string) ([]*InventoryBatch, error) {
	var batches []*InventoryBatch
	query := `
		SELECT * FROM inventory_batches
		WHERE item_id = $1 AND is_active = true
		ORDER BY expiry_date
	`
	if err := r.db.SelectContext(ctx, &batches, query, itemID); err != nil {
		return nil, err
	}
	return batches, nil
}

// Update updates a batch
func (r *BatchRepository) Update(ctx context.Context, batch *InventoryBatch) error {
	query := `
		UPDATE inventory_batches SET
			batch_number = $2, expiry_date = $3, quantity = $4, received_date = $5,
			room_id = $6, cabinet_id = $7, shelf_id = $8, notes = $9, is_active = $10
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		batch.ID, batch.BatchNumber, batch.ExpiryDate, batch.Quantity,
		batch.ReceivedDate, batch.RoomID, batch.CabinetID, batch.ShelfID,
		batch.Notes, batch.IsActive,
	)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("batch")
	}

	return nil
}

// Delete deletes a batch
func (r *BatchRepository) Delete(ctx context.Context, id string) error {
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
}

// GetTotalStock gets the total stock for an item
func (r *BatchRepository) GetTotalStock(ctx context.Context, itemID string) (int, error) {
	var total sql.NullInt64
	query := `SELECT SUM(quantity) FROM inventory_batches WHERE item_id = $1 AND is_active = true AND quantity > 0`
	if err := r.db.GetContext(ctx, &total, query, itemID); err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	return int(total.Int64), nil
}

// GetExpiringBatches gets batches expiring within days
func (r *BatchRepository) GetExpiringBatches(ctx context.Context, withinDays int) ([]*InventoryBatch, error) {
	var batches []*InventoryBatch
	query := `
		SELECT * FROM inventory_batches
		WHERE is_active = true AND quantity > 0
		AND expiry_date <= NOW() + INTERVAL '1 day' * $1
		ORDER BY expiry_date
	`
	if err := r.db.SelectContext(ctx, &batches, query, withinDays); err != nil {
		return nil, err
	}
	return batches, nil
}

// GetExpiredBatches gets expired batches
func (r *BatchRepository) GetExpiredBatches(ctx context.Context) ([]*InventoryBatch, error) {
	var batches []*InventoryBatch
	query := `
		SELECT * FROM inventory_batches
		WHERE is_active = true AND quantity > 0 AND expiry_date < NOW()
		ORDER BY expiry_date
	`
	if err := r.db.SelectContext(ctx, &batches, query); err != nil {
		return nil, err
	}
	return batches, nil
}

// AdjustStock adjusts the stock for a batch
func (r *BatchRepository) AdjustStock(ctx context.Context, adj *StockAdjustment) error {
	if adj.ID == "" {
		adj.ID = uuid.New().String()
	}

	// Update batch quantity
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

	if adj.BatchID != nil {
		query := `UPDATE inventory_batches SET quantity = $2 WHERE id = $1`
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
}

// GetAllActiveBatches gets all active batches
func (r *BatchRepository) GetAllActiveBatches(ctx context.Context) ([]*InventoryBatch, error) {
	var batches []*InventoryBatch
	query := `SELECT * FROM inventory_batches WHERE is_active = true ORDER BY expiry_date`
	if err := r.db.SelectContext(ctx, &batches, query); err != nil {
		return nil, err
	}
	return batches, nil
}
