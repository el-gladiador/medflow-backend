package service

import (
	"context"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// BioSafetyService handles biological safety business logic (BioStoffV compliance)
type BioSafetyService struct {
	bioRepo      *repository.BioSafetyRepository
	auditService *AuditService
	logger       *logger.Logger
}

// NewBioSafetyService creates a new biosafety service
func NewBioSafetyService(bioRepo *repository.BioSafetyRepository, auditService *AuditService, log *logger.Logger) *BioSafetyService {
	return &BioSafetyService{
		bioRepo:      bioRepo,
		auditService: auditService,
		logger:       log,
	}
}

// --- Risk Assessments ---

// CreateAssessment creates a new bio risk assessment
func (s *BioSafetyService) CreateAssessment(ctx context.Context, assessment *repository.BioRiskAssessment) error {
	if err := s.bioRepo.CreateAssessment(ctx, assessment); err != nil {
		return err
	}

	s.auditService.RecordCreate(ctx, "bio_risk_assessment", assessment.ID, map[string]interface{}{
		"item_id":    assessment.ItemID,
		"risk_group": assessment.RiskGroup,
	})

	return nil
}

// GetAssessment gets a risk assessment by ID
func (s *BioSafetyService) GetAssessment(ctx context.Context, id string) (*repository.BioRiskAssessment, error) {
	return s.bioRepo.GetAssessment(ctx, id)
}

// ListAssessmentsByItem lists risk assessments for an item
func (s *BioSafetyService) ListAssessmentsByItem(ctx context.Context, itemID string) ([]*repository.BioRiskAssessment, error) {
	return s.bioRepo.ListAssessmentsByItem(ctx, itemID)
}

// UpdateAssessment updates a risk assessment
func (s *BioSafetyService) UpdateAssessment(ctx context.Context, assessment *repository.BioRiskAssessment) error {
	if err := s.bioRepo.UpdateAssessment(ctx, assessment); err != nil {
		return err
	}

	s.auditService.RecordUpdate(ctx, "bio_risk_assessment", assessment.ID, map[string]interface{}{
		"risk_group":    assessment.RiskGroup,
		"assessor_name": assessment.AssessorName,
	}, nil)

	return nil
}

// DeleteAssessment soft-deletes a risk assessment
func (s *BioSafetyService) DeleteAssessment(ctx context.Context, id string) error {
	if err := s.bioRepo.DeleteAssessment(ctx, id); err != nil {
		return err
	}

	s.auditService.RecordDelete(ctx, "bio_risk_assessment", id, nil)

	return nil
}

// --- Bio Trainings ---

// CreateTraining creates a new bio training record
func (s *BioSafetyService) CreateTraining(ctx context.Context, training *repository.BioTraining) error {
	if err := s.bioRepo.CreateTraining(ctx, training); err != nil {
		return err
	}

	s.auditService.RecordCreate(ctx, "bio_training", training.ID, map[string]interface{}{
		"training_type": training.TrainingType,
		"trainer_name":  training.TrainerName,
	})

	return nil
}

// GetTraining gets a bio training by ID
func (s *BioSafetyService) GetTraining(ctx context.Context, id string) (*repository.BioTraining, error) {
	return s.bioRepo.GetTraining(ctx, id)
}

// ListTrainings lists all bio trainings
func (s *BioSafetyService) ListTrainings(ctx context.Context) ([]*repository.BioTraining, error) {
	return s.bioRepo.ListTrainings(ctx)
}

// UpdateTraining updates a bio training record
func (s *BioSafetyService) UpdateTraining(ctx context.Context, training *repository.BioTraining) error {
	if err := s.bioRepo.UpdateTraining(ctx, training); err != nil {
		return err
	}

	s.auditService.RecordUpdate(ctx, "bio_training", training.ID, map[string]interface{}{
		"training_type": training.TrainingType,
		"trainer_name":  training.TrainerName,
	}, nil)

	return nil
}

// DeleteTraining soft-deletes a bio training record
func (s *BioSafetyService) DeleteTraining(ctx context.Context, id string) error {
	if err := s.bioRepo.DeleteTraining(ctx, id); err != nil {
		return err
	}

	s.auditService.RecordDelete(ctx, "bio_training", id, nil)

	return nil
}
