package service

import (
	"context"
	"fmt"
	"time"

	"github.com/medflow/medflow-backend/internal/docprocessing/domain"
	"github.com/medflow/medflow-backend/internal/docprocessing/processor"
	"github.com/medflow/medflow-backend/internal/docprocessing/storage"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// Service orchestrates document processing: detect type → dispatch → cleanup
type Service struct {
	registry *processor.Registry
	storage  *storage.TempStorage
	db       *database.DB
	log      *logger.Logger
}

// NewService creates a new document processing service
func NewService(registry *processor.Registry, store *storage.TempStorage, db *database.DB, log *logger.Logger) *Service {
	return &Service{
		registry: registry,
		storage:  store,
		db:       db,
		log:      log,
	}
}

// StartExtraction creates a new extraction job and processes the document asynchronously.
// Returns the job immediately so the caller can poll for results.
// Image bytes are zeroed immediately after processing.
func (s *Service) StartExtraction(ctx context.Context, imageData []byte, docType domain.DocumentType, consentTimestamp time.Time, userID string) (*domain.ExtractionJob, error) {
	jobID := storage.GenerateJobID()

	// Create job in processing state
	job := &domain.ExtractionJob{
		JobID:     jobID,
		Status:    domain.StatusProcessing,
		CreatedAt: time.Now(),
	}
	s.storage.StoreJob(job)

	// Find all processors that can handle this document type (supports fallback)
	processors := s.registry.FindProcessors(docType)
	if len(processors) == 0 {
		s.storage.UpdateJob(jobID, func(j *domain.ExtractionJob) {
			j.Status = domain.StatusFailed
			j.Error = fmt.Sprintf("no processor available for document type: %s", docType)
		})
		storage.ZeroBytes(imageData)
		return s.storage.GetJob(jobID), nil
	}

	// Process asynchronously — return job ID immediately for polling
	go s.processAsync(ctx, jobID, imageData, docType, processors, consentTimestamp, userID)

	return s.storage.GetJob(jobID), nil
}

// processAsync runs extraction in a background goroutine.
func (s *Service) processAsync(ctx context.Context, jobID string, imageData []byte, docType domain.DocumentType, processors []processor.Processor, consentTimestamp time.Time, userID string) {
	// Use a detached context so the request cancellation doesn't kill processing
	bgCtx := context.Background()

	// Try processors in order; if one fails, fall through to the next
	var result *domain.ExtractionResult
	var lastErr error
	for _, proc := range processors {
		s.log.Info().
			Str("job_id", jobID).
			Str("processor", proc.Name()).
			Str("doc_type", string(docType)).
			Msg("trying document extraction")

		result, lastErr = proc.Process(bgCtx, imageData, docType)
		if lastErr == nil {
			s.log.Info().
				Str("job_id", jobID).
				Str("processor", proc.Name()).
				Msg("processor succeeded")
			break
		}
		s.log.Warn().Err(lastErr).
			Str("job_id", jobID).
			Str("processor", proc.Name()).
			Msg("processor failed, trying next")
	}

	// CRITICAL: Zero image data immediately after processing (DSGVO compliance)
	storage.ZeroBytes(imageData)
	imageDeletedAt := time.Now()

	if lastErr != nil {
		s.storage.UpdateJob(jobID, func(j *domain.ExtractionJob) {
			j.Status = domain.StatusFailed
			j.Error = lastErr.Error()
		})
		s.log.Error().Err(lastErr).Str("job_id", jobID).Msg("all processors failed")
		return
	}

	// Update job with results
	s.storage.UpdateJob(jobID, func(j *domain.ExtractionJob) {
		j.Status = domain.StatusCompleted
		j.Results = []domain.ExtractionResult{*result}
	})

	// Write audit log (async, non-blocking)
	go s.writeAuditLog(bgCtx, docType, consentTimestamp, userID, result, imageDeletedAt)

	s.log.Info().
		Str("job_id", jobID).
		Int("fields_extracted", len(result.Fields)).
		Int64("duration_ms", result.ProcessingTimeMs).
		Msg("document extraction completed")
}

// GetJob retrieves an extraction job by ID
func (s *Service) GetJob(jobID string) *domain.ExtractionJob {
	return s.storage.GetJob(jobID)
}

// writeAuditLog records the processing event in the tenant's audit table
func (s *Service) writeAuditLog(ctx context.Context, docType domain.DocumentType, consentTimestamp time.Time, userID string, result *domain.ExtractionResult, imageDeletedAt time.Time) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		s.log.Warn().Err(err).Msg("cannot write audit log: no tenant context")
		return
	}

	// Collect extracted field keys
	fieldKeys := make([]string, len(result.Fields))
	for i, f := range result.Fields {
		fieldKeys[i] = f.Key
	}

	err = s.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `INSERT INTO document_processing_audit
			(document_type, consent_timestamp, consent_given_by, fields_extracted, processing_duration_ms, image_deleted_at)
			VALUES ($1, $2, $3, $4, $5, $6)`
		_, err := s.db.ExecContext(ctx, query,
			string(docType),
			consentTimestamp,
			userID,
			fmt.Sprintf("{%s}", joinStrings(fieldKeys)),
			result.ProcessingTimeMs,
			imageDeletedAt,
		)
		return err
	})

	if err != nil {
		s.log.Error().Err(err).Msg("failed to write document processing audit log")
	}
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}
