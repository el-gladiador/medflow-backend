package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// ExportHandler handles PDF export endpoints
type ExportHandler struct {
	service *service.InventoryService
	logger  *logger.Logger
}

// NewExportHandler creates a new export handler
func NewExportHandler(svc *service.InventoryService, log *logger.Logger) *ExportHandler {
	return &ExportHandler{
		service: svc,
		logger:  log,
	}
}

// ExportInventoryRegister generates and serves the inventory register PDF
func (h *ExportHandler) ExportInventoryRegister(w http.ResponseWriter, r *http.Request) {
	pdfBytes, err := h.service.ExportInventoryRegister(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to generate inventory register PDF")
		http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("inventory-register-%s.pdf", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.Write(pdfBytes)
}

// ExportBestandsverzeichnis generates and serves the Bestandsverzeichnis PDF (MPBetreibV §14)
func (h *ExportHandler) ExportBestandsverzeichnis(w http.ResponseWriter, r *http.Request) {
	pdfBytes, err := h.service.ExportBestandsverzeichnis(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to generate Bestandsverzeichnis PDF")
		http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("bestandsverzeichnis-%s.pdf", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.Write(pdfBytes)
}

// ExportGefahrstoffverzeichnis generates and serves the hazardous substance register PDF
func (h *ExportHandler) ExportGefahrstoffverzeichnis(w http.ResponseWriter, r *http.Request) {
	pdfBytes, err := h.service.ExportGefahrstoffverzeichnis(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to generate Gefahrstoffverzeichnis PDF")
		http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("gefahrstoffverzeichnis-%s.pdf", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.Write(pdfBytes)
}
