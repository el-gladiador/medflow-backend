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

// ItemHandler handles item endpoints
type ItemHandler struct {
	service *service.InventoryService
	logger  *logger.Logger
}

// NewItemHandler creates a new item handler
func NewItemHandler(svc *service.InventoryService, log *logger.Logger) *ItemHandler {
	return &ItemHandler{
		service: svc,
		logger:  log,
	}
}

// List lists inventory items
func (h *ItemHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	category := r.URL.Query().Get("category")

	items, total, err := h.service.ListItems(r.Context(), page, perPage, category)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, items, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// Get gets an item by ID
func (h *ItemHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	item, err := h.service.GetItem(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, item)
}

// Create creates a new item
func (h *ItemHandler) Create(w http.ResponseWriter, r *http.Request) {
	var item repository.InventoryItem
	if err := httputil.DecodeJSON(r, &item); err != nil {
		httputil.Error(w, err)
		return
	}

	item.IsActive = true
	if err := h.service.CreateItem(r.Context(), &item); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, item)
}

// Update updates an item
func (h *ItemHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var item repository.InventoryItem
	if err := httputil.DecodeJSON(r, &item); err != nil {
		httputil.Error(w, err)
		return
	}

	item.ID = id
	if err := h.service.UpdateItem(r.Context(), &item); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, item)
}

// Delete deletes an item
func (h *ItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteItem(r.Context(), id); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}
