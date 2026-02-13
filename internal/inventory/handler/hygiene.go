package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// HygieneHandler handles hygiene plan and inspection endpoints (IfSG compliance)
type HygieneHandler struct {
	service *service.HygieneService
	logger  *logger.Logger
}

// NewHygieneHandler creates a new hygiene handler
func NewHygieneHandler(svc *service.HygieneService, log *logger.Logger) *HygieneHandler {
	return &HygieneHandler{
		service: svc,
		logger:  log,
	}
}

// --- Hygiene Plan Handlers ---

// CreatePlan creates a new hygiene plan
// POST /hygiene/plans
func (h *HygieneHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var plan repository.HygienePlan
	if err := httputil.DecodeJSON(r, &plan); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.CreatePlan(r.Context(), &plan); err != nil {
		h.logger.Error().Err(err).Str("title", plan.Title).Msg("failed to create hygiene plan")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, plan)
}

// ListPlans lists hygiene plans with pagination and optional filters
// GET /hygiene/plans?status=&category=&page=&per_page=
func (h *HygieneHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	status := r.URL.Query().Get("status")
	category := r.URL.Query().Get("category")

	plans, total, err := h.service.ListPlans(r.Context(), status, category, page, perPage)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list hygiene plans")
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, plans, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetPlan gets a hygiene plan by ID
// GET /hygiene/plans/{id}
func (h *HygieneHandler) GetPlan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	plan, err := h.service.GetPlan(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, plan)
}

// UpdatePlan updates a hygiene plan
// PUT /hygiene/plans/{id}
func (h *HygieneHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var plan repository.HygienePlan
	if err := httputil.DecodeJSON(r, &plan); err != nil {
		httputil.Error(w, err)
		return
	}

	plan.ID = id
	if err := h.service.UpdatePlan(r.Context(), &plan); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to update hygiene plan")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, plan)
}

// DeletePlan soft-deletes a hygiene plan
// DELETE /hygiene/plans/{id}
func (h *HygieneHandler) DeletePlan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeletePlan(r.Context(), id); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to delete hygiene plan")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// --- Hygiene Inspection Handlers ---

// CreateInspection creates a new hygiene inspection
// POST /hygiene/inspections
func (h *HygieneHandler) CreateInspection(w http.ResponseWriter, r *http.Request) {
	var inspection repository.HygieneInspection
	if err := httputil.DecodeJSON(r, &inspection); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.CreateInspection(r.Context(), &inspection); err != nil {
		h.logger.Error().Err(err).Msg("failed to create hygiene inspection")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, inspection)
}

// ListInspections lists hygiene inspections with pagination and optional plan_id filter
// GET /hygiene/inspections?plan_id=&page=&per_page=
func (h *HygieneHandler) ListInspections(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	planID := r.URL.Query().Get("plan_id")

	inspections, total, err := h.service.ListInspections(r.Context(), planID, page, perPage)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list hygiene inspections")
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, inspections, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetInspection gets a hygiene inspection by ID
// GET /hygiene/inspections/{id}
func (h *HygieneHandler) GetInspection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	inspection, err := h.service.GetInspection(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, inspection)
}

// UpdateInspection updates a hygiene inspection
// PUT /hygiene/inspections/{id}
func (h *HygieneHandler) UpdateInspection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var inspection repository.HygieneInspection
	if err := httputil.DecodeJSON(r, &inspection); err != nil {
		httputil.Error(w, err)
		return
	}

	inspection.ID = id
	if err := h.service.UpdateInspection(r.Context(), &inspection); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to update hygiene inspection")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, inspection)
}

// DeleteInspection soft-deletes a hygiene inspection
// DELETE /hygiene/inspections/{id}
func (h *HygieneHandler) DeleteInspection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteInspection(r.Context(), id); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to delete hygiene inspection")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}
