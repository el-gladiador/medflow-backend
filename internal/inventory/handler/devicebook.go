package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// DeviceBookHandler handles Medizinproduktebuch endpoints
type DeviceBookHandler struct {
	service *service.InventoryService
	logger  *logger.Logger
}

// NewDeviceBookHandler creates a new device book handler
func NewDeviceBookHandler(svc *service.InventoryService, log *logger.Logger) *DeviceBookHandler {
	return &DeviceBookHandler{
		service: svc,
		logger:  log,
	}
}

// Inspection handlers

// ListInspections lists inspections for a device
func (h *DeviceBookHandler) ListInspections(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	inspections, err := h.service.ListInspections(r.Context(), itemID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, inspections)
}

// CreateInspection creates a new inspection
func (h *DeviceBookHandler) CreateInspection(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	var insp repository.DeviceInspection
	if err := httputil.DecodeJSON(r, &insp); err != nil {
		httputil.Error(w, err)
		return
	}

	insp.ItemID = itemID
	if err := h.service.CreateInspection(r.Context(), &insp); err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to create inspection")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, insp)
}

// UpdateInspection updates an inspection
func (h *DeviceBookHandler) UpdateInspection(w http.ResponseWriter, r *http.Request) {
	inspID := chi.URLParam(r, "inspId")

	var insp repository.DeviceInspection
	if err := httputil.DecodeJSON(r, &insp); err != nil {
		httputil.Error(w, err)
		return
	}

	insp.ID = inspID
	if err := h.service.UpdateInspection(r.Context(), &insp); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, insp)
}

// DeleteInspection deletes an inspection
func (h *DeviceBookHandler) DeleteInspection(w http.ResponseWriter, r *http.Request) {
	inspID := chi.URLParam(r, "inspId")

	if err := h.service.DeleteInspection(r.Context(), inspID); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// Training handlers

// ListTrainings lists trainings for a device
func (h *DeviceBookHandler) ListTrainings(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	trainings, err := h.service.ListTrainings(r.Context(), itemID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, trainings)
}

// CreateTraining creates a new training
func (h *DeviceBookHandler) CreateTraining(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	var tr repository.DeviceTraining
	if err := httputil.DecodeJSON(r, &tr); err != nil {
		httputil.Error(w, err)
		return
	}

	tr.ItemID = itemID
	if err := h.service.CreateTraining(r.Context(), &tr); err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to create training")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, tr)
}

// UpdateTraining updates a training
func (h *DeviceBookHandler) UpdateTraining(w http.ResponseWriter, r *http.Request) {
	trID := chi.URLParam(r, "trId")

	var tr repository.DeviceTraining
	if err := httputil.DecodeJSON(r, &tr); err != nil {
		httputil.Error(w, err)
		return
	}

	tr.ID = trID
	if err := h.service.UpdateTraining(r.Context(), &tr); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, tr)
}

// DeleteTraining deletes a training
func (h *DeviceBookHandler) DeleteTraining(w http.ResponseWriter, r *http.Request) {
	trID := chi.URLParam(r, "trId")

	if err := h.service.DeleteTraining(r.Context(), trID); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// Incident handlers

// ListIncidents lists incidents for a device
func (h *DeviceBookHandler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	incidents, err := h.service.ListIncidents(r.Context(), itemID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, incidents)
}

// CreateIncident creates a new incident
func (h *DeviceBookHandler) CreateIncident(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	var inc repository.DeviceIncident
	if err := httputil.DecodeJSON(r, &inc); err != nil {
		httputil.Error(w, err)
		return
	}

	inc.ItemID = itemID
	if err := h.service.CreateIncident(r.Context(), &inc); err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to create incident")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, inc)
}

// UpdateIncident updates an incident
func (h *DeviceBookHandler) UpdateIncident(w http.ResponseWriter, r *http.Request) {
	incID := chi.URLParam(r, "incId")

	var inc repository.DeviceIncident
	if err := httputil.DecodeJSON(r, &inc); err != nil {
		httputil.Error(w, err)
		return
	}

	inc.ID = incID
	if err := h.service.UpdateIncident(r.Context(), &inc); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, inc)
}

// DeleteIncident deletes an incident
func (h *DeviceBookHandler) DeleteIncident(w http.ResponseWriter, r *http.Request) {
	incID := chi.URLParam(r, "incId")

	if err := h.service.DeleteIncident(r.Context(), incID); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}
