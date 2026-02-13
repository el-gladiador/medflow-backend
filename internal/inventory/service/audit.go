package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// AuditService handles audit trail business logic for GoBD compliance.
// It computes diffs and records changes as append-only audit entries.
type AuditService struct {
	repo   *repository.AuditTrailRepository
	logger *logger.Logger
}

// NewAuditService creates a new audit service
func NewAuditService(repo *repository.AuditTrailRepository, log *logger.Logger) *AuditService {
	return &AuditService{
		repo:   repo,
		logger: log,
	}
}

// RecordCreate records a create action in the audit trail
func (s *AuditService) RecordCreate(ctx context.Context, entityType, entityID string, metadata map[string]interface{}) error {
	return s.record(ctx, entityType, entityID, "create", nil, metadata)
}

// RecordUpdate records an update action with field changes in the audit trail
func (s *AuditService) RecordUpdate(ctx context.Context, entityType, entityID string, changes map[string]interface{}, metadata map[string]interface{}) error {
	return s.record(ctx, entityType, entityID, "update", changes, metadata)
}

// RecordDelete records a delete action in the audit trail
func (s *AuditService) RecordDelete(ctx context.Context, entityType, entityID string, metadata map[string]interface{}) error {
	return s.record(ctx, entityType, entityID, "delete", nil, metadata)
}

// RecordAction records a custom action in the audit trail
func (s *AuditService) RecordAction(ctx context.Context, entityType, entityID, action string, metadata map[string]interface{}) error {
	return s.record(ctx, entityType, entityID, action, nil, metadata)
}

// ListByEntity lists audit entries for a specific entity with pagination
func (s *AuditService) ListByEntity(ctx context.Context, entityType, entityID string, page, perPage int) ([]*repository.AuditEntry, int64, error) {
	return s.repo.ListByEntity(ctx, entityType, entityID, page, perPage)
}

// ListByTenant lists audit entries for the tenant with optional filters
func (s *AuditService) ListByTenant(ctx context.Context, entityType string, from, to *time.Time, page, perPage int) ([]*repository.AuditEntry, int64, error) {
	return s.repo.ListByTenant(ctx, entityType, from, to, page, perPage)
}

// ExportGoBD exports all audit entries in a date range for GoBD compliance
func (s *AuditService) ExportGoBD(ctx context.Context, from, to *time.Time) ([]*repository.AuditEntry, error) {
	return s.repo.ExportGoBD(ctx, from, to)
}

// record is the internal helper that constructs an AuditEntry and persists it
func (s *AuditService) record(ctx context.Context, entityType, entityID, action string, fieldChanges, metadata map[string]interface{}) error {
	entry := &repository.AuditEntry{
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
	}

	// Extract user from context
	userID := httputil.GetUserID(ctx)
	if userID != "" {
		entry.PerformedBy = &userID
	}

	// Marshal field changes to JSON string
	if fieldChanges != nil {
		changesJSON, err := json.Marshal(fieldChanges)
		if err != nil {
			s.logger.Error().Err(err).Str("entity_type", entityType).Str("entity_id", entityID).Msg("failed to marshal field changes")
		} else {
			changesStr := string(changesJSON)
			entry.FieldChanges = &changesStr
		}
	}

	// Marshal metadata to JSON string
	if metadata != nil {
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			s.logger.Error().Err(err).Str("entity_type", entityType).Str("entity_id", entityID).Msg("failed to marshal metadata")
		} else {
			metaStr := string(metadataJSON)
			entry.Metadata = &metaStr
		}
	}

	if err := s.repo.Create(ctx, entry); err != nil {
		s.logger.Error().Err(err).
			Str("entity_type", entityType).
			Str("entity_id", entityID).
			Str("action", action).
			Msg("failed to create audit entry")
		return err
	}

	return nil
}
