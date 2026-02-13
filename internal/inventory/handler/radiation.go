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

// RadiationHandler handles radiation protection endpoints (StrlSchV/RoV compliance)
type RadiationHandler struct {
	service *service.RadiationService
	logger  *logger.Logger
}

// NewRadiationHandler creates a new radiation handler
func NewRadiationHandler(svc *service.RadiationService, log *logger.Logger) *RadiationHandler {
	return &RadiationHandler{
		service: svc,
		logger:  log,
	}
}

// --- Radiation Device Handlers ---

// CreateDevice creates a new radiation device
// POST /radiation/devices
func (h *RadiationHandler) CreateDevice(w http.ResponseWriter, r *http.Request) {
	var device repository.RadiationDevice
	if err := httputil.DecodeJSON(r, &device); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.CreateDevice(r.Context(), &device); err != nil {
		h.logger.Error().Err(err).Str("item_id", device.ItemID).Msg("failed to create radiation device")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, device)
}

// ListDevices lists all radiation devices
// GET /radiation/devices
func (h *RadiationHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.service.ListDevices(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list radiation devices")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, devices)
}

// GetDevice gets a radiation device by ID
// GET /radiation/devices/{id}
func (h *RadiationHandler) GetDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	device, err := h.service.GetDevice(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, device)
}

// UpdateDevice updates a radiation device
// PUT /radiation/devices/{id}
func (h *RadiationHandler) UpdateDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var device repository.RadiationDevice
	if err := httputil.DecodeJSON(r, &device); err != nil {
		httputil.Error(w, err)
		return
	}

	device.ID = id
	if err := h.service.UpdateDevice(r.Context(), &device); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to update radiation device")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, device)
}

// DeleteDevice soft-deletes a radiation device
// DELETE /radiation/devices/{id}
func (h *RadiationHandler) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteDevice(r.Context(), id); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to delete radiation device")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// --- Constancy Test Handlers ---

// CreateTest creates a new constancy test for a device
// POST /radiation/devices/{deviceId}/constancy-tests
func (h *RadiationHandler) CreateTest(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceId")

	var test repository.ConstancyTest
	if err := httputil.DecodeJSON(r, &test); err != nil {
		httputil.Error(w, err)
		return
	}

	test.DeviceID = deviceID
	if err := h.service.CreateTest(r.Context(), &test); err != nil {
		h.logger.Error().Err(err).Str("device_id", deviceID).Msg("failed to create constancy test")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, test)
}

// ListTests lists constancy tests for a device
// GET /radiation/devices/{deviceId}/constancy-tests
func (h *RadiationHandler) ListTests(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceId")

	tests, err := h.service.ListTestsByDevice(r.Context(), deviceID)
	if err != nil {
		h.logger.Error().Err(err).Str("device_id", deviceID).Msg("failed to list constancy tests")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, tests)
}

// --- Expert Inspection Handlers ---

// CreateExpertInspection creates a new expert inspection for a device
// POST /radiation/devices/{deviceId}/expert-inspections
func (h *RadiationHandler) CreateExpertInspection(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceId")

	var insp repository.ExpertInspection
	if err := httputil.DecodeJSON(r, &insp); err != nil {
		httputil.Error(w, err)
		return
	}

	insp.DeviceID = deviceID
	if err := h.service.CreateExpertInspection(r.Context(), &insp); err != nil {
		h.logger.Error().Err(err).Str("device_id", deviceID).Msg("failed to create expert inspection")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, insp)
}

// ListExpertInspections lists expert inspections for a device
// GET /radiation/devices/{deviceId}/expert-inspections
func (h *RadiationHandler) ListExpertInspections(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceId")

	inspections, err := h.service.ListExpertInspectionsByDevice(r.Context(), deviceID)
	if err != nil {
		h.logger.Error().Err(err).Str("device_id", deviceID).Msg("failed to list expert inspections")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, inspections)
}

// --- Staff Radiation Certification Handlers ---

// CreateCertification creates a new staff radiation certification
// POST /radiation/certifications
func (h *RadiationHandler) CreateCertification(w http.ResponseWriter, r *http.Request) {
	var cert repository.StaffRadiationCertification
	if err := httputil.DecodeJSON(r, &cert); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.CreateCertification(r.Context(), &cert); err != nil {
		h.logger.Error().Err(err).Str("employee_id", cert.EmployeeID).Msg("failed to create radiation certification")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, cert)
}

// ListCertifications lists all staff radiation certifications
// GET /radiation/certifications
func (h *RadiationHandler) ListCertifications(w http.ResponseWriter, r *http.Request) {
	certs, err := h.service.ListCertifications(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list radiation certifications")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, certs)
}

// UpdateCertification updates a staff radiation certification
// PUT /radiation/certifications/{id}
func (h *RadiationHandler) UpdateCertification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var cert repository.StaffRadiationCertification
	if err := httputil.DecodeJSON(r, &cert); err != nil {
		httputil.Error(w, err)
		return
	}

	cert.ID = id
	if err := h.service.UpdateCertification(r.Context(), &cert); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to update radiation certification")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, cert)
}

// DeleteCertification soft-deletes a staff radiation certification
// DELETE /radiation/certifications/{id}
func (h *RadiationHandler) DeleteCertification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteCertification(r.Context(), id); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to delete radiation certification")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// --- Dosimetry Record Handlers ---

// CreateDosimetryRecord creates a new dosimetry record
// POST /radiation/dosimetry
func (h *RadiationHandler) CreateDosimetryRecord(w http.ResponseWriter, r *http.Request) {
	var record repository.DosimetryRecord
	if err := httputil.DecodeJSON(r, &record); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.CreateDosimetryRecord(r.Context(), &record); err != nil {
		h.logger.Error().Err(err).Str("employee_id", record.EmployeeID).Msg("failed to create dosimetry record")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, record)
}

// ListAllDosimetry lists all dosimetry records with pagination
// GET /radiation/dosimetry
func (h *RadiationHandler) ListAllDosimetry(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	records, total, err := h.service.ListAllDosimetry(r.Context(), page, perPage)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list dosimetry records")
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, records, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// ListDosimetryByEmployee lists dosimetry records for a specific employee
// GET /radiation/dosimetry/employee/{employeeId}
func (h *RadiationHandler) ListDosimetryByEmployee(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "employeeId")

	records, err := h.service.ListDosimetryByEmployee(r.Context(), employeeID)
	if err != nil {
		h.logger.Error().Err(err).Str("employee_id", employeeID).Msg("failed to list dosimetry records for employee")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, records)
}
