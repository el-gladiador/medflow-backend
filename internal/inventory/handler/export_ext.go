package handler

import (
	"net/http"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// DataPortabilityExport represents the complete DSGVO data portability export for a tenant
type DataPortabilityExport struct {
	ExportedAt             time.Time                          `json:"exported_at"`
	Format                 string                             `json:"format"`
	Items                  interface{}                        `json:"items,omitempty"`
	HazardousItems         interface{}                        `json:"hazardous_items,omitempty"`
	BioRiskAssessments     interface{}                        `json:"bio_risk_assessments,omitempty"`
	BioTrainings           interface{}                        `json:"bio_trainings,omitempty"`
	RetentionPolicies      interface{}                        `json:"retention_policies,omitempty"`
	SterilizationBatches   interface{}                        `json:"sterilization_batches,omitempty"`
	HygienePlans           interface{}                        `json:"hygiene_plans,omitempty"`
	HygieneInspections     interface{}                        `json:"hygiene_inspections,omitempty"`
	RadiationDevices       interface{}                        `json:"radiation_devices,omitempty"`
	RadiationCertifications interface{}                       `json:"radiation_certifications,omitempty"`
	DosimetryRecords       interface{}                        `json:"dosimetry_records,omitempty"`
}

// DataPortabilityHandler handles DSGVO data portability export
type DataPortabilityHandler struct {
	inventoryService    *service.InventoryService
	bioSafetyService    *service.BioSafetyService
	retentionService    *service.RetentionService
	reprocessingService *service.ReprocessingService
	hygieneService      *service.HygieneService
	radiationService    *service.RadiationService
	logger              *logger.Logger
}

// NewDataPortabilityHandler creates a new data portability handler
func NewDataPortabilityHandler(
	inventoryService *service.InventoryService,
	bioSafetyService *service.BioSafetyService,
	retentionService *service.RetentionService,
	reprocessingService *service.ReprocessingService,
	hygieneService *service.HygieneService,
	radiationService *service.RadiationService,
	log *logger.Logger,
) *DataPortabilityHandler {
	return &DataPortabilityHandler{
		inventoryService:    inventoryService,
		bioSafetyService:    bioSafetyService,
		retentionService:    retentionService,
		reprocessingService: reprocessingService,
		hygieneService:      hygieneService,
		radiationService:    radiationService,
		logger:              log,
	}
}

// ExportDataPortability exports all inventory data for DSGVO data portability (Art. 20 DSGVO)
// GET /export/data-portability
func (h *DataPortabilityHandler) ExportDataPortability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	export := DataPortabilityExport{
		ExportedAt: time.Now(),
		Format:     "application/json",
	}

	// Collect all inventory items
	items, err := h.inventoryService.GetAllActiveItems(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("data portability export: failed to get items")
	} else {
		export.Items = items
	}

	// Collect hazardous items with details
	hazardousItems, err := h.inventoryService.ListAllHazardousItems(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("data portability export: failed to get hazardous items")
	} else {
		export.HazardousItems = hazardousItems
	}

	// Collect bio trainings
	if h.bioSafetyService != nil {
		bioTrainings, err := h.bioSafetyService.ListTrainings(ctx)
		if err != nil {
			h.logger.Error().Err(err).Msg("data portability export: failed to get bio trainings")
		} else {
			export.BioTrainings = bioTrainings
		}
	}

	// Collect retention policies
	if h.retentionService != nil {
		retentionPolicies, err := h.retentionService.List(ctx)
		if err != nil {
			h.logger.Error().Err(err).Msg("data portability export: failed to get retention policies")
		} else {
			export.RetentionPolicies = retentionPolicies
		}
	}

	// Collect sterilization batches (first page, up to 1000)
	if h.reprocessingService != nil {
		batches, _, err := h.reprocessingService.ListBatches(ctx, 1, 1000)
		if err != nil {
			h.logger.Error().Err(err).Msg("data portability export: failed to get sterilization batches")
		} else {
			export.SterilizationBatches = batches
		}
	}

	// Collect hygiene plans (all statuses, all categories, up to 1000)
	if h.hygieneService != nil {
		hygienePlans, _, err := h.hygieneService.ListPlans(ctx, "", "", 1, 1000)
		if err != nil {
			h.logger.Error().Err(err).Msg("data portability export: failed to get hygiene plans")
		} else {
			export.HygienePlans = hygienePlans
		}

		hygieneInspections, _, err := h.hygieneService.ListInspections(ctx, "", 1, 1000)
		if err != nil {
			h.logger.Error().Err(err).Msg("data portability export: failed to get hygiene inspections")
		} else {
			export.HygieneInspections = hygieneInspections
		}
	}

	// Collect radiation devices
	if h.radiationService != nil {
		devices, err := h.radiationService.ListDevices(ctx)
		if err != nil {
			h.logger.Error().Err(err).Msg("data portability export: failed to get radiation devices")
		} else {
			export.RadiationDevices = devices
		}

		certifications, err := h.radiationService.ListCertifications(ctx)
		if err != nil {
			h.logger.Error().Err(err).Msg("data portability export: failed to get radiation certifications")
		} else {
			export.RadiationCertifications = certifications
		}

		dosimetry, _, err := h.radiationService.ListAllDosimetry(ctx, 1, 1000)
		if err != nil {
			h.logger.Error().Err(err).Msg("data portability export: failed to get dosimetry records")
		} else {
			export.DosimetryRecords = dosimetry
		}
	}

	httputil.JSON(w, http.StatusOK, export)
}
