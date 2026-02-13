package service

import (
	"context"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// HygieneService handles hygiene plan and inspection business logic (IfSG compliance)
type HygieneService struct {
	hygieneRepo  *repository.HygieneRepository
	auditService *AuditService
	logger       *logger.Logger
}

// NewHygieneService creates a new hygiene service
func NewHygieneService(hygieneRepo *repository.HygieneRepository, auditService *AuditService, log *logger.Logger) *HygieneService {
	return &HygieneService{
		hygieneRepo:  hygieneRepo,
		auditService: auditService,
		logger:       log,
	}
}

// --- Hygiene Plans ---

// CreatePlan creates a new hygiene plan
func (s *HygieneService) CreatePlan(ctx context.Context, plan *repository.HygienePlan) error {
	if err := s.hygieneRepo.CreatePlan(ctx, plan); err != nil {
		return err
	}

	s.auditService.RecordCreate(ctx, "hygiene_plan", plan.ID, map[string]interface{}{
		"title":    plan.Title,
		"version":  plan.Version,
		"category": plan.Category,
		"status":   plan.Status,
	})

	return nil
}

// GetPlan gets a hygiene plan by ID
func (s *HygieneService) GetPlan(ctx context.Context, id string) (*repository.HygienePlan, error) {
	return s.hygieneRepo.GetPlan(ctx, id)
}

// ListPlans lists hygiene plans with pagination and optional filters
func (s *HygieneService) ListPlans(ctx context.Context, status, category string, page, perPage int) ([]*repository.HygienePlan, int64, error) {
	return s.hygieneRepo.ListPlans(ctx, status, category, page, perPage)
}

// UpdatePlan updates a hygiene plan
func (s *HygieneService) UpdatePlan(ctx context.Context, plan *repository.HygienePlan) error {
	if err := s.hygieneRepo.UpdatePlan(ctx, plan); err != nil {
		return err
	}

	s.auditService.RecordUpdate(ctx, "hygiene_plan", plan.ID, map[string]interface{}{
		"title":    plan.Title,
		"version":  plan.Version,
		"category": plan.Category,
		"status":   plan.Status,
	}, nil)

	return nil
}

// DeletePlan soft-deletes a hygiene plan
func (s *HygieneService) DeletePlan(ctx context.Context, id string) error {
	if err := s.hygieneRepo.DeletePlan(ctx, id); err != nil {
		return err
	}

	s.auditService.RecordDelete(ctx, "hygiene_plan", id, nil)

	return nil
}

// --- Hygiene Inspections ---

// CreateInspection creates a new hygiene inspection
func (s *HygieneService) CreateInspection(ctx context.Context, inspection *repository.HygieneInspection) error {
	if err := s.hygieneRepo.CreateInspection(ctx, inspection); err != nil {
		return err
	}

	s.auditService.RecordCreate(ctx, "hygiene_inspection", inspection.ID, map[string]interface{}{
		"inspector_name": inspection.InspectorName,
		"overall_result": inspection.OverallResult,
	})

	return nil
}

// GetInspection gets a hygiene inspection by ID
func (s *HygieneService) GetInspection(ctx context.Context, id string) (*repository.HygieneInspection, error) {
	return s.hygieneRepo.GetInspection(ctx, id)
}

// ListInspections lists hygiene inspections with pagination and optional plan_id filter
func (s *HygieneService) ListInspections(ctx context.Context, planID string, page, perPage int) ([]*repository.HygieneInspection, int64, error) {
	return s.hygieneRepo.ListInspections(ctx, planID, page, perPage)
}

// UpdateInspection updates a hygiene inspection
func (s *HygieneService) UpdateInspection(ctx context.Context, inspection *repository.HygieneInspection) error {
	if err := s.hygieneRepo.UpdateInspection(ctx, inspection); err != nil {
		return err
	}

	s.auditService.RecordUpdate(ctx, "hygiene_inspection", inspection.ID, map[string]interface{}{
		"inspector_name": inspection.InspectorName,
		"overall_result": inspection.OverallResult,
	}, nil)

	return nil
}

// DeleteInspection soft-deletes a hygiene inspection
func (s *HygieneService) DeleteInspection(ctx context.Context, id string) error {
	if err := s.hygieneRepo.DeleteInspection(ctx, id); err != nil {
		return err
	}

	s.auditService.RecordDelete(ctx, "hygiene_inspection", id, nil)

	return nil
}
