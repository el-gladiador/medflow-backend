package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
)

// InventoryItem represents an inventory item
type InventoryItem struct {
	ID                string     `db:"id" json:"id"`
	Name              string     `db:"name" json:"name"`
	Category          string     `db:"category" json:"category"`
	Unit              string     `db:"unit" json:"unit"`
	PricePerUnit      float64    `db:"price_per_unit" json:"price_per_unit"`
	MinStock          int        `db:"min_stock" json:"min_stock"`
	Barcode           *string    `db:"barcode" json:"barcode,omitempty"`
	ArticleNumber     *string    `db:"article_number" json:"article_number,omitempty"`
	Supplier          *string    `db:"supplier" json:"supplier,omitempty"`
	UseBatchTracking  bool       `db:"use_batch_tracking" json:"use_batch_tracking"`
	RequiresCooling   bool       `db:"requires_cooling" json:"requires_cooling"`
	DefaultRoomID     *string    `db:"default_room_id" json:"default_room_id,omitempty"`
	DefaultCabinetID  *string    `db:"default_cabinet_id" json:"default_cabinet_id,omitempty"`
	DefaultShelfID    *string    `db:"default_shelf_id" json:"default_shelf_id,omitempty"`
	IsActive          bool       `db:"is_active" json:"is_active"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt         *time.Time `db:"deleted_at" json:"-"`
}

// ItemRepository handles inventory item persistence
type ItemRepository struct {
	db *database.DB
}

// NewItemRepository creates a new item repository
func NewItemRepository(db *database.DB) *ItemRepository {
	return &ItemRepository{db: db}
}

// Create creates a new inventory item
func (r *ItemRepository) Create(ctx context.Context, item *InventoryItem) error {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}

	query := `
		INSERT INTO inventory_items (
			id, name, category, unit, price_per_unit, min_stock, barcode, article_number,
			supplier, use_batch_tracking, requires_cooling, default_room_id, default_cabinet_id,
			default_shelf_id, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING created_at, updated_at
	`

	return r.db.QueryRowxContext(ctx, query,
		item.ID, item.Name, item.Category, item.Unit, item.PricePerUnit, item.MinStock,
		item.Barcode, item.ArticleNumber, item.Supplier, item.UseBatchTracking,
		item.RequiresCooling, item.DefaultRoomID, item.DefaultCabinetID, item.DefaultShelfID,
		item.IsActive,
	).Scan(&item.CreatedAt, &item.UpdatedAt)
}

// GetByID gets an item by ID
func (r *ItemRepository) GetByID(ctx context.Context, id string) (*InventoryItem, error) {
	var item InventoryItem
	query := `SELECT * FROM inventory_items WHERE id = $1 AND deleted_at IS NULL`
	if err := r.db.GetContext(ctx, &item, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("item")
		}
		return nil, err
	}
	return &item, nil
}

// GetByBarcode gets an item by barcode
func (r *ItemRepository) GetByBarcode(ctx context.Context, barcode string) (*InventoryItem, error) {
	var item InventoryItem
	query := `SELECT * FROM inventory_items WHERE barcode = $1 AND deleted_at IS NULL`
	if err := r.db.GetContext(ctx, &item, query, barcode); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("item")
		}
		return nil, err
	}
	return &item, nil
}

// List lists inventory items with pagination
func (r *ItemRepository) List(ctx context.Context, page, perPage int, category string) ([]*InventoryItem, int64, error) {
	var total int64
	countQuery := `SELECT COUNT(*) FROM inventory_items WHERE deleted_at IS NULL`
	args := []interface{}{}

	if category != "" {
		countQuery += ` AND category = $1`
		args = append(args, category)
	}

	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	query := `SELECT * FROM inventory_items WHERE deleted_at IS NULL`

	if category != "" {
		query += ` AND category = $1`
	}

	query += ` ORDER BY name`

	if category != "" {
		query += ` LIMIT $2 OFFSET $3`
		args = append(args, perPage, offset)
	} else {
		query += ` LIMIT $1 OFFSET $2`
		args = append(args, perPage, offset)
	}

	var items []*InventoryItem
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Update updates an inventory item
func (r *ItemRepository) Update(ctx context.Context, item *InventoryItem) error {
	query := `
		UPDATE inventory_items SET
			name = $2, category = $3, unit = $4, price_per_unit = $5, min_stock = $6,
			barcode = $7, article_number = $8, supplier = $9, use_batch_tracking = $10,
			requires_cooling = $11, default_room_id = $12, default_cabinet_id = $13,
			default_shelf_id = $14, is_active = $15
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query,
		item.ID, item.Name, item.Category, item.Unit, item.PricePerUnit, item.MinStock,
		item.Barcode, item.ArticleNumber, item.Supplier, item.UseBatchTracking,
		item.RequiresCooling, item.DefaultRoomID, item.DefaultCabinetID, item.DefaultShelfID,
		item.IsActive,
	)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("item")
	}

	return nil
}

// SoftDelete soft deletes an item
func (r *ItemRepository) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE inventory_items SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("item")
	}

	return nil
}

// GetAllActive gets all active items
func (r *ItemRepository) GetAllActive(ctx context.Context) ([]*InventoryItem, error) {
	var items []*InventoryItem
	query := `SELECT * FROM inventory_items WHERE deleted_at IS NULL AND is_active = true ORDER BY name`
	if err := r.db.SelectContext(ctx, &items, query); err != nil {
		return nil, err
	}
	return items, nil
}
