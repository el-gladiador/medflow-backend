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

// InventoryItem represents an inventory item
// Actual DB schema: name, description, category, barcode, article_number, manufacturer, supplier,
// unit, min_stock, max_stock, reorder_point, reorder_quantity, use_batch_tracking, requires_cooling,
// is_hazardous, shelf_life_days, default_location_id, unit_price_cents, currency, is_active
type InventoryItem struct {
	ID                string     `db:"id" json:"id"`
	Name              string     `db:"name" json:"name"`
	Description       *string    `db:"description" json:"description,omitempty"`
	Category          string     `db:"category" json:"category"`
	Unit              string     `db:"unit" json:"unit"`
	UnitPriceCents    int        `db:"unit_price_cents" json:"unit_price_cents"`
	Currency          string     `db:"currency" json:"currency"`
	MinStock          int        `db:"min_stock" json:"min_stock"`
	MaxStock          *int       `db:"max_stock" json:"max_stock,omitempty"`
	ReorderPoint      *int       `db:"reorder_point" json:"reorder_point,omitempty"`
	ReorderQuantity   *int       `db:"reorder_quantity" json:"reorder_quantity,omitempty"`
	Barcode           *string    `db:"barcode" json:"barcode,omitempty"`
	ArticleNumber     *string    `db:"article_number" json:"article_number,omitempty"`
	Manufacturer      *string    `db:"manufacturer" json:"manufacturer,omitempty"`
	Supplier          *string    `db:"supplier" json:"supplier,omitempty"`
	UseBatchTracking  bool       `db:"use_batch_tracking" json:"use_batch_tracking"`
	RequiresCooling   bool       `db:"requires_cooling" json:"requires_cooling"`
	IsHazardous       bool       `db:"is_hazardous" json:"is_hazardous"`
	ShelfLifeDays     *int       `db:"shelf_life_days" json:"shelf_life_days,omitempty"`
	DefaultLocationID *string    `db:"default_location_id" json:"default_location_id,omitempty"`
	IsActive          bool       `db:"is_active" json:"is_active"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt         *time.Time `db:"deleted_at" json:"-"`
	// Computed field for API compatibility
	PricePerUnit float64 `db:"-" json:"price_per_unit"`
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
// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *ItemRepository) Create(ctx context.Context, item *InventoryItem) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	if item.ID == "" {
		item.ID = uuid.New().String()
	}

	// Set default currency if not set
	if item.Currency == "" {
		item.Currency = "EUR"
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO inventory_items (
				id, name, description, category, unit, unit_price_cents, currency, min_stock,
				max_stock, reorder_point, reorder_quantity, barcode, article_number, manufacturer,
				supplier, use_batch_tracking, requires_cooling, is_hazardous, shelf_life_days,
				default_location_id, is_active
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
			RETURNING created_at, updated_at
		`

		return r.db.QueryRowxContext(ctx, query,
			item.ID, item.Name, item.Description, item.Category, item.Unit, item.UnitPriceCents,
			item.Currency, item.MinStock, item.MaxStock, item.ReorderPoint, item.ReorderQuantity,
			item.Barcode, item.ArticleNumber, item.Manufacturer, item.Supplier, item.UseBatchTracking,
			item.RequiresCooling, item.IsHazardous, item.ShelfLifeDays, item.DefaultLocationID,
			item.IsActive,
		).Scan(&item.CreatedAt, &item.UpdatedAt)
	})
}

// GetByID gets an item by ID
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *ItemRepository) GetByID(ctx context.Context, id string) (*InventoryItem, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var item InventoryItem

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, name, description, category, barcode, article_number, manufacturer, supplier,
			       unit, min_stock, max_stock, reorder_point, reorder_quantity, use_batch_tracking,
			       requires_cooling, is_hazardous, shelf_life_days, default_location_id,
			       unit_price_cents, currency, is_active, created_at, updated_at
			FROM inventory_items WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &item, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("item")
	}
	if err != nil {
		return nil, err
	}

	// Compute price_per_unit from cents
	item.PricePerUnit = float64(item.UnitPriceCents) / 100.0

	return &item, nil
}

// GetByBarcode gets an item by barcode
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *ItemRepository) GetByBarcode(ctx context.Context, barcode string) (*InventoryItem, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var item InventoryItem

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, name, description, category, barcode, article_number, manufacturer, supplier,
			       unit, min_stock, max_stock, reorder_point, reorder_quantity, use_batch_tracking,
			       requires_cooling, is_hazardous, shelf_life_days, default_location_id,
			       unit_price_cents, currency, is_active, created_at, updated_at
			FROM inventory_items WHERE barcode = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &item, query, barcode)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("item")
	}
	if err != nil {
		return nil, err
	}

	// Compute price_per_unit from cents
	item.PricePerUnit = float64(item.UnitPriceCents) / 100.0

	return &item, nil
}

// List lists inventory items with pagination
// TENANT-ISOLATED: Returns only items from the tenant's schema
func (r *ItemRepository) List(ctx context.Context, page, perPage int, category string) ([]*InventoryItem, int64, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, 0, err // Fail-fast if tenant context missing
	}

	var total int64
	var items []*InventoryItem

	// Execute queries with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Count total items
		countQuery := `SELECT COUNT(*) FROM inventory_items WHERE deleted_at IS NULL`
		args := []interface{}{}

		if category != "" {
			countQuery += ` AND category = $1`
			args = append(args, category)
		}

		if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
			return err
		}

		// Get paginated items
		offset := (page - 1) * perPage
		query := `
			SELECT id, name, description, category, barcode, article_number, manufacturer, supplier,
			       unit, min_stock, max_stock, reorder_point, reorder_quantity, use_batch_tracking,
			       requires_cooling, is_hazardous, shelf_life_days, default_location_id,
			       unit_price_cents, currency, is_active, created_at, updated_at
			FROM inventory_items WHERE deleted_at IS NULL
		`

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

		if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
			return err
		}

		// Compute price_per_unit from cents for each item
		for _, item := range items {
			item.PricePerUnit = float64(item.UnitPriceCents) / 100.0
		}

		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Update updates an inventory item
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *ItemRepository) Update(ctx context.Context, item *InventoryItem) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE inventory_items SET
				name = $2, description = $3, category = $4, unit = $5, unit_price_cents = $6,
				currency = $7, min_stock = $8, max_stock = $9, reorder_point = $10,
				reorder_quantity = $11, barcode = $12, article_number = $13, manufacturer = $14,
				supplier = $15, use_batch_tracking = $16, requires_cooling = $17, is_hazardous = $18,
				shelf_life_days = $19, default_location_id = $20, is_active = $21, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`

		result, err := r.db.ExecContext(ctx, query,
			item.ID, item.Name, item.Description, item.Category, item.Unit, item.UnitPriceCents,
			item.Currency, item.MinStock, item.MaxStock, item.ReorderPoint, item.ReorderQuantity,
			item.Barcode, item.ArticleNumber, item.Manufacturer, item.Supplier, item.UseBatchTracking,
			item.RequiresCooling, item.IsHazardous, item.ShelfLifeDays, item.DefaultLocationID,
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
	})
}

// SoftDelete soft deletes an item
// TENANT-ISOLATED: Soft deletes only in the tenant's schema
func (r *ItemRepository) SoftDelete(ctx context.Context, id string) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
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
	})
}

// GetAllActive gets all active items
// TENANT-ISOLATED: Returns only active items from the tenant's schema
func (r *ItemRepository) GetAllActive(ctx context.Context) ([]*InventoryItem, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var items []*InventoryItem

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, name, description, category, barcode, article_number, manufacturer, supplier,
			       unit, min_stock, max_stock, reorder_point, reorder_quantity, use_batch_tracking,
			       requires_cooling, is_hazardous, shelf_life_days, default_location_id,
			       unit_price_cents, currency, is_active, created_at, updated_at
			FROM inventory_items WHERE deleted_at IS NULL AND is_active = true ORDER BY name
		`
		if err := r.db.SelectContext(ctx, &items, query); err != nil {
			return err
		}

		// Compute price_per_unit from cents for each item
		for _, item := range items {
			item.PricePerUnit = float64(item.UnitPriceCents) / 100.0
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return items, nil
}
