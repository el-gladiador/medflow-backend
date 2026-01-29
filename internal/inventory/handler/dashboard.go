package handler

import (
	"net/http"

	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// DashboardHandler handles dashboard endpoints
type DashboardHandler struct {
	service *service.InventoryService
	logger  *logger.Logger
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(svc *service.InventoryService, log *logger.Logger) *DashboardHandler {
	return &DashboardHandler{
		service: svc,
		logger:  log,
	}
}

// GetStats returns dashboard statistics
func (h *DashboardHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetDashboardStats(r.Context())
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, stats)
}
