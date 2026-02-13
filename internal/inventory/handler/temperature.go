package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// TemperatureHandler handles temperature monitoring endpoints
type TemperatureHandler struct {
	service *service.InventoryService
	logger  *logger.Logger
}

// NewTemperatureHandler creates a new temperature handler
func NewTemperatureHandler(svc *service.InventoryService, log *logger.Logger) *TemperatureHandler {
	return &TemperatureHandler{
		service: svc,
		logger:  log,
	}
}

// RecordTemperature records a manual temperature reading
func (h *TemperatureHandler) RecordTemperature(w http.ResponseWriter, r *http.Request) {
	cabinetID := chi.URLParam(r, "id")

	var req struct {
		TemperatureCelsius float64 `json:"temperature_celsius"`
		Notes              *string `json:"notes,omitempty"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	userID := r.Header.Get("X-User-ID")
	var recordedBy *string
	if userID != "" {
		recordedBy = &userID
	}

	reading, err := h.service.RecordTemperature(r.Context(), cabinetID, req.TemperatureCelsius, "manual", recordedBy, req.Notes)
	if err != nil {
		h.logger.Error().Err(err).Str("cabinet_id", cabinetID).Msg("failed to record temperature")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, reading)
}

// ListReadings lists temperature readings for a cabinet
func (h *TemperatureHandler) ListReadings(w http.ResponseWriter, r *http.Request) {
	cabinetID := chi.URLParam(r, "id")

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 {
		perPage = 50
	}

	var from, to *time.Time
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = &t
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			endOfDay := t.Add(24*time.Hour - time.Second)
			to = &endOfDay
		}
	}

	readings, total, err := h.service.ListTemperatureReadings(r.Context(), cabinetID, from, to, page, perPage)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]interface{}{
		"data":     readings,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// Webhook handles incoming temperature readings from external sensors
func (h *TemperatureHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CabinetID          string    `json:"cabinet_id"`
		TemperatureCelsius float64   `json:"temperature_celsius"`
		RecordedAt         *time.Time `json:"recorded_at,omitempty"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if req.CabinetID == "" {
		http.Error(w, "cabinet_id required", http.StatusBadRequest)
		return
	}

	reading, err := h.service.RecordTemperature(r.Context(), req.CabinetID, req.TemperatureCelsius, "webhook", nil, nil)
	if err != nil {
		h.logger.Error().Err(err).Str("cabinet_id", req.CabinetID).Msg("failed to record webhook temperature")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, reading)
}
