package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// BioSafetyHandler handles biological safety endpoints (BioStoffV compliance)
type BioSafetyHandler struct {
	service *service.BioSafetyService
	logger  *logger.Logger
}

// NewBioSafetyHandler creates a new biosafety handler
func NewBioSafetyHandler(svc *service.BioSafetyService, log *logger.Logger) *BioSafetyHandler {
	return &BioSafetyHandler{
		service: svc,
		logger:  log,
	}
}

// --- Risk Assessment Handlers ---

// CreateAssessment creates a new bio risk assessment
// POST /bio-safety/assessments
func (h *BioSafetyHandler) CreateAssessment(w http.ResponseWriter, r *http.Request) {
	var assessment repository.BioRiskAssessment
	if err := httputil.DecodeJSON(r, &assessment); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.CreateAssessment(r.Context(), &assessment); err != nil {
		h.logger.Error().Err(err).Str("item_id", assessment.ItemID).Msg("failed to create bio risk assessment")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, assessment)
}

// ListAssessmentsByItem lists risk assessments for a specific item
// GET /bio-safety/items/{itemId}/assessments
func (h *BioSafetyHandler) ListAssessmentsByItem(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")

	assessments, err := h.service.ListAssessmentsByItem(r.Context(), itemID)
	if err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to list bio risk assessments")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, assessments)
}

// UpdateAssessment updates a bio risk assessment
// PUT /bio-safety/assessments/{id}
func (h *BioSafetyHandler) UpdateAssessment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var assessment repository.BioRiskAssessment
	if err := httputil.DecodeJSON(r, &assessment); err != nil {
		httputil.Error(w, err)
		return
	}

	assessment.ID = id
	if err := h.service.UpdateAssessment(r.Context(), &assessment); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to update bio risk assessment")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, assessment)
}

// DeleteAssessment soft-deletes a bio risk assessment
// DELETE /bio-safety/assessments/{id}
func (h *BioSafetyHandler) DeleteAssessment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteAssessment(r.Context(), id); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to delete bio risk assessment")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// --- Bio Training Handlers ---

// CreateTraining creates a new bio training record
// POST /bio-safety/trainings
func (h *BioSafetyHandler) CreateTraining(w http.ResponseWriter, r *http.Request) {
	var training repository.BioTraining
	if err := httputil.DecodeJSON(r, &training); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.CreateTraining(r.Context(), &training); err != nil {
		h.logger.Error().Err(err).Msg("failed to create bio training")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, training)
}

// ListTrainings lists all bio trainings
// GET /bio-safety/trainings
func (h *BioSafetyHandler) ListTrainings(w http.ResponseWriter, r *http.Request) {
	trainings, err := h.service.ListTrainings(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list bio trainings")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, trainings)
}

// UpdateTraining updates a bio training record
// PUT /bio-safety/trainings/{id}
func (h *BioSafetyHandler) UpdateTraining(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var training repository.BioTraining
	if err := httputil.DecodeJSON(r, &training); err != nil {
		httputil.Error(w, err)
		return
	}

	training.ID = id
	if err := h.service.UpdateTraining(r.Context(), &training); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to update bio training")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, training)
}

// DeleteTraining soft-deletes a bio training record
// DELETE /bio-safety/trainings/{id}
func (h *BioSafetyHandler) DeleteTraining(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteTraining(r.Context(), id); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to delete bio training")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}
