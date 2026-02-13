package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// AuditHandler handles audit trail HTTP endpoints
type AuditHandler struct {
	auditService *service.AuditService
	logger       *logger.Logger
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(svc *service.AuditService, log *logger.Logger) *AuditHandler {
	return &AuditHandler{
		auditService: svc,
		logger:       log,
	}
}

// GetItemAudit lists audit entries for a specific item
// GET /items/{id}/audit
func (h *AuditHandler) GetItemAudit(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	entries, total, err := h.auditService.ListByEntity(r.Context(), "item", itemID, page, perPage)
	if err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to list item audit entries")
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, entries, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// ListAudit lists all audit entries for the tenant
// GET /audit — supports query params: entity_type, from, to, page, per_page
func (h *AuditHandler) ListAudit(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	entityType := r.URL.Query().Get("entity_type")

	var from, to *time.Time

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = &t
		}
	}

	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = &t
		}
	}

	entries, total, err := h.auditService.ListByTenant(r.Context(), entityType, from, to, page, perPage)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list audit entries")
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, entries, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// ExportGoBD exports audit trail as JSON lines for GoBD compliance
// GET /export/gobd — supports query params: from, to
func (h *AuditHandler) ExportGoBD(w http.ResponseWriter, r *http.Request) {
	var from, to *time.Time

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = &t
		}
	}

	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = &t
		}
	}

	entries, err := h.auditService.ExportGoBD(r.Context(), from, to)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to export GoBD audit trail")
		httputil.Error(w, err)
		return
	}

	// Set headers for download
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=gobd_audit_export_%s.jsonl", time.Now().Format("2006-01-02")))
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			h.logger.Error().Err(err).Msg("failed to encode audit entry for export")
			return
		}
	}
}
