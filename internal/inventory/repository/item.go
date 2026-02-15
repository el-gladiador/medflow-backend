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
	PZN               *string    `db:"pzn" json:"pzn,omitempty"`
	Manufacturer      *string    `db:"manufacturer" json:"manufacturer,omitempty"`
	Supplier          *string    `db:"supplier" json:"supplier,omitempty"`
	UseBatchTracking  bool       `db:"use_batch_tracking" json:"use_batch_tracking"`
	RequiresCooling   bool       `db:"requires_cooling" json:"requires_cooling"`
	IsHazardous       bool       `db:"is_hazardous" json:"is_hazardous"`
	ShelfLifeDays     *int       `db:"shelf_life_days" json:"shelf_life_days,omitempty"`
	DefaultLocationID *string    `db:"default_location_id" json:"default_location_id,omitempty"`
	IsActive          bool       `db:"is_active" json:"is_active"`
	// Compliance fields
	ManufacturerAddress *string    `db:"manufacturer_address" json:"manufacturer_address,omitempty"`
	CEMarkingNumber     *string    `db:"ce_marking_number" json:"ce_marking_number,omitempty"`
	NotifiedBodyID      *string    `db:"notified_body_id" json:"notified_body_id,omitempty"`
	AcquisitionDate     *time.Time `db:"acquisition_date" json:"acquisition_date,omitempty"`
	SerialNumber        *string    `db:"serial_number" json:"serial_number,omitempty"`
	UdiDI               *string    `db:"udi_di" json:"udi_di,omitempty"`
	UdiPI               *string    `db:"udi_pi" json:"udi_pi,omitempty"`
	// Medical device fields (MPBetreibV §14)
	IsMedicalDevice          bool       `db:"is_medical_device" json:"is_medical_device"`
	DeviceType               *string    `db:"device_type" json:"device_type,omitempty"`
	DeviceModel              *string    `db:"device_model" json:"device_model,omitempty"`
	AuthorizedRepresentative *string    `db:"authorized_representative" json:"authorized_representative,omitempty"`
	Importer                 *string    `db:"importer" json:"importer,omitempty"`
	OperationalIDNumber      *string    `db:"operational_id_number" json:"operational_id_number,omitempty"`
	LocationAssignment       *string    `db:"location_assignment" json:"location_assignment,omitempty"`
	RiskClass                *string    `db:"risk_class" json:"risk_class,omitempty"`
	StkIntervalMonths        *int       `db:"stk_interval_months" json:"stk_interval_months,omitempty"`
	MtkIntervalMonths        *int       `db:"mtk_interval_months" json:"mtk_interval_months,omitempty"`
	LastStkDate              *time.Time `db:"last_stk_date" json:"last_stk_date,omitempty"`
	NextStkDue               *time.Time `db:"next_stk_due" json:"next_stk_due,omitempty"`
	LastMtkDate              *time.Time `db:"last_mtk_date" json:"last_mtk_date,omitempty"`
	NextMtkDue               *time.Time `db:"next_mtk_due" json:"next_mtk_due,omitempty"`
	ShelfLifeAfterOpeningDays *int      `db:"shelf_life_after_opening_days" json:"shelf_life_after_opening_days,omitempty"`
	// Timestamps
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"-"`
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
	// Extract tenant ID from context
	tenantID, err := tenant.TenantID(ctx)
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

	// Execute query with tenant RLS
	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO inventory_items (
				id, tenant_id, name, description, category, unit, unit_price_cents, currency, min_stock,
				max_stock, reorder_point, reorder_quantity, barcode, article_number, pzn, manufacturer,
				supplier, use_batch_tracking, requires_cooling, is_hazardous, shelf_life_days,
				default_location_id, is_active,
				manufacturer_address, ce_marking_number, notified_body_id, acquisition_date,
				serial_number, udi_di, udi_pi,
				is_medical_device, device_type, device_model, authorized_representative, importer,
				operational_id_number, location_assignment, risk_class,
				stk_interval_months, mtk_interval_months, last_stk_date, next_stk_due,
				last_mtk_date, next_mtk_due, shelf_life_after_opening_days
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21,
				$22, $23,
				$24, $25, $26, $27, $28, $29, $30,
				$31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43, $44, $45)
			RETURNING created_at, updated_at
		`

		return r.db.QueryRowxContext(ctx, query,
			item.ID, tenantID, item.Name, item.Description, item.Category, item.Unit, item.UnitPriceCents,
			item.Currency, item.MinStock, item.MaxStock, item.ReorderPoint, item.ReorderQuantity,
			item.Barcode, item.ArticleNumber, item.PZN, item.Manufacturer, item.Supplier, item.UseBatchTracking,
			item.RequiresCooling, item.IsHazardous, item.ShelfLifeDays, item.DefaultLocationID,
			item.IsActive,
			item.ManufacturerAddress, item.CEMarkingNumber, item.NotifiedBodyID, item.AcquisitionDate,
			item.SerialNumber, item.UdiDI, item.UdiPI,
			item.IsMedicalDevice, item.DeviceType, item.DeviceModel, item.AuthorizedRepresentative,
			item.Importer, item.OperationalIDNumber, item.LocationAssignment, item.RiskClass,
			item.StkIntervalMonths, item.MtkIntervalMonths, item.LastStkDate, item.NextStkDue,
			item.LastMtkDate, item.NextMtkDue, item.ShelfLifeAfterOpeningDays,
		).Scan(&item.CreatedAt, &item.UpdatedAt)
	})
}

// GetByID gets an item by ID
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *ItemRepository) GetByID(ctx context.Context, id string) (*InventoryItem, error) {
	// Extract tenant ID from context
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var item InventoryItem

	// Execute query with tenant RLS
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, name, description, category, barcode, article_number, pzn, manufacturer, supplier,
			       unit, min_stock, max_stock, reorder_point, reorder_quantity, use_batch_tracking,
			       requires_cooling, is_hazardous, shelf_life_days, default_location_id,
			       unit_price_cents, currency, is_active,
			       manufacturer_address, ce_marking_number, notified_body_id, acquisition_date,
			       serial_number, udi_di, udi_pi,
			       is_medical_device, device_type, device_model, authorized_representative, importer,
			       operational_id_number, location_assignment, risk_class,
			       stk_interval_months, mtk_interval_months, last_stk_date, next_stk_due,
			       last_mtk_date, next_mtk_due, shelf_life_after_opening_days,
			       created_at, updated_at
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
// TENANT-ISOLATED: Queries only the tenant's schema via RLS
func (r *ItemRepository) GetByBarcode(ctx context.Context, barcode string) (*InventoryItem, error) {
	// Extract tenant ID from context
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var item InventoryItem

	// Execute query with tenant RLS
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, name, description, category, barcode, article_number, pzn, manufacturer, supplier,
			       unit, min_stock, max_stock, reorder_point, reorder_quantity, use_batch_tracking,
			       requires_cooling, is_hazardous, shelf_life_days, default_location_id,
			       unit_price_cents, currency, is_active,
			       manufacturer_address, ce_marking_number, notified_body_id, acquisition_date,
			       serial_number, udi_di, udi_pi,
			       is_medical_device, device_type, device_model, authorized_representative, importer,
			       operational_id_number, location_assignment, risk_class,
			       stk_interval_months, mtk_interval_months, last_stk_date, next_stk_due,
			       last_mtk_date, next_mtk_due, shelf_life_after_opening_days,
			       created_at, updated_at
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

// GetByArticleNumber gets an item by article number
// TENANT-ISOLATED: Queries only the tenant's schema via RLS
func (r *ItemRepository) GetByArticleNumber(ctx context.Context, articleNumber string) (*InventoryItem, error) {
	// Extract tenant ID from context
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var item InventoryItem

	// Execute query with tenant RLS
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, name, description, category, barcode, article_number, pzn, manufacturer, supplier,
			       unit, min_stock, max_stock, reorder_point, reorder_quantity, use_batch_tracking,
			       requires_cooling, is_hazardous, shelf_life_days, default_location_id,
			       unit_price_cents, currency, is_active,
			       manufacturer_address, ce_marking_number, notified_body_id, acquisition_date,
			       serial_number, udi_di, udi_pi,
			       is_medical_device, device_type, device_model, authorized_representative, importer,
			       operational_id_number, location_assignment, risk_class,
			       stk_interval_months, mtk_interval_months, last_stk_date, next_stk_due,
			       last_mtk_date, next_mtk_due, shelf_life_after_opening_days,
			       created_at, updated_at
			FROM inventory_items WHERE article_number = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &item, query, articleNumber)
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
	// Extract tenant ID from context
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err // Fail-fast if tenant context missing
	}

	var total int64
	var items []*InventoryItem

	// Execute queries with tenant RLS
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
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
			SELECT id, name, description, category, barcode, article_number, pzn, manufacturer, supplier,
			       unit, min_stock, max_stock, reorder_point, reorder_quantity, use_batch_tracking,
			       requires_cooling, is_hazardous, shelf_life_days, default_location_id,
			       unit_price_cents, currency, is_active,
			       manufacturer_address, ce_marking_number, notified_body_id, acquisition_date,
			       serial_number, udi_di, udi_pi,
			       is_medical_device, device_type, device_model, authorized_representative, importer,
			       operational_id_number, location_assignment, risk_class,
			       stk_interval_months, mtk_interval_months, last_stk_date, next_stk_due,
			       last_mtk_date, next_mtk_due, shelf_life_after_opening_days,
			       created_at, updated_at
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
	// Extract tenant ID from context
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	// Execute query with tenant RLS
	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE inventory_items SET
				name = $2, description = $3, category = $4, unit = $5, unit_price_cents = $6,
				currency = $7, min_stock = $8, max_stock = $9, reorder_point = $10,
				reorder_quantity = $11, barcode = $12, article_number = $13, pzn = $14, manufacturer = $15,
				supplier = $16, use_batch_tracking = $17, requires_cooling = $18, is_hazardous = $19,
				shelf_life_days = $20, default_location_id = $21, is_active = $22,
				manufacturer_address = $23, ce_marking_number = $24, notified_body_id = $25,
				acquisition_date = $26, serial_number = $27, udi_di = $28, udi_pi = $29,
				is_medical_device = $30, device_type = $31, device_model = $32,
				authorized_representative = $33, importer = $34, operational_id_number = $35,
				location_assignment = $36, risk_class = $37,
				stk_interval_months = $38, mtk_interval_months = $39,
				last_stk_date = $40, next_stk_due = $41, last_mtk_date = $42, next_mtk_due = $43,
				shelf_life_after_opening_days = $44,
				updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`

		result, err := r.db.ExecContext(ctx, query,
			item.ID, item.Name, item.Description, item.Category, item.Unit, item.UnitPriceCents,
			item.Currency, item.MinStock, item.MaxStock, item.ReorderPoint, item.ReorderQuantity,
			item.Barcode, item.ArticleNumber, item.PZN, item.Manufacturer, item.Supplier, item.UseBatchTracking,
			item.RequiresCooling, item.IsHazardous, item.ShelfLifeDays, item.DefaultLocationID,
			item.IsActive,
			item.ManufacturerAddress, item.CEMarkingNumber, item.NotifiedBodyID, item.AcquisitionDate,
			item.SerialNumber, item.UdiDI, item.UdiPI,
			item.IsMedicalDevice, item.DeviceType, item.DeviceModel,
			item.AuthorizedRepresentative, item.Importer, item.OperationalIDNumber,
			item.LocationAssignment, item.RiskClass,
			item.StkIntervalMonths, item.MtkIntervalMonths,
			item.LastStkDate, item.NextStkDue, item.LastMtkDate, item.NextMtkDue,
			item.ShelfLifeAfterOpeningDays,
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
	// Extract tenant ID from context
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	// Execute query with tenant RLS
	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
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
	// Extract tenant ID from context
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var items []*InventoryItem

	// Execute query with tenant RLS
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, name, description, category, barcode, article_number, pzn, manufacturer, supplier,
			       unit, min_stock, max_stock, reorder_point, reorder_quantity, use_batch_tracking,
			       requires_cooling, is_hazardous, shelf_life_days, default_location_id,
			       unit_price_cents, currency, is_active,
			       manufacturer_address, ce_marking_number, notified_body_id, acquisition_date,
			       serial_number, udi_di, udi_pi,
			       is_medical_device, device_type, device_model, authorized_representative, importer,
			       operational_id_number, location_assignment, risk_class,
			       stk_interval_months, mtk_interval_months, last_stk_date, next_stk_due,
			       last_mtk_date, next_mtk_due, shelf_life_after_opening_days,
			       created_at, updated_at
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

// GetByPZN gets an item by PZN (Pharmazentralnummer)
// TENANT-ISOLATED: Queries only the tenant's schema via RLS
func (r *ItemRepository) GetByPZN(ctx context.Context, pzn string) (*InventoryItem, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var item InventoryItem

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, name, description, category, barcode, article_number, pzn, manufacturer, supplier,
			       unit, min_stock, max_stock, reorder_point, reorder_quantity, use_batch_tracking,
			       requires_cooling, is_hazardous, shelf_life_days, default_location_id,
			       unit_price_cents, currency, is_active,
			       manufacturer_address, ce_marking_number, notified_body_id, acquisition_date,
			       serial_number, udi_di, udi_pi,
			       is_medical_device, device_type, device_model, authorized_representative, importer,
			       operational_id_number, location_assignment, risk_class,
			       stk_interval_months, mtk_interval_months, last_stk_date, next_stk_due,
			       last_mtk_date, next_mtk_due, shelf_life_after_opening_days,
			       created_at, updated_at
			FROM inventory_items WHERE pzn = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &item, query, pzn)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("item")
	}
	if err != nil {
		return nil, err
	}

	item.PricePerUnit = float64(item.UnitPriceCents) / 100.0

	return &item, nil
}

// ListMedicalDevices gets all active medical devices (for Bestandsverzeichnis MPBetreibV §14)
// TENANT-ISOLATED: Returns only medical devices from the tenant's schema
func (r *ItemRepository) ListMedicalDevices(ctx context.Context) ([]*InventoryItem, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var items []*InventoryItem
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
		SELECT id, name, description, category, barcode, article_number, pzn, manufacturer, supplier,
			       unit, min_stock, max_stock, reorder_point, reorder_quantity, use_batch_tracking,
			       requires_cooling, is_hazardous, shelf_life_days, default_location_id,
			       unit_price_cents, currency, is_active,
			       manufacturer_address, ce_marking_number, notified_body_id, acquisition_date,
			       serial_number, udi_di, udi_pi,
			       is_medical_device, device_type, device_model, authorized_representative, importer,
			       operational_id_number, location_assignment, risk_class,
			       stk_interval_months, mtk_interval_months, last_stk_date, next_stk_due,
			       last_mtk_date, next_mtk_due, shelf_life_after_opening_days,
			       created_at, updated_at
			FROM inventory_items
			WHERE is_medical_device = TRUE AND deleted_at IS NULL AND is_active = TRUE
			ORDER BY name
		`
		if err := r.db.SelectContext(ctx, &items, query); err != nil {
			return err
		}

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
