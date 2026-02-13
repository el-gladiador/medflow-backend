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

// ReprocessingHandler handles sterilization and reprocessing endpoints (KRINKO compliance)
type ReprocessingHandler struct {
	service *service.ReprocessingService
	logger  *logger.Logger
}

// NewReprocessingHandler creates a new reprocessing handler
func NewReprocessingHandler(svc *service.ReprocessingService, log *logger.Logger) *ReprocessingHandler {
	return &ReprocessingHandler{
		service: svc,
		logger:  log,
	}
}

// --- Sterilization Batch Handlers ---

// CreateBatch creates a new sterilization batch
// POST /sterilization/batches
func (h *ReprocessingHandler) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var batch repository.SterilizationBatch
	if err := httputil.DecodeJSON(r, &batch); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.CreateBatch(r.Context(), &batch); err != nil {
		h.logger.Error().Err(err).Str("batch_number", batch.BatchNumber).Msg("failed to create sterilization batch")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, batch)
}

// ListBatches lists sterilization batches with pagination
// GET /sterilization/batches
func (h *ReprocessingHandler) ListBatches(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	batches, total, err := h.service.ListBatches(r.Context(), page, perPage)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list sterilization batches")
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, batches, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetBatch gets a sterilization batch by ID
// GET /sterilization/batches/{id}
func (h *ReprocessingHandler) GetBatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	batch, err := h.service.GetBatch(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, batch)
}

// UpdateBatch updates a sterilization batch
// PUT /sterilization/batches/{id}
func (h *ReprocessingHandler) UpdateBatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var batch repository.SterilizationBatch
	if err := httputil.DecodeJSON(r, &batch); err != nil {
		httputil.Error(w, err)
		return
	}

	batch.ID = id
	if err := h.service.UpdateBatch(r.Context(), &batch); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to update sterilization batch")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, batch)
}

// DeleteBatch soft-deletes a sterilization batch
// DELETE /sterilization/batches/{id}
func (h *ReprocessingHandler) DeleteBatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteBatch(r.Context(), id); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to delete sterilization batch")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// --- Reprocessing Cycle Handlers ---

// CreateCycle creates a new reprocessing cycle (auto-increments cycle_number)
// POST /reprocessing/items/{itemId}/cycles
func (h *ReprocessingHandler) CreateCycle(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")

	var cycle repository.ReprocessingCycle
	if err := httputil.DecodeJSON(r, &cycle); err != nil {
		httputil.Error(w, err)
		return
	}

	cycle.ItemID = itemID
	if err := h.service.CreateCycle(r.Context(), &cycle); err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to create reprocessing cycle")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, cycle)
}

// ListCyclesByItem lists reprocessing cycles for an item
// GET /reprocessing/items/{itemId}/cycles
func (h *ReprocessingHandler) ListCyclesByItem(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")

	cycles, err := h.service.ListCyclesByItem(r.Context(), itemID)
	if err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to list reprocessing cycles")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, cycles)
}

// UpdateCycle updates a reprocessing cycle
// PUT /reprocessing/cycles/{id}
func (h *ReprocessingHandler) UpdateCycle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var cycle repository.ReprocessingCycle
	if err := httputil.DecodeJSON(r, &cycle); err != nil {
		httputil.Error(w, err)
		return
	}

	cycle.ID = id
	if err := h.service.UpdateCycle(r.Context(), &cycle); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to update reprocessing cycle")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, cycle)
}

// DeleteCycle soft-deletes a reprocessing cycle
// DELETE /reprocessing/cycles/{id}
func (h *ReprocessingHandler) DeleteCycle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteCycle(r.Context(), id); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to delete reprocessing cycle")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}
