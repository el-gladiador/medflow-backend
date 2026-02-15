package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// ScanHandler handles barcode/QR scan lookup endpoints
type ScanHandler struct {
	service *service.InventoryService
	logger  *logger.Logger
}

// NewScanHandler creates a new scan handler
func NewScanHandler(svc *service.InventoryService, log *logger.Logger) *ScanHandler {
	return &ScanHandler{
		service: svc,
		logger:  log,
	}
}

// LookupByBarcode looks up an item by barcode or article number
func (h *ScanHandler) LookupByBarcode(w http.ResponseWriter, r *http.Request) {
	barcode := chi.URLParam(r, "barcode")
	if barcode == "" {
		httputil.Error(w, errors.BadRequest("barcode is required"))
		return
	}
	// Reject excessively long input to avoid unnecessary DB queries
	if len(barcode) > 200 {
		httputil.Error(w, errors.BadRequest("barcode too long"))
		return
	}

	item, err := h.service.GetItemByBarcode(r.Context(), barcode)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, item)
}

// LookupByBatchNumber looks up a batch by batch number
func (h *ScanHandler) LookupByBatchNumber(w http.ResponseWriter, r *http.Request) {
	batchNumber := r.URL.Query().Get("batchNumber")
	if batchNumber == "" {
		httputil.Error(w, errors.BadRequest("batchNumber query parameter is required"))
		return
	}
	if len(batchNumber) > 200 {
		httputil.Error(w, errors.BadRequest("batchNumber too long"))
		return
	}

	result, err := h.service.GetBatchByBatchNumber(r.Context(), batchNumber)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}
