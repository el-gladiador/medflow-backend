package service

import (
	"context"
	"fmt"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// AlertScanner scans inventory data and generates alerts for compliance violations.
// Each scan method checks for a specific condition and creates alerts with deduplication.
type AlertScanner struct {
	itemRepo     *repository.ItemRepository
	batchRepo    *repository.BatchRepository
	alertRepo    *repository.AlertRepository
	tempRepo     *repository.TemperatureRepository
	locationRepo *repository.LocationRepository
	logger       *logger.Logger
}

// NewAlertScanner creates a new alert scanner
func NewAlertScanner(
	itemRepo *repository.ItemRepository,
	batchRepo *repository.BatchRepository,
	alertRepo *repository.AlertRepository,
	tempRepo *repository.TemperatureRepository,
	locationRepo *repository.LocationRepository,
	log *logger.Logger,
) *AlertScanner {
	return &AlertScanner{
		itemRepo:     itemRepo,
		batchRepo:    batchRepo,
		alertRepo:    alertRepo,
		tempRepo:     tempRepo,
		locationRepo: locationRepo,
		logger:       log,
	}
}

// ScanAll runs all alert scans. Logs errors but continues scanning.
func (s *AlertScanner) ScanAll(ctx context.Context) error {
	scanners := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"low_stock", s.scanLowStock},
		{"expiry", s.scanExpiryAlerts},
		{"maintenance", s.scanMaintenanceAlerts},
		{"temperature_missing", s.scanTemperatureMissing},
		{"opening_expired", s.scanOpeningExpired},
		{"resolve_cleared", s.resolveCleared},
	}

	var lastErr error
	for _, scanner := range scanners {
		if err := scanner.fn(ctx); err != nil {
			s.logger.Error().Err(err).Str("scanner", scanner.name).Msg("alert scan failed")
			lastErr = err
		}
	}

	return lastErr
}

// scanLowStock checks for low stock and out of stock conditions
func (s *AlertScanner) scanLowStock(ctx context.Context) error {
	items, err := s.itemRepo.GetAllActive(ctx)
	if err != nil {
		return fmt.Errorf("scanLowStock: get active items: %w", err)
	}

	for _, item := range items {
		totalStock, err := s.batchRepo.GetTotalStock(ctx, item.ID)
		if err != nil {
			s.logger.Error().Err(err).Str("item_id", item.ID).Msg("scanLowStock: failed to get total stock")
			continue
		}

		if totalStock >= item.MinStock {
			continue
		}

		alertType := "low_stock"
		severity := "warning"
		if totalStock == 0 {
			alertType = "out_of_stock"
			severity = "critical"
		} else if totalStock < item.MinStock/2 {
			severity = "critical"
		}

		// Dedup: check if alert already exists
		exists, err := s.alertRepo.ExistsByTypeAndEntity(ctx, alertType, item.ID, nil)
		if err != nil {
			s.logger.Error().Err(err).Str("item_id", item.ID).Msg("scanLowStock: failed to check existing alert")
			continue
		}
		if exists {
			continue
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

		if err := s.alertRepo.Create(ctx, alert); err != nil {
			s.logger.Error().Err(err).Str("item_id", item.ID).Msg("scanLowStock: failed to create alert")
		}
	}

	return nil
}

// scanExpiryAlerts checks for expiring and expired batches
func (s *AlertScanner) scanExpiryAlerts(ctx context.Context) error {
	batches, err := s.batchRepo.GetExpiringBatches(ctx, 90)
	if err != nil {
		return fmt.Errorf("scanExpiryAlerts: get expiring batches: %w", err)
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

		// Dedup: check if alert already exists for this batch
		exists, err := s.alertRepo.ExistsByTypeAndEntity(ctx, alertType, item.ID, &batch.ID)
		if err != nil {
			s.logger.Error().Err(err).Str("item_id", item.ID).Str("batch_id", batch.ID).Msg("scanExpiryAlerts: failed to check existing alert")
			continue
		}
		if exists {
			continue
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

		if err := s.alertRepo.Create(ctx, alert); err != nil {
			s.logger.Error().Err(err).Str("item_id", item.ID).Msg("scanExpiryAlerts: failed to create alert")
		}
	}

	return nil
}

// scanMaintenanceAlerts checks for overdue/upcoming STK/MTK inspections
func (s *AlertScanner) scanMaintenanceAlerts(ctx context.Context) error {
	items, err := s.itemRepo.ListMedicalDevices(ctx)
	if err != nil {
		return fmt.Errorf("scanMaintenanceAlerts: list medical devices: %w", err)
	}

	now := time.Now()
	soonThreshold := now.AddDate(0, 0, 30)

	for _, item := range items {
		// STK checks
		if item.NextStkDue != nil {
			if item.NextStkDue.Before(now) {
				s.createMaintenanceAlert(ctx, item, "stk_overdue", "critical",
					fmt.Sprintf("STK overdue for %s (due: %s) - Betriebsverbot!", item.Name, item.NextStkDue.Format("2006-01-02")))
			} else if item.NextStkDue.Before(soonThreshold) {
				s.createMaintenanceAlert(ctx, item, "stk_due_soon", "warning",
					fmt.Sprintf("STK due soon for %s (due: %s)", item.Name, item.NextStkDue.Format("2006-01-02")))
			}
		}

		// MTK checks
		if item.NextMtkDue != nil {
			if item.NextMtkDue.Before(now) {
				s.createMaintenanceAlert(ctx, item, "mtk_overdue", "critical",
					fmt.Sprintf("MTK overdue for %s (due: %s)", item.Name, item.NextMtkDue.Format("2006-01-02")))
			} else if item.NextMtkDue.Before(soonThreshold) {
				s.createMaintenanceAlert(ctx, item, "mtk_due_soon", "warning",
					fmt.Sprintf("MTK due soon for %s (due: %s)", item.Name, item.NextMtkDue.Format("2006-01-02")))
			}
		}
	}

	return nil
}

// createMaintenanceAlert creates a maintenance alert with deduplication
func (s *AlertScanner) createMaintenanceAlert(ctx context.Context, item *repository.InventoryItem, alertType, severity, message string) {
	exists, err := s.alertRepo.ExistsByTypeAndEntity(ctx, alertType, item.ID, nil)
	if err != nil {
		s.logger.Error().Err(err).Str("item_id", item.ID).Str("alert_type", alertType).Msg("failed to check existing maintenance alert")
		return
	}
	if exists {
		return
	}

	alert := &repository.InventoryAlert{
		AlertType: alertType,
		ItemID:    item.ID,
		ItemName:  item.Name,
		Severity:  severity,
		Message:   message,
	}

	if err := s.alertRepo.Create(ctx, alert); err != nil {
		s.logger.Error().Err(err).Str("item_id", item.ID).Str("alert_type", alertType).Msg("failed to create maintenance alert")
	}
}

// scanTemperatureMissing checks for monitored cabinets without today's readings
func (s *AlertScanner) scanTemperatureMissing(ctx context.Context) error {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	cabinetIDs, err := s.tempRepo.GetMonitoredCabinetsWithoutReading(ctx, startOfDay)
	if err != nil {
		return fmt.Errorf("scanTemperatureMissing: get cabinets: %w", err)
	}

	for _, cabID := range cabinetIDs {
		// Dedup check
		exists, err := s.alertRepo.ExistsByTypeAndEntity(ctx, "temperature_missing", cabID, nil)
		if err != nil {
			s.logger.Error().Err(err).Str("cabinet_id", cabID).Msg("scanTemperatureMissing: failed to check existing alert")
			continue
		}
		if exists {
			continue
		}

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

		if err := s.alertRepo.Create(ctx, alert); err != nil {
			s.logger.Error().Err(err).Str("cabinet_id", cabID).Msg("scanTemperatureMissing: failed to create alert")
		}
	}

	return nil
}

// scanOpeningExpired checks for opened batches past their post-opening shelf life
func (s *AlertScanner) scanOpeningExpired(ctx context.Context) error {
	items, err := s.itemRepo.GetAllActive(ctx)
	if err != nil {
		return fmt.Errorf("scanOpeningExpired: get active items: %w", err)
	}

	now := time.Now()

	for _, item := range items {
		if item.ShelfLifeAfterOpeningDays == nil {
			continue
		}

		batches, err := s.batchRepo.ListByItem(ctx, item.ID)
		if err != nil {
			continue
		}

		for _, batch := range batches {
			if batch.OpenedAt == nil {
				continue
			}

			openingExpiry := batch.OpenedAt.AddDate(0, 0, *item.ShelfLifeAfterOpeningDays)
			if openingExpiry.After(now) {
				continue
			}

			// Dedup check
			exists, err := s.alertRepo.ExistsByTypeAndEntity(ctx, "opening_expired", item.ID, &batch.ID)
			if err != nil {
				s.logger.Error().Err(err).Str("item_id", item.ID).Str("batch_id", batch.ID).Msg("scanOpeningExpired: failed to check existing alert")
				continue
			}
			if exists {
				continue
			}

			alert := &repository.InventoryAlert{
				AlertType:   "opening_expired",
				ItemID:      item.ID,
				ItemName:    item.Name,
				BatchID:     &batch.ID,
				BatchNumber: &batch.BatchNumber,
				Severity:    "critical",
				Message:     fmt.Sprintf("%s batch %s: post-opening shelf life expired", item.Name, batch.BatchNumber),
			}

			if err := s.alertRepo.Create(ctx, alert); err != nil {
				s.logger.Error().Err(err).Str("item_id", item.ID).Str("batch_id", batch.ID).Msg("scanOpeningExpired: failed to create alert")
			}
		}
	}

	return nil
}

// resolveCleared auto-resolves alerts when the underlying condition has been fixed.
// - low_stock/out_of_stock: stock now >= minStock
// - temperature_missing: reading exists today
func (s *AlertScanner) resolveCleared(ctx context.Context) error {
	activeAlerts, err := s.alertRepo.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("resolveCleared: list active alerts: %w", err)
	}

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, alert := range activeAlerts {
		switch alert.AlertType {
		case "low_stock", "out_of_stock":
			item, err := s.itemRepo.GetByID(ctx, alert.ItemID)
			if err != nil {
				continue
			}
			totalStock, err := s.batchRepo.GetTotalStock(ctx, alert.ItemID)
			if err != nil {
				continue
			}
			if totalStock >= item.MinStock {
				if err := s.alertRepo.Resolve(ctx, alert.ID, "system"); err != nil {
					s.logger.Error().Err(err).Str("alert_id", alert.ID).Msg("resolveCleared: failed to resolve stock alert")
				}
			}

		case "temperature_missing":
			// Check if a reading has been recorded today
			cabinetIDs, err := s.tempRepo.GetMonitoredCabinetsWithoutReading(ctx, startOfDay)
			if err != nil {
				continue
			}
			// If the cabinet now has a reading (not in the missing list), resolve
			found := false
			for _, id := range cabinetIDs {
				if id == alert.ItemID {
					found = true
					break
				}
			}
			if !found {
				if err := s.alertRepo.Resolve(ctx, alert.ID, "system"); err != nil {
					s.logger.Error().Err(err).Str("alert_id", alert.ID).Msg("resolveCleared: failed to resolve temperature alert")
				}
			}
		}
	}

	return nil
}
