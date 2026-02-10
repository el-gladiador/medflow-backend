package handler

import (
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/docprocessing/domain"
	"github.com/medflow/medflow-backend/internal/docprocessing/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

const maxUploadSize = 20 << 20 // 20MB

// Handler handles HTTP requests for document extraction
type Handler struct {
	service *service.Service
	log     *logger.Logger
}

// NewHandler creates a new document extraction handler
func NewHandler(svc *service.Service, log *logger.Logger) *Handler {
	return &Handler{
		service: svc,
		log:     log,
	}
}

// Extract handles POST /documents/extract
// Accepts multipart form with:
// - file: the document image
// - document_type: one of personalausweis, reisepass, fuehrerschein, lebenslauf
// - consent_timestamp: ISO 8601 timestamp of consent
func (h *Handler) Extract(w http.ResponseWriter, r *http.Request) {
	// Limit request size
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		httputil.JSON(w, http.StatusBadRequest, map[string]string{
			"error": "File too large or invalid multipart form",
		})
		return
	}

	// Get document type
	docTypeStr := r.FormValue("document_type")
	docType := domain.DocumentType(docTypeStr)
	switch docType {
	case domain.DocumentTypePersonalausweis, domain.DocumentTypeReisepass,
		domain.DocumentTypeFuehrerschein, domain.DocumentTypeLebenslauf:
		// valid
	default:
		httputil.JSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid document_type. Must be one of: personalausweis, reisepass, fuehrerschein, lebenslauf",
		})
		return
	}

	// Parse consent timestamp
	consentStr := r.FormValue("consent_timestamp")
	consentTimestamp, err := time.Parse(time.RFC3339, consentStr)
	if err != nil {
		httputil.JSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid consent_timestamp. Must be RFC3339 format.",
		})
		return
	}

	// Get uploaded file
	file, _, err := r.FormFile("file")
	if err != nil {
		httputil.JSON(w, http.StatusBadRequest, map[string]string{
			"error": "Missing file in request",
		})
		return
	}
	defer file.Close()

	// Read file into memory (never to disk)
	imageData, err := io.ReadAll(file)
	if err != nil {
		httputil.JSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to read uploaded file",
		})
		return
	}

	// Get user ID from context
	userID := httputil.GetUserID(r.Context())

	// Start extraction (imageData will be zeroed by the service)
	job, err := h.service.StartExtraction(r.Context(), imageData, docType, consentTimestamp, userID)
	if err != nil {
		h.log.Error().Err(err).Msg("extraction failed")
		httputil.JSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Document processing failed",
		})
		return
	}

	httputil.JSON(w, http.StatusOK, job)
}

// GetResult handles GET /documents/extract/{jobId}
// Returns the extraction job status and results
func (h *Handler) GetResult(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	if jobID == "" {
		httputil.JSON(w, http.StatusBadRequest, map[string]string{
			"error": "Missing jobId parameter",
		})
		return
	}

	job := h.service.GetJob(jobID)
	if job == nil {
		httputil.JSON(w, http.StatusNotFound, map[string]string{
			"error": "Job not found",
		})
		return
	}

	httputil.JSON(w, http.StatusOK, job)
}
