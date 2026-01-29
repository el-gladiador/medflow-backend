package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// BatchHandler handles batch endpoints
type BatchHandler struct {
	service *service.InventoryService
	logger  *logger.Logger
}

// NewBatchHandler creates a new batch handler
func NewBatchHandler(svc *service.InventoryService, log *logger.Logger) *BatchHandler {
	return &BatchHandler{
		service: svc,
		logger:  log,
	}
}

// ListByItem lists batches for an item
func (h *BatchHandler) ListByItem(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	batches, err := h.service.ListBatchesByItem(r.Context(), itemID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, batches)
}

// Get gets a batch by ID
func (h *BatchHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	batch, err := h.service.GetBatch(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, batch)
}

// Create creates a new batch
func (h *BatchHandler) Create(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	var batch repository.InventoryBatch
	if err := httputil.DecodeJSON(r, &batch); err != nil {
		httputil.Error(w, err)
		return
	}

	batch.ItemID = itemID
	batch.IsActive = true
	if err := h.service.CreateBatch(r.Context(), &batch); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, batch)
}

// Update updates a batch
func (h *BatchHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var batch repository.InventoryBatch
	if err := httputil.DecodeJSON(r, &batch); err != nil {
		httputil.Error(w, err)
		return
	}

	batch.ID = id
	if err := h.service.UpdateBatch(r.Context(), &batch); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, batch)
}

// Delete deletes a batch
func (h *BatchHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteBatch(r.Context(), id); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// AdjustStock adjusts stock for a batch
func (h *BatchHandler) AdjustStock(w http.ResponseWriter, r *http.Request) {
	batchID := chi.URLParam(r, "id")

	var req struct {
		Quantity int    `json:"quantity" validate:"required"`
		Type     string `json:"type" validate:"required,oneof=add deduct adjust"`
		Reason   string `json:"reason"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := httputil.Validate(&req); err != nil {
		httputil.Error(w, err)
		return
	}

	userID := r.Header.Get("X-User-ID")
	userName := r.Header.Get("X-User-Email")

	adj, err := h.service.AdjustStock(r.Context(), batchID, req.Quantity, req.Type, req.Reason, userID, userName)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, adj)
}
