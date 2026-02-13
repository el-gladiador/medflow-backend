package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

const (
	maxUploadSize = 20 << 20 // 20MB
	uploadBaseDir = "uploads/inventory"
)

var allowedMimeTypes = map[string]bool{
	"application/pdf": true,
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
}

// ComplianceHandler handles compliance endpoints
type ComplianceHandler struct {
	service *service.InventoryService
	logger  *logger.Logger
}

// NewComplianceHandler creates a new compliance handler
func NewComplianceHandler(svc *service.InventoryService, log *logger.Logger) *ComplianceHandler {
	return &ComplianceHandler{
		service: svc,
		logger:  log,
	}
}

// GetHazardousDetails gets hazardous substance details for an item
func (h *ComplianceHandler) GetHazardousDetails(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	details, err := h.service.GetHazardousDetails(r.Context(), itemID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, details)
}

// UpsertHazardousDetails creates or updates hazardous substance details
func (h *ComplianceHandler) UpsertHazardousDetails(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	var detail repository.HazardousSubstanceDetail
	if err := httputil.DecodeJSON(r, &detail); err != nil {
		httputil.Error(w, err)
		return
	}

	detail.ItemID = itemID
	if err := h.service.UpsertHazardousDetails(r.Context(), &detail); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, detail)
}

// DeleteHazardousDetails deletes hazardous substance details for an item
func (h *ComplianceHandler) DeleteHazardousDetails(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	if err := h.service.DeleteHazardousDetails(r.Context(), itemID); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// ListDocuments lists documents for an item
func (h *ComplianceHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	docs, err := h.service.ListItemDocuments(r.Context(), itemID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, docs)
}

// UploadDocument uploads a document for an item
func (h *ComplianceHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "File too large (max 20MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	documentType := r.FormValue("document_type")
	if documentType != "sdb" && documentType != "manual" && documentType != "certificate" {
		http.Error(w, "Invalid document_type (must be sdb, manual, or certificate)", http.StatusBadRequest)
		return
	}

	// Validate MIME type
	mimeType := header.Header.Get("Content-Type")
	if !allowedMimeTypes[mimeType] {
		http.Error(w, "Unsupported file type (PDF and images only)", http.StatusBadRequest)
		return
	}

	// Build storage path
	tenantID, err := tenant.TenantID(r.Context())
	if err != nil {
		httputil.Error(w, err)
		return
	}

	fileUUID := uuid.New().String()
	ext := filepath.Ext(header.Filename)
	storedName := fileUUID + ext
	relPath := filepath.Join(tenantID, itemID, storedName)
	absPath := filepath.Join(uploadBaseDir, relPath)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(absPath), 0750); err != nil {
		h.logger.Error().Err(err).Msg("failed to create upload directory")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Write file
	dst, err := os.Create(absPath)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to create file")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		h.logger.Error().Err(err).Msg("failed to write file")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Save metadata
	size := int(header.Size)
	userID := r.Header.Get("X-User-ID")
	var uploadedBy *string
	if userID != "" {
		uploadedBy = &userID
	}

	doc := &repository.ItemDocument{
		ItemID:        itemID,
		DocumentType:  documentType,
		FileName:      header.Filename,
		FilePath:      relPath,
		FileSizeBytes: &size,
		MimeType:      &mimeType,
		UploadedBy:    uploadedBy,
	}

	if err := h.service.CreateItemDocument(r.Context(), doc); err != nil {
		// Clean up file on DB error
		os.Remove(absPath)
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, doc)
}

// DeleteDocument deletes a document
func (h *ComplianceHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Get doc to find file path
	doc, err := h.service.GetItemDocument(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.DeleteItemDocument(r.Context(), id); err != nil {
		httputil.Error(w, err)
		return
	}

	// Remove file (best-effort)
	absPath := filepath.Join(uploadBaseDir, doc.FilePath)
	os.Remove(absPath)

	httputil.NoContent(w)
}

// DownloadDocument serves a document file
func (h *ComplianceHandler) DownloadDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	doc, err := h.service.GetItemDocument(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	absPath := filepath.Join(uploadBaseDir, doc.FilePath)

	// Prevent path traversal
	absBase, _ := filepath.Abs(uploadBaseDir)
	absPath, err = filepath.Abs(absPath)
	if err != nil || !strings.HasPrefix(absPath, absBase) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	f, err := os.Open(absPath)
	if err != nil {
		h.logger.Error().Err(err).Str("path", absPath).Msg("file not found on disk")
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	if doc.MimeType != nil {
		w.Header().Set("Content-Type", *doc.MimeType)
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", doc.FileName))

	io.Copy(w, f)
}
