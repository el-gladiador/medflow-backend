package domain

import "time"

// DocumentType represents the type of document being processed
type DocumentType string

const (
	DocumentTypePersonalausweis DocumentType = "personalausweis"
	DocumentTypeReisepass       DocumentType = "reisepass"
	DocumentTypeFuehrerschein   DocumentType = "fuehrerschein"
	DocumentTypeLebenslauf      DocumentType = "lebenslauf"
)

// ExtractionStatus represents the processing state of an extraction job
type ExtractionStatus string

const (
	StatusPending    ExtractionStatus = "pending"
	StatusProcessing ExtractionStatus = "processing"
	StatusCompleted  ExtractionStatus = "completed"
	StatusFailed     ExtractionStatus = "failed"
)

// ExtractionField represents a single extracted field with confidence
type ExtractionField struct {
	Key        string       `json:"key"`
	Value      string       `json:"value"`
	Confidence float64      `json:"confidence"`
	Source     DocumentType `json:"source"`
}

// ExtractionResult represents the result from processing a single document
type ExtractionResult struct {
	DocumentType     DocumentType      `json:"document_type"`
	Fields           []ExtractionField `json:"fields"`
	Warnings         []string          `json:"warnings,omitempty"`
	ProcessingTimeMs int64             `json:"processing_time_ms"`
}

// ExtractionJob represents a complete extraction job
type ExtractionJob struct {
	JobID     string            `json:"job_id"`
	Status    ExtractionStatus  `json:"status"`
	Results   []ExtractionResult `json:"results,omitempty"`
	Error     string            `json:"error,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// ProcessingAuditEntry records a document processing event for DSGVO compliance
type ProcessingAuditEntry struct {
	ID                   string    `db:"id"`
	DocumentType         string    `db:"document_type"`
	ConsentTimestamp     time.Time `db:"consent_timestamp"`
	ConsentGivenBy       string    `db:"consent_given_by"`
	FieldsExtracted      []string  `db:"fields_extracted"`
	ProcessingDurationMs int       `db:"processing_duration_ms"`
	ImageDeletedAt       time.Time `db:"image_deleted_at"`
	CreatedAt            time.Time `db:"created_at"`
}
