package service

import (
	"context"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// RetentionService handles retention policy business logic
type RetentionService struct {
	retentionRepo *repository.RetentionRepository
	auditService  *AuditService
	logger        *logger.Logger
}

// NewRetentionService creates a new retention service
func NewRetentionService(retentionRepo *repository.RetentionRepository, auditService *AuditService, log *logger.Logger) *RetentionService {
	return &RetentionService{
		retentionRepo: retentionRepo,
		auditService:  auditService,
		logger:        log,
	}
}

// Create creates a new retention policy
func (s *RetentionService) Create(ctx context.Context, policy *repository.RetentionPolicy) error {
	if err := s.retentionRepo.Create(ctx, policy); err != nil {
		return err
	}

	s.auditService.RecordCreate(ctx, "retention_policy", policy.ID, map[string]interface{}{
		"entity_type":     policy.EntityType,
		"retention_years": policy.RetentionYears,
	})

	return nil
}

// GetByEntityType gets the retention policy for a specific entity type
func (s *RetentionService) GetByEntityType(ctx context.Context, entityType string) (*repository.RetentionPolicy, error) {
	return s.retentionRepo.GetByEntityType(ctx, entityType)
}

// List lists all retention policies
func (s *RetentionService) List(ctx context.Context) ([]*repository.RetentionPolicy, error) {
	return s.retentionRepo.List(ctx)
}

// Update updates a retention policy
func (s *RetentionService) Update(ctx context.Context, policy *repository.RetentionPolicy) error {
	if err := s.retentionRepo.Update(ctx, policy); err != nil {
		return err
	}

	s.auditService.RecordUpdate(ctx, "retention_policy", policy.ID, map[string]interface{}{
		"entity_type":     policy.EntityType,
		"retention_years": policy.RetentionYears,
	}, nil)

	return nil
}

// Delete soft-deletes a retention policy
func (s *RetentionService) Delete(ctx context.Context, id string) error {
	if err := s.retentionRepo.Delete(ctx, id); err != nil {
		return err
	}

	s.auditService.RecordDelete(ctx, "retention_policy", id, nil)

	return nil
}

// ValidateDeletion checks if a record can be deleted based on retention policies
func (s *RetentionService) ValidateDeletion(ctx context.Context, entityType string, recordDate time.Time) error {
	return s.retentionRepo.ValidateDeletion(ctx, entityType, recordDate)
}
