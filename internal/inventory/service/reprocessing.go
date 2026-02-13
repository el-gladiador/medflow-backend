package service

import (
	"context"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// ReprocessingService handles reprocessing and sterilization business logic (KRINKO compliance)
type ReprocessingService struct {
	reprocessingRepo *repository.ReprocessingRepository
	auditService     *AuditService
	logger           *logger.Logger
}

// NewReprocessingService creates a new reprocessing service
func NewReprocessingService(reprocessingRepo *repository.ReprocessingRepository, auditService *AuditService, log *logger.Logger) *ReprocessingService {
	return &ReprocessingService{
		reprocessingRepo: reprocessingRepo,
		auditService:     auditService,
		logger:           log,
	}
}

// --- Sterilization Batches ---

// CreateBatch creates a new sterilization batch
func (s *ReprocessingService) CreateBatch(ctx context.Context, batch *repository.SterilizationBatch) error {
	if err := s.reprocessingRepo.CreateBatch(ctx, batch); err != nil {
		return err
	}

	s.auditService.RecordCreate(ctx, "sterilization_batch", batch.ID, map[string]interface{}{
		"batch_number":    batch.BatchNumber,
		"sterilizer_name": batch.SterilizerName,
		"overall_result":  batch.OverallResult,
	})

	return nil
}

// GetBatch gets a sterilization batch by ID
func (s *ReprocessingService) GetBatch(ctx context.Context, id string) (*repository.SterilizationBatch, error) {
	return s.reprocessingRepo.GetBatch(ctx, id)
}

// ListBatches lists sterilization batches with pagination
func (s *ReprocessingService) ListBatches(ctx context.Context, page, perPage int) ([]*repository.SterilizationBatch, int64, error) {
	return s.reprocessingRepo.ListBatches(ctx, page, perPage)
}

// UpdateBatch updates a sterilization batch
func (s *ReprocessingService) UpdateBatch(ctx context.Context, batch *repository.SterilizationBatch) error {
	if err := s.reprocessingRepo.UpdateBatch(ctx, batch); err != nil {
		return err
	}

	s.auditService.RecordUpdate(ctx, "sterilization_batch", batch.ID, map[string]interface{}{
		"batch_number":   batch.BatchNumber,
		"overall_result": batch.OverallResult,
	}, nil)

	return nil
}

// DeleteBatch soft-deletes a sterilization batch
func (s *ReprocessingService) DeleteBatch(ctx context.Context, id string) error {
	if err := s.reprocessingRepo.DeleteBatch(ctx, id); err != nil {
		return err
	}

	s.auditService.RecordDelete(ctx, "sterilization_batch", id, nil)

	return nil
}

// --- Reprocessing Cycles ---

// CreateCycle creates a new reprocessing cycle with auto-incremented cycle number
func (s *ReprocessingService) CreateCycle(ctx context.Context, cycle *repository.ReprocessingCycle) error {
	if err := s.reprocessingRepo.CreateCycle(ctx, cycle); err != nil {
		return err
	}

	s.auditService.RecordCreate(ctx, "reprocessing_cycle", cycle.ID, map[string]interface{}{
		"item_id":      cycle.ItemID,
		"cycle_number": cycle.CycleNumber,
	})

	return nil
}

// GetCycle gets a reprocessing cycle by ID
func (s *ReprocessingService) GetCycle(ctx context.Context, id string) (*repository.ReprocessingCycle, error) {
	return s.reprocessingRepo.GetCycle(ctx, id)
}

// ListCyclesByItem lists reprocessing cycles for an item
func (s *ReprocessingService) ListCyclesByItem(ctx context.Context, itemID string) ([]*repository.ReprocessingCycle, error) {
	return s.reprocessingRepo.ListCyclesByItem(ctx, itemID)
}

// UpdateCycle updates a reprocessing cycle
func (s *ReprocessingService) UpdateCycle(ctx context.Context, cycle *repository.ReprocessingCycle) error {
	if err := s.reprocessingRepo.UpdateCycle(ctx, cycle); err != nil {
		return err
	}

	s.auditService.RecordUpdate(ctx, "reprocessing_cycle", cycle.ID, map[string]interface{}{
		"cycle_number": cycle.CycleNumber,
	}, nil)

	return nil
}

// DeleteCycle soft-deletes a reprocessing cycle
func (s *ReprocessingService) DeleteCycle(ctx context.Context, id string) error {
	if err := s.reprocessingRepo.DeleteCycle(ctx, id); err != nil {
		return err
	}

	s.auditService.RecordDelete(ctx, "reprocessing_cycle", id, nil)

	return nil
}
