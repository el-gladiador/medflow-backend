package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// AlertHandler handles alert endpoints
type AlertHandler struct {
	repo   *repository.AlertRepository
	logger *logger.Logger
}

// NewAlertHandler creates a new alert handler
func NewAlertHandler(repo *repository.AlertRepository, log *logger.Logger) *AlertHandler {
	return &AlertHandler{
		repo:   repo,
		logger: log,
	}
}

// List lists alerts
func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	var acknowledged *bool
	if ack := r.URL.Query().Get("acknowledged"); ack != "" {
		a := ack == "true"
		acknowledged = &a
	}

	alertType := r.URL.Query().Get("type")

	alerts, total, err := h.repo.List(r.Context(), acknowledged, alertType, page, perPage)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, alerts, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// Acknowledge acknowledges an alert
func (h *AlertHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-ID")

	if err := h.repo.Acknowledge(r.Context(), id, userID); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}
