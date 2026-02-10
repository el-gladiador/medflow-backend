package processor

import (
	"context"
	"time"

	"github.com/medflow/medflow-backend/internal/docprocessing/domain"
)

// CVProcessor is a stub processor for CV/resume documents.
// AI-powered CV extraction will be added in a future release.
type CVProcessor struct{}

func NewCVProcessor() *CVProcessor {
	return &CVProcessor{}
}

func (p *CVProcessor) Name() string {
	return "cv_stub"
}

func (p *CVProcessor) CanProcess(docType domain.DocumentType) bool {
	return docType == domain.DocumentTypeLebenslauf
}

func (p *CVProcessor) Process(ctx context.Context, imageData []byte, docType domain.DocumentType) (*domain.ExtractionResult, error) {
	start := time.Now()

	return &domain.ExtractionResult{
		DocumentType:     docType,
		Fields:           nil,
		Warnings:         []string{"CV extraction is not yet available. AI-powered extraction coming soon."},
		ProcessingTimeMs: time.Since(start).Milliseconds(),
	}, nil
}
