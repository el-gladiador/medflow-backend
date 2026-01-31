package service

import (
	"context"
	"fmt"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/events"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// InventoryService handles inventory business logic
type InventoryService struct {
	locationRepo *repository.LocationRepository
	itemRepo     *repository.ItemRepository
	batchRepo    *repository.BatchRepository
	alertRepo    *repository.AlertRepository
	publisher    *events.InventoryEventPublisher
	logger       *logger.Logger
}

// NewInventoryService creates a new inventory service
func NewInventoryService(
	locationRepo *repository.LocationRepository,
	itemRepo *repository.ItemRepository,
	batchRepo *repository.BatchRepository,
	alertRepo *repository.AlertRepository,
	publisher *events.InventoryEventPublisher,
	log *logger.Logger,
) *InventoryService {
	return &InventoryService{
		locationRepo: locationRepo,
		itemRepo:     itemRepo,
		batchRepo:    batchRepo,
		alertRepo:    alertRepo,
		publisher:    publisher,
		logger:       log,
	}
}

// ItemWithBatches represents an item with its batches
type ItemWithBatches struct {
	*repository.InventoryItem
	Batches       []*repository.InventoryBatch `json:"batches"`
	TotalStock    int                          `json:"total_stock"`
	Status        string                       `json:"status"`
	NearestExpiry *time.Time                   `json:"nearest_expiry,omitempty"`
	ExpiryStatus  string                       `json:"expiry_status,omitempty"`
}

// DashboardStats represents dashboard statistics
type DashboardStats struct {
	TotalItems        int64  `json:"total_items"`
	TotalStock        int    `json:"total_stock"`
	LowStockCount     int64  `json:"low_stock_count"`
	ExpiringCount     int64  `json:"expiring_count"`
	ExpiredCount      int64  `json:"expired_count"`
	AlertCount        int64  `json:"alert_count"`
	CategoryBreakdown map[string]int64 `json:"category_breakdown"`
}

// Item operations

// CreateItem creates a new inventory item
func (s *InventoryService) CreateItem(ctx context.Context, item *repository.InventoryItem) error {
	return s.itemRepo.Create(ctx, item)
}

// GetItem gets an item with its batches
func (s *InventoryService) GetItem(ctx context.Context, id string) (*ItemWithBatches, error) {
	item, err := s.itemRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	batches, err := s.batchRepo.ListByItem(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.enrichItem(item, batches), nil
}

// ListItems lists items with batches
func (s *InventoryService) ListItems(ctx context.Context, page, perPage int, category string) ([]*ItemWithBatches, int64, error) {
	items, total, err := s.itemRepo.List(ctx, page, perPage, category)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*ItemWithBatches, len(items))
	for i, item := range items {
		batches, _ := s.batchRepo.ListByItem(ctx, item.ID)
		result[i] = s.enrichItem(item, batches)
	}

	return result, total, nil
}

// UpdateItem updates an inventory item
func (s *InventoryService) UpdateItem(ctx context.Context, item *repository.InventoryItem) error {
	return s.itemRepo.Update(ctx, item)
}

// DeleteItem deletes an inventory item
func (s *InventoryService) DeleteItem(ctx context.Context, id string) error {
	return s.itemRepo.SoftDelete(ctx, id)
}

// Batch operations

// CreateBatch creates a new batch
func (s *InventoryService) CreateBatch(ctx context.Context, batch *repository.InventoryBatch) error {
	return s.batchRepo.Create(ctx, batch)
}

// GetBatch gets a batch by ID
func (s *InventoryService) GetBatch(ctx context.Context, id string) (*repository.InventoryBatch, error) {
	return s.batchRepo.GetByID(ctx, id)
}

// ListBatchesByItem lists batches for an item
func (s *InventoryService) ListBatchesByItem(ctx context.Context, itemID string) ([]*repository.InventoryBatch, error) {
	return s.batchRepo.ListByItem(ctx, itemID)
}

// UpdateBatch updates a batch
func (s *InventoryService) UpdateBatch(ctx context.Context, batch *repository.InventoryBatch) error {
	return s.batchRepo.Update(ctx, batch)
}

// DeleteBatch deletes a batch
func (s *InventoryService) DeleteBatch(ctx context.Context, id string) error {
	return s.batchRepo.Delete(ctx, id)
}

// AdjustStock adjusts stock for a batch
func (s *InventoryService) AdjustStock(ctx context.Context, batchID string, adjustment int, adjustmentType, reason, userID, userName string) (*repository.StockAdjustment, error) {
	batch, err := s.batchRepo.GetByID(ctx, batchID)
	if err != nil {
		return nil, err
	}

	adj := &repository.StockAdjustment{
		ItemID:           batch.ItemID,
		BatchID:          &batchID,
		AdjustmentType:   adjustmentType,
		Quantity:         adjustment,
		PreviousQuantity: batch.Quantity,
		Reason:           &reason,
		PerformedBy:      userID,
		PerformedByName:  &userName,
	}

	if err := s.batchRepo.AdjustStock(ctx, adj); err != nil {
		return nil, err
	}

	// Publish event
	s.publisher.PublishStockAdjusted(ctx, adj)

	// Check for low stock alert
	s.checkAndCreateAlerts(ctx, batch.ItemID)

	return adj, nil
}

// Alert generation

func (s *InventoryService) checkAndCreateAlerts(ctx context.Context, itemID string) {
	item, err := s.itemRepo.GetByID(ctx, itemID)
	if err != nil {
		return
	}

	totalStock, err := s.batchRepo.GetTotalStock(ctx, itemID)
	if err != nil {
		return
	}

	// Low stock alert
	if totalStock < item.MinStock {
		severity := "warning"
		alertType := "low_stock"
		if totalStock == 0 {
			severity = "critical"
			alertType = "out_of_stock"
		} else if totalStock < item.MinStock/2 {
			severity = "critical"
		}

		alert := &repository.InventoryAlert{
			AlertType:    alertType,
			ItemID:       item.ID,
			ItemName:     item.Name,
			Severity:     severity,
			Message:      fmt.Sprintf("%s is %s (%d/%d)", item.Name, alertType, totalStock, item.MinStock),
			CurrentStock: &totalStock,
			MinStock:     &item.MinStock,
		}

		s.alertRepo.Create(ctx, alert)
	}
}

// GenerateExpiryAlerts generates alerts for expiring batches
func (s *InventoryService) GenerateExpiryAlerts(ctx context.Context) error {
	// Check batches expiring within 90 days
	batches, err := s.batchRepo.GetExpiringBatches(ctx, 90)
	if err != nil {
		return err
	}

	for _, batch := range batches {
		item, err := s.itemRepo.GetByID(ctx, batch.ItemID)
		if err != nil {
			continue
		}

		if batch.ExpiryDate == nil {
			continue
		}

		daysUntil := int(time.Until(*batch.ExpiryDate).Hours() / 24)

		var alertType, severity string
		if daysUntil < 0 {
			alertType = "expired"
			severity = "critical"
		} else if daysUntil <= 30 {
			alertType = "expiring"
			severity = "critical"
		} else {
			alertType = "expiring_soon"
			severity = "warning"
		}

		alert := &repository.InventoryAlert{
			AlertType:       alertType,
			ItemID:          item.ID,
			ItemName:        item.Name,
			BatchID:         &batch.ID,
			BatchNumber:     &batch.BatchNumber,
			Severity:        severity,
			Message:         fmt.Sprintf("%s batch %s expires in %d days", item.Name, batch.BatchNumber, daysUntil),
			ExpiryDate:      batch.ExpiryDate,
			DaysUntilExpiry: &daysUntil,
		}

		s.alertRepo.Create(ctx, alert)
	}

	return nil
}

// Dashboard

// GetDashboardStats gets dashboard statistics
func (s *InventoryService) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	items, err := s.itemRepo.GetAllActive(ctx)
	if err != nil {
		return nil, err
	}

	batches, err := s.batchRepo.GetAllActiveBatches(ctx)
	if err != nil {
		return nil, err
	}

	alertCount, err := s.alertRepo.GetUnacknowledgedCount(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate stats
	stats := &DashboardStats{
		TotalItems:        int64(len(items)),
		AlertCount:        alertCount,
		CategoryBreakdown: make(map[string]int64),
	}

	// Build batch map for quick lookup
	batchMap := make(map[string][]*repository.InventoryBatch)
	for _, b := range batches {
		batchMap[b.ItemID] = append(batchMap[b.ItemID], b)
	}

	now := time.Now()
	for _, item := range items {
		stats.CategoryBreakdown[item.Category]++

		itemBatches := batchMap[item.ID]
		totalStock := 0
		for _, b := range itemBatches {
			totalStock += b.Quantity

			// Check expiry - only if ExpiryDate is set
			if b.ExpiryDate != nil {
				daysUntil := int(b.ExpiryDate.Sub(now).Hours() / 24)
				if daysUntil < 0 {
					stats.ExpiredCount++
				} else if daysUntil <= 30 {
					stats.ExpiringCount++
				}
			}
		}

		stats.TotalStock += totalStock

		if totalStock < item.MinStock {
			stats.LowStockCount++
		}
	}

	return stats, nil
}

// Helper functions

func (s *InventoryService) enrichItem(item *repository.InventoryItem, batches []*repository.InventoryBatch) *ItemWithBatches {
	result := &ItemWithBatches{
		InventoryItem: item,
		Batches:       batches,
	}

	// Calculate total stock
	for _, b := range batches {
		result.TotalStock += b.Quantity
	}

	// Find nearest expiry
	var nearestExpiry *time.Time
	for _, b := range batches {
		if b.Quantity > 0 && b.ExpiryDate != nil {
			if nearestExpiry == nil || b.ExpiryDate.Before(*nearestExpiry) {
				nearestExpiry = b.ExpiryDate
			}
		}
	}
	result.NearestExpiry = nearestExpiry

	// Calculate expiry status
	if nearestExpiry != nil {
		daysUntil := int(time.Until(*nearestExpiry).Hours() / 24)
		if daysUntil < 0 {
			result.ExpiryStatus = "expired"
		} else if daysUntil <= 30 {
			result.ExpiryStatus = "expiring"
		} else if daysUntil <= 90 {
			result.ExpiryStatus = "expiring_soon"
		} else {
			result.ExpiryStatus = "ok"
		}
	}

	// Calculate status
	if result.TotalStock == 0 {
		result.Status = "Out of Stock"
	} else if result.TotalStock < item.MinStock/2 {
		result.Status = "Critical"
	} else if result.TotalStock < item.MinStock {
		result.Status = "Low Stock"
	} else if result.ExpiryStatus == "expiring" || result.ExpiryStatus == "expired" {
		result.Status = "Expiring"
	} else {
		result.Status = "In Stock"
	}

	return result
}
