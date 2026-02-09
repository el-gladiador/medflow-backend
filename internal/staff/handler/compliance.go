package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/internal/staff/service"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// ComplianceHandler handles compliance-related HTTP requests
type ComplianceHandler struct {
	service      *service.ComplianceService
	staffService *service.StaffService
	logger       *logger.Logger
}

// NewComplianceHandler creates a new compliance handler
func NewComplianceHandler(
	service *service.ComplianceService,
	staffService *service.StaffService,
	log *logger.Logger,
) *ComplianceHandler {
	return &ComplianceHandler{
		service:      service,
		staffService: staffService,
		logger:       log,
	}
}

// ============================================================================
// BREAK VALIDATION
// ============================================================================

// CheckBreakEnd validates if the current user can end their break
// GET /api/v1/compliance/break/check
func (h *ComplianceHandler) CheckBreakEnd(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		httputil.Error(w, errors.Unauthorized("user not authenticated"))
		return
	}

	// Get employee for user
	employee, err := h.staffService.GetByUserID(r.Context(), userID)
	if err != nil {
		httputil.Error(w, errors.NotFound("employee"))
		return
	}

	result, err := h.service.CheckBreakEndAllowed(r.Context(), employee.ID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to check break end")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// CheckBreakEndForEmployee validates if an employee can end their break
// GET /api/v1/compliance/employees/{id}/break/check
func (h *ComplianceHandler) CheckBreakEndForEmployee(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	result, err := h.service.CheckBreakEndAllowed(r.Context(), employeeID)
	if err != nil {
		h.logger.Error().Err(err).Str("employee_id", employeeID).Msg("failed to check break end for employee")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// ============================================================================
// CLOCK OUT VALIDATION
// ============================================================================

// CheckClockOut checks compliance for clock out
// GET /api/v1/compliance/clock-out/check
func (h *ComplianceHandler) CheckClockOut(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		httputil.Error(w, errors.Unauthorized("user not authenticated"))
		return
	}

	// Get employee for user
	employee, err := h.staffService.GetByUserID(r.Context(), userID)
	if err != nil {
		httputil.Error(w, errors.NotFound("employee"))
		return
	}

	result, err := h.service.CheckClockOutCompliance(r.Context(), employee.ID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to check clock out compliance")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// ============================================================================
// SHIFT VALIDATION
// ============================================================================

// ValidateShiftRequest is the request body for shift validation
type ValidateShiftRequest struct {
	EmployeeID string    `json:"employee_id"`
	ShiftStart time.Time `json:"shift_start"`
	ShiftEnd   time.Time `json:"shift_end"`
}

// ValidateShift validates a shift assignment against ArbZG
// POST /api/v1/compliance/shifts/validate
func (h *ComplianceHandler) ValidateShift(w http.ResponseWriter, r *http.Request) {
	var req ValidateShiftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, errors.BadRequest("invalid request body"))
		return
	}

	if req.EmployeeID == "" {
		httputil.Error(w, errors.BadRequest("employee_id is required"))
		return
	}

	result, err := h.service.ValidateShiftAssignment(r.Context(), req.EmployeeID, req.ShiftStart, req.ShiftEnd)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to validate shift")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// ============================================================================
// ALERTS
// ============================================================================

// GetActiveAlerts gets all active compliance alerts
// GET /api/v1/compliance/alerts
func (h *ComplianceHandler) GetActiveAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.service.GetActiveAlerts(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get alerts")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
	})
}

// DismissAlert dismisses a compliance alert
// POST /api/v1/compliance/alerts/{id}/dismiss
func (h *ComplianceHandler) DismissAlert(w http.ResponseWriter, r *http.Request) {
	alertID := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-ID")

	if err := h.service.DismissAlert(r.Context(), alertID, userID); err != nil {
		h.logger.Error().Err(err).Str("alert_id", alertID).Msg("failed to dismiss alert")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
}

// ============================================================================
// VIOLATIONS
// ============================================================================

// GetViolations gets compliance violations with filters
// GET /api/v1/compliance/violations
func (h *ComplianceHandler) GetViolations(w http.ResponseWriter, r *http.Request) {
	// Parse query params
	var employeeID *string
	if eid := r.URL.Query().Get("employee_id"); eid != "" {
		employeeID = &eid
	}

	var startDate *time.Time
	if sd := r.URL.Query().Get("start_date"); sd != "" {
		if t, err := time.Parse("2006-01-02", sd); err == nil {
			startDate = &t
		}
	}

	var endDate *time.Time
	if ed := r.URL.Query().Get("end_date"); ed != "" {
		if t, err := time.Parse("2006-01-02", ed); err == nil {
			endDate = &t
		}
	}

	var status *string
	if s := r.URL.Query().Get("status"); s != "" {
		status = &s
	}

	violations, err := h.service.GetViolations(r.Context(), employeeID, startDate, endDate, status)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get violations")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]interface{}{
		"violations": violations,
	})
}

// AcknowledgeViolation acknowledges a violation
// POST /api/v1/compliance/violations/{id}/acknowledge
func (h *ComplianceHandler) AcknowledgeViolation(w http.ResponseWriter, r *http.Request) {
	violationID := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-ID")

	if err := h.service.AcknowledgeViolation(r.Context(), violationID, userID); err != nil {
		h.logger.Error().Err(err).Str("violation_id", violationID).Msg("failed to acknowledge violation")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

// ============================================================================
// SETTINGS
// ============================================================================

// GetSettings gets compliance settings
// GET /api/v1/compliance/settings
func (h *ComplianceHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetSettings(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get settings")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, settings)
}

// UpdateSettings updates compliance settings
// PUT /api/v1/compliance/settings
func (h *ComplianceHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings repository.ComplianceSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		httputil.Error(w, errors.BadRequest("invalid request body"))
		return
	}

	if err := h.service.UpdateSettings(r.Context(), &settings); err != nil {
		h.logger.Error().Err(err).Msg("failed to update settings")
		httputil.Error(w, err)
		return
	}

	// Return updated settings
	updated, err := h.service.GetSettings(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get updated settings")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, updated)
}

// ============================================================================
// REAL-TIME MONITORING
// ============================================================================

// RunComplianceCheck runs compliance checks for all active employees
// POST /api/v1/compliance/check-all
func (h *ComplianceHandler) RunComplianceCheck(w http.ResponseWriter, r *http.Request) {
	if err := h.service.CheckAllActiveEmployees(r.Context()); err != nil {
		h.logger.Error().Err(err).Msg("failed to run compliance check")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// ============================================================================
// TIME CORRECTION REQUESTS
// ============================================================================

// CreateCorrectionRequestInput is the input for creating a correction request
type CreateCorrectionRequestInput struct {
	TimeEntryID       *string    `json:"time_entry_id,omitempty"`
	RequestedDate     string     `json:"requested_date"`     // YYYY-MM-DD
	RequestedClockIn  *string    `json:"requested_clock_in,omitempty"`  // ISO timestamp
	RequestedClockOut *string    `json:"requested_clock_out,omitempty"` // ISO timestamp
	RequestType       string     `json:"request_type"`       // clock_in_correction, clock_out_correction, missed_entry, delete_entry
	Reason            string     `json:"reason"`             // Required for audit
}

// CreateCorrectionRequest creates a new time correction request
// POST /api/v1/compliance/correction-requests
func (h *ComplianceHandler) CreateCorrectionRequest(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		httputil.Error(w, errors.Unauthorized("user not authenticated"))
		return
	}

	// Get employee for user
	employee, err := h.staffService.GetByUserID(r.Context(), userID)
	if err != nil {
		httputil.Error(w, errors.NotFound("employee"))
		return
	}

	var input CreateCorrectionRequestInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.Error(w, errors.BadRequest("invalid request body"))
		return
	}

	// Parse date
	requestedDate, err := time.Parse("2006-01-02", input.RequestedDate)
	if err != nil {
		httputil.Error(w, errors.BadRequest("invalid date format, use YYYY-MM-DD"))
		return
	}

	// Parse optional timestamps
	var clockIn, clockOut *time.Time
	if input.RequestedClockIn != nil {
		t, err := time.Parse(time.RFC3339, *input.RequestedClockIn)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid clock_in format"))
			return
		}
		clockIn = &t
	}
	if input.RequestedClockOut != nil {
		t, err := time.Parse(time.RFC3339, *input.RequestedClockOut)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid clock_out format"))
			return
		}
		clockOut = &t
	}

	req := &repository.CorrectionRequest{
		EmployeeID:        employee.ID,
		TimeEntryID:       input.TimeEntryID,
		RequestedDate:     requestedDate,
		RequestedClockIn:  clockIn,
		RequestedClockOut: clockOut,
		RequestType:       input.RequestType,
		Reason:            input.Reason,
	}

	if err := h.service.CreateCorrectionRequest(r.Context(), req); err != nil {
		h.logger.Error().Err(err).Msg("failed to create correction request")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusCreated, req)
}

// GetMyCorrectionRequests gets the current user's correction requests
// GET /api/v1/compliance/correction-requests/my
func (h *ComplianceHandler) GetMyCorrectionRequests(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		httputil.Error(w, errors.Unauthorized("user not authenticated"))
		return
	}

	employee, err := h.staffService.GetByUserID(r.Context(), userID)
	if err != nil {
		httputil.Error(w, errors.NotFound("employee"))
		return
	}

	requests, err := h.service.ListEmployeeCorrectionRequests(r.Context(), employee.ID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get correction requests")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]interface{}{
		"requests": requests,
	})
}

// GetPendingCorrectionRequests gets all pending correction requests (manager view)
// GET /api/v1/compliance/correction-requests/pending
func (h *ComplianceHandler) GetPendingCorrectionRequests(w http.ResponseWriter, r *http.Request) {
	requests, err := h.service.ListPendingCorrectionRequests(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get pending correction requests")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]interface{}{
		"requests": requests,
	})
}

// GetCorrectionRequest gets a specific correction request
// GET /api/v1/compliance/correction-requests/{id}
func (h *ComplianceHandler) GetCorrectionRequest(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "id")

	req, err := h.service.GetCorrectionRequest(r.Context(), requestID)
	if err != nil {
		h.logger.Error().Err(err).Str("request_id", requestID).Msg("failed to get correction request")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, req)
}

// ApproveCorrectionRequest approves a correction request
// POST /api/v1/compliance/correction-requests/{id}/approve
func (h *ComplianceHandler) ApproveCorrectionRequest(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-ID")

	// Get reviewer employee ID
	employee, err := h.staffService.GetByUserID(r.Context(), userID)
	if err != nil {
		httputil.Error(w, errors.NotFound("employee"))
		return
	}

	if err := h.service.ApproveCorrectionRequest(r.Context(), requestID, employee.ID); err != nil {
		h.logger.Error().Err(err).Str("request_id", requestID).Msg("failed to approve correction request")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// RejectCorrectionRequestInput is the input for rejecting a correction request
type RejectCorrectionRequestInput struct {
	Reason string `json:"reason"`
}

// RejectCorrectionRequest rejects a correction request
// POST /api/v1/compliance/correction-requests/{id}/reject
func (h *ComplianceHandler) RejectCorrectionRequest(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-ID")

	var input RejectCorrectionRequestInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.Error(w, errors.BadRequest("invalid request body"))
		return
	}

	// Get reviewer employee ID
	employee, err := h.staffService.GetByUserID(r.Context(), userID)
	if err != nil {
		httputil.Error(w, errors.NotFound("employee"))
		return
	}

	if err := h.service.RejectCorrectionRequest(r.Context(), requestID, employee.ID, input.Reason); err != nil {
		h.logger.Error().Err(err).Str("request_id", requestID).Msg("failed to reject correction request")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}
