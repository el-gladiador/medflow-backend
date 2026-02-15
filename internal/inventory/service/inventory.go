package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/events"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	apperrors "github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// InventoryService handles inventory business logic
type InventoryService struct {
	locationRepo    *repository.LocationRepository
	itemRepo        *repository.ItemRepository
	batchRepo       *repository.BatchRepository
	alertRepo       *repository.AlertRepository
	hazardousRepo   *repository.HazardousRepository
	documentRepo    *repository.DocumentRepository
	temperatureRepo *repository.TemperatureRepository
	inspectionRepo  *repository.InspectionRepository
	trainingRepo    *repository.TrainingRepository
	incidentRepo    *repository.IncidentRepository
	publisher       *events.InventoryEventPublisher
	logger          *logger.Logger
}

// NewInventoryService creates a new inventory service
func NewInventoryService(
	locationRepo *repository.LocationRepository,
	itemRepo *repository.ItemRepository,
	batchRepo *repository.BatchRepository,
	alertRepo *repository.AlertRepository,
	hazardousRepo *repository.HazardousRepository,
	documentRepo *repository.DocumentRepository,
	temperatureRepo *repository.TemperatureRepository,
	inspectionRepo *repository.InspectionRepository,
	trainingRepo *repository.TrainingRepository,
	incidentRepo *repository.IncidentRepository,
	publisher *events.InventoryEventPublisher,
	log *logger.Logger,
) *InventoryService {
	return &InventoryService{
		locationRepo:    locationRepo,
		itemRepo:        itemRepo,
		batchRepo:       batchRepo,
		alertRepo:       alertRepo,
		hazardousRepo:   hazardousRepo,
		documentRepo:    documentRepo,
		temperatureRepo: temperatureRepo,
		inspectionRepo:  inspectionRepo,
		trainingRepo:    trainingRepo,
		incidentRepo:    incidentRepo,
		publisher:       publisher,
		logger:          log,
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

// AdjustStock adjusts stock for a batch.
// The repository layer uses SELECT FOR UPDATE to prevent race conditions
// and rejects deductions that would result in negative stock.
func (s *InventoryService) AdjustStock(ctx context.Context, batchID string, adjustment int, adjustmentType, reason, userID, userName string) (*repository.StockAdjustment, error) {
	if adjustment <= 0 {
		return nil, fmt.Errorf("adjustment quantity must be positive, got %d", adjustment)
	}

	batch, err := s.batchRepo.GetByID(ctx, batchID)
	if err != nil {
		return nil, err
	}

	adj := &repository.StockAdjustment{
		ItemID:         batch.ItemID,
		BatchID:        &batchID,
		AdjustmentType: adjustmentType,
		Quantity:       adjustment,
		// PreviousQuantity is set atomically inside AdjustStock via SELECT FOR UPDATE
		Reason:          &reason,
		PerformedBy:     userID,
		PerformedByName: &userName,
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

// Hazardous substance operations

// GetHazardousDetails gets hazardous substance details for an item
func (s *InventoryService) GetHazardousDetails(ctx context.Context, itemID string) (*repository.HazardousSubstanceDetail, error) {
	return s.hazardousRepo.GetByItemID(ctx, itemID)
}

// UpsertHazardousDetails creates or updates hazardous substance details
func (s *InventoryService) UpsertHazardousDetails(ctx context.Context, detail *repository.HazardousSubstanceDetail) error {
	return s.hazardousRepo.Upsert(ctx, detail)
}

// DeleteHazardousDetails deletes hazardous substance details for an item
func (s *InventoryService) DeleteHazardousDetails(ctx context.Context, itemID string) error {
	return s.hazardousRepo.Delete(ctx, itemID)
}

// Document operations

// ListItemDocuments lists documents for an item
func (s *InventoryService) ListItemDocuments(ctx context.Context, itemID string) ([]*repository.ItemDocument, error) {
	return s.documentRepo.ListByItem(ctx, itemID)
}

// GetItemDocument gets a document by ID
func (s *InventoryService) GetItemDocument(ctx context.Context, id string) (*repository.ItemDocument, error) {
	return s.documentRepo.GetByID(ctx, id)
}

// CreateItemDocument creates a new item document
func (s *InventoryService) CreateItemDocument(ctx context.Context, doc *repository.ItemDocument) error {
	return s.documentRepo.Create(ctx, doc)
}

// DeleteItemDocument deletes an item document
func (s *InventoryService) DeleteItemDocument(ctx context.Context, id string) error {
	return s.documentRepo.Delete(ctx, id)
}

// Export operations

// HazardousItemWithDetails represents an item with its hazardous details for export
type HazardousItemWithDetails struct {
	Item    *repository.InventoryItem            `json:"item"`
	Details *repository.HazardousSubstanceDetail `json:"details"`
}

// ListAllHazardousItems lists all hazardous items with their details
func (s *InventoryService) ListAllHazardousItems(ctx context.Context) ([]*HazardousItemWithDetails, error) {
	items, err := s.itemRepo.GetAllActive(ctx)
	if err != nil {
		return nil, err
	}

	details, err := s.hazardousRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	// Build lookup map
	detailMap := make(map[string]*repository.HazardousSubstanceDetail)
	for _, d := range details {
		detailMap[d.ItemID] = d
	}

	var result []*HazardousItemWithDetails
	for _, item := range items {
		if !item.IsHazardous {
			continue
		}
		result = append(result, &HazardousItemWithDetails{
			Item:    item,
			Details: detailMap[item.ID],
		})
	}

	return result, nil
}

// GetAllActiveItems returns all active items (for inventory register export)
func (s *InventoryService) GetAllActiveItems(ctx context.Context) ([]*repository.InventoryItem, error) {
	return s.itemRepo.GetAllActive(ctx)
}

// ListMedicalDevices returns all active medical devices (for Bestandsverzeichnis)
func (s *InventoryService) ListMedicalDevices(ctx context.Context) ([]*repository.InventoryItem, error) {
	return s.itemRepo.ListMedicalDevices(ctx)
}

// Batch opening operations (AMG)

// OpenBatch marks a batch as opened and sets opened_at timestamp
func (s *InventoryService) OpenBatch(ctx context.Context, batchID string) (*repository.InventoryBatch, error) {
	batch, err := s.batchRepo.GetByID(ctx, batchID)
	if err != nil {
		return nil, err
	}

	if batch.OpenedAt != nil {
		return nil, fmt.Errorf("batch already opened on %s", batch.OpenedAt.Format("2006-01-02"))
	}

	now := time.Now()
	batch.OpenedAt = &now

	if err := s.batchRepo.Update(ctx, batch); err != nil {
		return nil, err
	}

	// Check if effective expiry triggers alerts
	item, err := s.itemRepo.GetByID(ctx, batch.ItemID)
	if err == nil && item.ShelfLifeAfterOpeningDays != nil {
		effectiveExpiry := GetEffectiveExpiry(batch, item)
		if effectiveExpiry != nil {
			daysUntil := int(time.Until(*effectiveExpiry).Hours() / 24)
			if daysUntil <= 7 {
				alertType := "opening_expiry_soon"
				severity := "warning"
				if daysUntil <= 0 {
					alertType = "opening_expired"
					severity = "critical"
				}
				alert := &repository.InventoryAlert{
					AlertType:   alertType,
					ItemID:      item.ID,
					ItemName:    item.Name,
					BatchID:     &batch.ID,
					BatchNumber: &batch.BatchNumber,
					Severity:    severity,
					Message:     fmt.Sprintf("%s batch %s: post-opening expiry in %d days", item.Name, batch.BatchNumber, daysUntil),
				}
				s.alertRepo.Create(ctx, alert)
			}
		}
	}

	return batch, nil
}

// GetEffectiveExpiry returns the earlier of batch expiry and post-opening expiry
func GetEffectiveExpiry(batch *repository.InventoryBatch, item *repository.InventoryItem) *time.Time {
	if batch.OpenedAt == nil || item.ShelfLifeAfterOpeningDays == nil {
		return batch.ExpiryDate
	}

	openingExpiry := batch.OpenedAt.AddDate(0, 0, *item.ShelfLifeAfterOpeningDays)

	if batch.ExpiryDate == nil {
		return &openingExpiry
	}

	if openingExpiry.Before(*batch.ExpiryDate) {
		return &openingExpiry
	}
	return batch.ExpiryDate
}

// Temperature operations

// RecordTemperature records a temperature reading for a cabinet
func (s *InventoryService) RecordTemperature(ctx context.Context, cabinetID string, tempCelsius float64, source string, recordedBy *string, notes *string) (*repository.TemperatureReading, error) {
	// Look up cabinet to check thresholds
	cabinet, err := s.locationRepo.GetCabinet(ctx, cabinetID)
	if err != nil {
		return nil, fmt.Errorf("cabinet not found: %w", err)
	}

	// Determine if this is an excursion
	isExcursion := false
	if cabinet.MinTemperature != nil && tempCelsius < *cabinet.MinTemperature {
		isExcursion = true
	}
	if cabinet.MaxTemperature != nil && tempCelsius > *cabinet.MaxTemperature {
		isExcursion = true
	}

	reading := &repository.TemperatureReading{
		CabinetID:          cabinetID,
		TemperatureCelsius: tempCelsius,
		RecordedAt:         time.Now(),
		RecordedBy:         recordedBy,
		Source:             source,
		IsExcursion:        isExcursion,
		Notes:              notes,
	}

	if err := s.temperatureRepo.Create(ctx, reading); err != nil {
		return nil, err
	}

	// Generate alert if excursion
	if isExcursion {
		alert := &repository.InventoryAlert{
			AlertType: "temperature_excursion",
			ItemID:    cabinetID, // Using cabinet ID as reference
			ItemName:  cabinet.Name,
			Severity:  "critical",
			Message:   fmt.Sprintf("Temperature excursion in %s: %.1f°C (range: %.1f-%.1f°C)", cabinet.Name, tempCelsius, derefFloat(cabinet.MinTemperature), derefFloat(cabinet.MaxTemperature)),
		}
		s.alertRepo.Create(ctx, alert)
	}

	return reading, nil
}

// ListTemperatureReadings lists temperature readings for a cabinet
func (s *InventoryService) ListTemperatureReadings(ctx context.Context, cabinetID string, from, to *time.Time, page, perPage int) ([]*repository.TemperatureReading, int64, error) {
	return s.temperatureRepo.ListByCabinet(ctx, cabinetID, from, to, page, perPage)
}

// CheckDailyTemperatureCompliance checks for monitored cabinets without readings today
func (s *InventoryService) CheckDailyTemperatureCompliance(ctx context.Context) error {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	cabinetIDs, err := s.temperatureRepo.GetMonitoredCabinetsWithoutReading(ctx, startOfDay)
	if err != nil {
		return err
	}

	for _, cabID := range cabinetIDs {
		cabinet, err := s.locationRepo.GetCabinet(ctx, cabID)
		if err != nil {
			continue
		}

		alert := &repository.InventoryAlert{
			AlertType: "temperature_missing",
			ItemID:    cabID,
			ItemName:  cabinet.Name,
			Severity:  "warning",
			Message:   fmt.Sprintf("No temperature reading today for %s", cabinet.Name),
		}
		s.alertRepo.Create(ctx, alert)
	}

	return nil
}

// Device inspection operations (Medizinproduktebuch)

// CreateInspection creates an inspection and updates the item's STK/MTK dates
func (s *InventoryService) CreateInspection(ctx context.Context, insp *repository.DeviceInspection) error {
	if err := s.inspectionRepo.Create(ctx, insp); err != nil {
		return err
	}

	// Update item's last/next STK/MTK dates
	item, err := s.itemRepo.GetByID(ctx, insp.ItemID)
	if err != nil {
		return nil // Inspection created, but item update failed - non-fatal
	}

	switch insp.InspectionType {
	case "STK":
		item.LastStkDate = &insp.InspectionDate
		item.NextStkDue = insp.NextDueDate
	case "MTK":
		item.LastMtkDate = &insp.InspectionDate
		item.NextMtkDue = insp.NextDueDate
	}

	s.itemRepo.Update(ctx, item)
	return nil
}

// ListInspections lists inspections for a device
func (s *InventoryService) ListInspections(ctx context.Context, itemID string) ([]*repository.DeviceInspection, error) {
	return s.inspectionRepo.ListByItem(ctx, itemID)
}

// GetInspection gets an inspection by ID
func (s *InventoryService) GetInspection(ctx context.Context, id string) (*repository.DeviceInspection, error) {
	return s.inspectionRepo.GetByID(ctx, id)
}

// UpdateInspection updates an inspection
func (s *InventoryService) UpdateInspection(ctx context.Context, insp *repository.DeviceInspection) error {
	return s.inspectionRepo.Update(ctx, insp)
}

// DeleteInspection deletes an inspection
func (s *InventoryService) DeleteInspection(ctx context.Context, id string) error {
	return s.inspectionRepo.Delete(ctx, id)
}

// Device training operations

// CreateTraining creates a device training record
func (s *InventoryService) CreateTraining(ctx context.Context, tr *repository.DeviceTraining) error {
	return s.trainingRepo.Create(ctx, tr)
}

// ListTrainings lists trainings for a device
func (s *InventoryService) ListTrainings(ctx context.Context, itemID string) ([]*repository.DeviceTraining, error) {
	return s.trainingRepo.ListByItem(ctx, itemID)
}

// GetTraining gets a training by ID
func (s *InventoryService) GetTraining(ctx context.Context, id string) (*repository.DeviceTraining, error) {
	return s.trainingRepo.GetByID(ctx, id)
}

// UpdateTraining updates a training
func (s *InventoryService) UpdateTraining(ctx context.Context, tr *repository.DeviceTraining) error {
	return s.trainingRepo.Update(ctx, tr)
}

// DeleteTraining deletes a training
func (s *InventoryService) DeleteTraining(ctx context.Context, id string) error {
	return s.trainingRepo.Delete(ctx, id)
}

// Device incident operations

// CreateIncident creates a device incident record
func (s *InventoryService) CreateIncident(ctx context.Context, inc *repository.DeviceIncident) error {
	return s.incidentRepo.Create(ctx, inc)
}

// ListIncidents lists incidents for a device
func (s *InventoryService) ListIncidents(ctx context.Context, itemID string) ([]*repository.DeviceIncident, error) {
	return s.incidentRepo.ListByItem(ctx, itemID)
}

// GetIncident gets an incident by ID
func (s *InventoryService) GetIncident(ctx context.Context, id string) (*repository.DeviceIncident, error) {
	return s.incidentRepo.GetByID(ctx, id)
}

// UpdateIncident updates an incident
func (s *InventoryService) UpdateIncident(ctx context.Context, inc *repository.DeviceIncident) error {
	return s.incidentRepo.Update(ctx, inc)
}

// DeleteIncident deletes an incident
func (s *InventoryService) DeleteIncident(ctx context.Context, id string) error {
	return s.incidentRepo.Delete(ctx, id)
}

// GenerateMaintenanceAlerts generates alerts for overdue/upcoming STK/MTK inspections
func (s *InventoryService) GenerateMaintenanceAlerts(ctx context.Context) error {
	items, err := s.itemRepo.ListMedicalDevices(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	soonThreshold := now.AddDate(0, 0, 30)

	for _, item := range items {
		// STK checks
		if item.NextStkDue != nil {
			if item.NextStkDue.Before(now) {
				alert := &repository.InventoryAlert{
					AlertType: "stk_overdue",
					ItemID:    item.ID,
					ItemName:  item.Name,
					Severity:  "critical",
					Message:   fmt.Sprintf("STK overdue for %s (due: %s) - Betriebsverbot!", item.Name, item.NextStkDue.Format("2006-01-02")),
				}
				s.alertRepo.Create(ctx, alert)
			} else if item.NextStkDue.Before(soonThreshold) {
				alert := &repository.InventoryAlert{
					AlertType: "stk_due_soon",
					ItemID:    item.ID,
					ItemName:  item.Name,
					Severity:  "warning",
					Message:   fmt.Sprintf("STK due soon for %s (due: %s)", item.Name, item.NextStkDue.Format("2006-01-02")),
				}
				s.alertRepo.Create(ctx, alert)
			}
		}

		// MTK checks
		if item.NextMtkDue != nil {
			if item.NextMtkDue.Before(now) {
				alert := &repository.InventoryAlert{
					AlertType: "mtk_overdue",
					ItemID:    item.ID,
					ItemName:  item.Name,
					Severity:  "critical",
					Message:   fmt.Sprintf("MTK overdue for %s (due: %s)", item.Name, item.NextMtkDue.Format("2006-01-02")),
				}
				s.alertRepo.Create(ctx, alert)
			} else if item.NextMtkDue.Before(soonThreshold) {
				alert := &repository.InventoryAlert{
					AlertType: "mtk_due_soon",
					ItemID:    item.ID,
					ItemName:  item.Name,
					Severity:  "warning",
					Message:   fmt.Sprintf("MTK due soon for %s (due: %s)", item.Name, item.NextMtkDue.Format("2006-01-02")),
				}
				s.alertRepo.Create(ctx, alert)
			}
		}
	}

	return nil
}

// Scan operations

// GetItemByBarcode looks up an item by barcode, falling back to article number, then PZN.
// Only falls back on NotFound errors; real errors (DB connection, timeout) fail fast.
func (s *InventoryService) GetItemByBarcode(ctx context.Context, barcode string) (*ItemWithBatches, error) {
	if barcode == "" {
		return nil, apperrors.BadRequest("barcode is required")
	}

	// 1. Try barcode column
	item, err := s.itemRepo.GetByBarcode(ctx, barcode)
	if err == nil {
		return s.enrichItemWithBatches(ctx, item)
	}
	if !apperrors.Is(err, apperrors.ErrNotFound) {
		return nil, err // Real error — fail fast, don't fallback
	}

	// 2. Fallback: try article_number
	item, err = s.itemRepo.GetByArticleNumber(ctx, barcode)
	if err == nil {
		return s.enrichItemWithBatches(ctx, item)
	}
	if !apperrors.Is(err, apperrors.ErrNotFound) {
		return nil, err
	}

	// 3. Fallback: try PZN (strip optional "PZN-" or "PZN" prefix)
	pzn := barcode
	pzn = strings.TrimPrefix(pzn, "PZN-")
	pzn = strings.TrimPrefix(pzn, "PZN")
	if pzn != "" {
		item, err = s.itemRepo.GetByPZN(ctx, pzn)
		if err == nil {
			return s.enrichItemWithBatches(ctx, item)
		}
		if !apperrors.Is(err, apperrors.ErrNotFound) {
			return nil, err
		}
	}

	return nil, apperrors.NotFound("item")
}

// enrichItemWithBatches loads batches for an item and returns the enriched result.
func (s *InventoryService) enrichItemWithBatches(ctx context.Context, item *repository.InventoryItem) (*ItemWithBatches, error) {
	batches, err := s.batchRepo.ListByItem(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	return s.enrichItem(item, batches), nil
}

// BatchWithItem represents a batch with its parent item
type BatchWithItem struct {
	*repository.InventoryBatch
	Item *repository.InventoryItem `json:"item"`
}

// GetBatchByBatchNumber looks up a batch by batch number and includes parent item
func (s *InventoryService) GetBatchByBatchNumber(ctx context.Context, batchNumber string) (*BatchWithItem, error) {
	batch, err := s.batchRepo.GetByBatchNumber(ctx, batchNumber)
	if err != nil {
		return nil, err
	}

	item, err := s.itemRepo.GetByID(ctx, batch.ItemID)
	if err != nil {
		return nil, err
	}

	return &BatchWithItem{
		InventoryBatch: batch,
		Item:           item,
	}, nil
}

// Helper functions

func derefFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func (s *InventoryService) enrichItem(item *repository.InventoryItem, batches []*repository.InventoryBatch) *ItemWithBatches {
	result := &ItemWithBatches{
		InventoryItem: item,
		Batches:       batches,
	}

	// Calculate total stock
	for _, b := range batches {
		result.TotalStock += b.Quantity
	}

	// Find nearest expiry (considering effective expiry for opened batches)
	var nearestExpiry *time.Time
	for _, b := range batches {
		if b.Quantity > 0 {
			effectiveExpiry := GetEffectiveExpiry(b, item)
			if effectiveExpiry != nil {
				if nearestExpiry == nil || effectiveExpiry.Before(*nearestExpiry) {
					nearestExpiry = effectiveExpiry
				}
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
