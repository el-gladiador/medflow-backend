package processor

import (
	"context"

	"github.com/medflow/medflow-backend/internal/docprocessing/domain"
)

// Processor defines the interface for document data extraction.
// Implementations can be swapped in to add AI/VLM capabilities
// without changing the service or handler layer.
type Processor interface {
	// CanProcess returns true if this processor handles the given document type
	CanProcess(docType domain.DocumentType) bool

	// Process extracts structured data from document image bytes.
	// The image data should NOT be retained after processing.
	Process(ctx context.Context, imageData []byte, docType domain.DocumentType) (*domain.ExtractionResult, error)

	// Name returns the processor name for logging/audit
	Name() string
}

// Registry holds all registered processors and dispatches to the right one
type Registry struct {
	processors []Processor
}

// NewRegistry creates a new processor registry
func NewRegistry(processors ...Processor) *Registry {
	return &Registry{processors: processors}
}

// FindProcessor returns the first processor that can handle the given document type
func (r *Registry) FindProcessor(docType domain.DocumentType) Processor {
	for _, p := range r.processors {
		if p.CanProcess(docType) {
			return p
		}
	}
	return nil
}

// FindProcessors returns all processors that can handle the given document type,
// in registration order. This supports fallback: if the first processor fails
// (e.g. VLM rejects non-image data), the next one can try.
func (r *Registry) FindProcessors(docType domain.DocumentType) []Processor {
	var result []Processor
	for _, p := range r.processors {
		if p.CanProcess(docType) {
			result = append(result, p)
		}
	}
	return result
}
