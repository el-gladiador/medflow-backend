package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// RetentionHandler handles retention policy endpoints
type RetentionHandler struct {
	service *service.RetentionService
	logger  *logger.Logger
}

// NewRetentionHandler creates a new retention handler
func NewRetentionHandler(svc *service.RetentionService, log *logger.Logger) *RetentionHandler {
	return &RetentionHandler{
		service: svc,
		logger:  log,
	}
}

// List lists all retention policies
// GET /retention-policies
func (h *RetentionHandler) List(w http.ResponseWriter, r *http.Request) {
	policies, err := h.service.List(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list retention policies")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, policies)
}

// Create creates a new retention policy
// POST /retention-policies
func (h *RetentionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var policy repository.RetentionPolicy
	if err := httputil.DecodeJSON(r, &policy); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.Create(r.Context(), &policy); err != nil {
		h.logger.Error().Err(err).Str("entity_type", policy.EntityType).Msg("failed to create retention policy")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, policy)
}

// Update updates a retention policy
// PUT /retention-policies/{id}
func (h *RetentionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var policy repository.RetentionPolicy
	if err := httputil.DecodeJSON(r, &policy); err != nil {
		httputil.Error(w, err)
		return
	}

	policy.ID = id
	if err := h.service.Update(r.Context(), &policy); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to update retention policy")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, policy)
}

// Delete soft-deletes a retention policy
// DELETE /retention-policies/{id}
func (h *RetentionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.Delete(r.Context(), id); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to delete retention policy")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}
