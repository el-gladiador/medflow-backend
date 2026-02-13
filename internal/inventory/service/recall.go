package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// RecallService handles field safety notice and recall matching business logic.
// It matches incoming notices against the tenant's inventory and creates recall matches.
type RecallService struct {
	recallRepo   *repository.RecallRepository
	safetyRepo   *repository.SafetyOfficerRepository
	itemRepo     *repository.ItemRepository
	batchRepo    *repository.BatchRepository
	alertRepo    *repository.AlertRepository
	auditService *AuditService
	logger       *logger.Logger
}

// NewRecallService creates a new recall service
func NewRecallService(
	recallRepo *repository.RecallRepository,
	safetyRepo *repository.SafetyOfficerRepository,
	itemRepo *repository.ItemRepository,
	batchRepo *repository.BatchRepository,
	alertRepo *repository.AlertRepository,
	auditService *AuditService,
	log *logger.Logger,
) *RecallService {
	return &RecallService{
		recallRepo:   recallRepo,
		safetyRepo:   safetyRepo,
		itemRepo:     itemRepo,
		batchRepo:    batchRepo,
		alertRepo:    alertRepo,
		auditService: auditService,
		logger:       log,
	}
}

// CreateFieldSafetyNotice creates a new notice and matches it against inventory
func (s *RecallService) CreateFieldSafetyNotice(ctx context.Context, notice *repository.FieldSafetyNotice) error {
	if err := s.recallRepo.CreateNotice(ctx, notice); err != nil {
		return fmt.Errorf("failed to create field safety notice: %w", err)
	}

	// Match against existing inventory
	if err := s.matchAgainstInventory(ctx, notice); err != nil {
		s.logger.Error().Err(err).Str("notice_id", notice.ID).Msg("failed to match notice against inventory")
		// Don't fail the creation — matching is best-effort
	}

	// Record audit
	s.auditService.RecordCreate(ctx, "field_safety_notice", notice.ID, map[string]interface{}{
		"notice_number": notice.NoticeNumber,
		"notice_type":   notice.NoticeType,
		"severity":      notice.Severity,
		"title":         notice.Title,
	})

	return nil
}

// matchAgainstInventory matches a notice against all active inventory items and batches
func (s *RecallService) matchAgainstInventory(ctx context.Context, notice *repository.FieldSafetyNotice) error {
	items, err := s.itemRepo.GetAllActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active items: %w", err)
	}

	// Parse affected identifiers from the notice (comma-separated)
	affectedBatches := parseCommaSeparated(notice.AffectedBatchNumbers)
	affectedUdiDIs := parseCommaSeparated(notice.AffectedUdiDIs)
	affectedSerials := parseCommaSeparated(notice.AffectedSerialNumbers)

	matchCount := 0

	for _, item := range items {
		// Match by UDI-DI
		if item.UdiDI != nil && *item.UdiDI != "" {
			for _, udi := range affectedUdiDIs {
				if strings.EqualFold(*item.UdiDI, udi) {
					match := &repository.RecallMatch{
						NoticeID:     notice.ID,
						ItemID:       item.ID,
						MatchType:    "udi_di",
						MatchedValue: item.UdiDI,
					}
					if err := s.recallRepo.CreateMatch(ctx, match); err != nil {
						s.logger.Error().Err(err).Str("item_id", item.ID).Msg("failed to create UDI-DI recall match")
					} else {
						matchCount++
					}
				}
			}
		}

		// Match by serial number
		if item.SerialNumber != nil && *item.SerialNumber != "" {
			for _, serial := range affectedSerials {
				if strings.EqualFold(*item.SerialNumber, serial) {
					match := &repository.RecallMatch{
						NoticeID:     notice.ID,
						ItemID:       item.ID,
						MatchType:    "serial_number",
						MatchedValue: item.SerialNumber,
					}
					if err := s.recallRepo.CreateMatch(ctx, match); err != nil {
						s.logger.Error().Err(err).Str("item_id", item.ID).Msg("failed to create serial number recall match")
					} else {
						matchCount++
					}
				}
			}
		}

		// Match batches by batch number
		if len(affectedBatches) > 0 {
			batches, err := s.batchRepo.ListByItem(ctx, item.ID)
			if err != nil {
				s.logger.Error().Err(err).Str("item_id", item.ID).Msg("failed to list batches for recall matching")
				continue
			}

			for _, batch := range batches {
				for _, affectedBatch := range affectedBatches {
					if strings.EqualFold(batch.BatchNumber, affectedBatch) {
						matchedValue := batch.BatchNumber
						match := &repository.RecallMatch{
							NoticeID:     notice.ID,
							ItemID:       item.ID,
							BatchID:      &batch.ID,
							MatchType:    "batch_number",
							MatchedValue: &matchedValue,
						}
						if err := s.recallRepo.CreateMatch(ctx, match); err != nil {
							s.logger.Error().Err(err).Str("item_id", item.ID).Str("batch_id", batch.ID).Msg("failed to create batch recall match")
						} else {
							matchCount++
						}
					}
				}
			}
		}
	}

	if matchCount > 0 {
		s.logger.Warn().
			Int("match_count", matchCount).
			Str("notice_id", notice.ID).
			Str("notice_number", notice.NoticeNumber).
			Msg("field safety notice matches found in inventory")
	}

	return nil
}

// GetNotice gets a field safety notice by ID
func (s *RecallService) GetNotice(ctx context.Context, id string) (*repository.FieldSafetyNotice, error) {
	return s.recallRepo.GetNotice(ctx, id)
}

// ListNotices lists field safety notices with optional status filter
func (s *RecallService) ListNotices(ctx context.Context, status string, page, perPage int) ([]*repository.FieldSafetyNotice, int64, error) {
	return s.recallRepo.ListNotices(ctx, status, page, perPage)
}

// UpdateNoticeStatus updates the status of a field safety notice
func (s *RecallService) UpdateNoticeStatus(ctx context.Context, id, status string) error {
	notice, err := s.recallRepo.GetNotice(ctx, id)
	if err != nil {
		return err
	}

	oldStatus := notice.Status
	notice.Status = status

	if err := s.recallRepo.UpdateNotice(ctx, notice); err != nil {
		return err
	}

	// Record audit
	s.auditService.RecordUpdate(ctx, "field_safety_notice", id, map[string]interface{}{
		"status": map[string]string{
			"old": oldStatus,
			"new": status,
		},
	}, nil)

	return nil
}

// ResolveMatch resolves a recall match with action taken
func (s *RecallService) ResolveMatch(ctx context.Context, matchID, actionTaken, actionBy string) error {
	if err := s.recallRepo.UpdateMatchStatus(ctx, matchID, "resolved", actionTaken, actionBy); err != nil {
		return fmt.Errorf("failed to resolve recall match: %w", err)
	}

	// Record audit
	s.auditService.RecordAction(ctx, "recall_match", matchID, "resolve", map[string]interface{}{
		"action_taken": actionTaken,
		"action_by":    actionBy,
	})

	return nil
}

// ListPendingMatches lists all pending recall matches across items
func (s *RecallService) ListPendingMatches(ctx context.Context) ([]*repository.RecallMatch, int64, error) {
	return s.recallRepo.ListPendingMatches(ctx)
}

// ListMatchesByNotice lists recall matches for a specific notice
func (s *RecallService) ListMatchesByNotice(ctx context.Context, noticeID string) ([]*repository.RecallMatch, error) {
	return s.recallRepo.ListMatchesByNotice(ctx, noticeID)
}

// parseCommaSeparated splits a comma-separated string into trimmed, non-empty values
func parseCommaSeparated(s *string) []string {
	if s == nil || *s == "" {
		return nil
	}

	parts := strings.Split(*s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
