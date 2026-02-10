package processor

import (
	"context"
	"time"

	"github.com/medflow/medflow-backend/internal/docprocessing/domain"
)

// LicenseProcessor is a stub processor for Fuhrerschein (driver's license).
// In the initial version, it returns a warning that manual review is required.
// A future VLM processor will handle actual license data extraction.
type LicenseProcessor struct{}

func NewLicenseProcessor() *LicenseProcessor {
	return &LicenseProcessor{}
}

func (p *LicenseProcessor) Name() string {
	return "license_stub"
}

func (p *LicenseProcessor) CanProcess(docType domain.DocumentType) bool {
	return docType == domain.DocumentTypeFuehrerschein
}

func (p *LicenseProcessor) Process(ctx context.Context, imageData []byte, docType domain.DocumentType) (*domain.ExtractionResult, error) {
	start := time.Now()

	return &domain.ExtractionResult{
		DocumentType:     docType,
		Fields:           nil,
		Warnings:         []string{"Driver's license extraction requires manual review. AI-powered extraction coming soon."},
		ProcessingTimeMs: time.Since(start).Milliseconds(),
	}, nil
}
