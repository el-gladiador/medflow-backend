package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/internal/staff/service"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// AbsenceHandler handles absence-related endpoints
type AbsenceHandler struct {
	service *service.AbsenceService
	logger  *logger.Logger
}

// NewAbsenceHandler creates a new absence handler
func NewAbsenceHandler(svc *service.AbsenceService, log *logger.Logger) *AbsenceHandler {
	return &AbsenceHandler{
		service: svc,
		logger:  log,
	}
}

// ============================================================================
// ABSENCES
// ============================================================================

// List lists absences with filters
func (h *AbsenceHandler) List(w http.ResponseWriter, r *http.Request) {
	params := repository.AbsenceListParams{
		Page:    1,
		PerPage: 20,
	}

	// Parse query parameters
	if page, _ := strconv.Atoi(r.URL.Query().Get("page")); page > 0 {
		params.Page = page
	}
	if perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page")); perPage > 0 && perPage <= 100 {
		params.PerPage = perPage
	}
	if employeeID := r.URL.Query().Get("employee_id"); employeeID != "" {
		params.EmployeeID = &employeeID
	}
	if startDate := r.URL.Query().Get("start_date"); startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			params.StartDate = &t
		}
	}
	if endDate := r.URL.Query().Get("end_date"); endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			params.EndDate = &t
		}
	}
	if status := r.URL.Query().Get("status"); status != "" {
		params.Status = &status
	}
	if absenceType := r.URL.Query().Get("absence_type"); absenceType != "" {
		params.AbsenceType = &absenceType
	}

	absences, total, err := h.service.List(r.Context(), params)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / params.PerPage
	if int(total)%params.PerPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, absences, &httputil.Meta{
		Page:       params.Page,
		PerPage:    params.PerPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// Get gets an absence by ID
func (h *AbsenceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	absence, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, absence)
}

// Create creates a new absence request
func (h *AbsenceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateAbsenceRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		httputil.Error(w, errors.BadRequest("invalid start_date format, expected YYYY-MM-DD"))
		return
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		httputil.Error(w, errors.BadRequest("invalid end_date format, expected YYYY-MM-DD"))
		return
	}

	// Validate dates
	if endDate.Before(startDate) {
		httputil.Error(w, errors.BadRequest("end_date cannot be before start_date"))
		return
	}

	absence := &repository.Absence{
		EmployeeID:   req.EmployeeID,
		StartDate:    startDate,
		EndDate:      endDate,
		AbsenceType:  req.AbsenceType,
		EmployeeNote: req.EmployeeNote,
	}

	// Set creator from header
	userID := r.Header.Get("X-User-ID")
	if userID != "" {
		absence.CreatedBy = &userID
	}

	if err := h.service.Create(r.Context(), absence); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, absence)
}

// Update updates an absence
func (h *AbsenceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req UpdateAbsenceRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	// Get existing absence
	absence, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	// Only allow updates to pending absences
	if absence.Status != "pending" {
		httputil.Error(w, errors.BadRequest("can only update pending absences"))
		return
	}

	// Update fields
	if req.StartDate != "" {
		if t, err := time.Parse("2006-01-02", req.StartDate); err == nil {
			absence.StartDate = t
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse("2006-01-02", req.EndDate); err == nil {
			absence.EndDate = t
		}
	}
	if req.AbsenceType != "" {
		absence.AbsenceType = req.AbsenceType
	}
	if req.EmployeeNote != nil {
		absence.EmployeeNote = req.EmployeeNote
	}

	// Validate dates
	if absence.EndDate.Before(absence.StartDate) {
		httputil.Error(w, errors.BadRequest("end_date cannot be before start_date"))
		return
	}

	// Set updater from header
	userID := r.Header.Get("X-User-ID")
	if userID != "" {
		absence.UpdatedBy = &userID
	}

	if err := h.service.Update(r.Context(), absence); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, absence)
}

// Delete deletes an absence
func (h *AbsenceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.Delete(r.Context(), id); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// Approve approves an absence request
func (h *AbsenceHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req ApproveRejectRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		// Allow empty body
		req = ApproveRejectRequest{}
	}

	reviewerID := r.Header.Get("X-User-ID")
	if reviewerID == "" {
		httputil.Error(w, errors.BadRequest("reviewer ID required"))
		return
	}

	if err := h.service.Approve(r.Context(), id, reviewerID, req.Note); err != nil {
		httputil.Error(w, err)
		return
	}

	// Get updated absence
	absence, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, absence)
}

// Reject rejects an absence request
func (h *AbsenceHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req ApproveRejectRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if req.Reason == "" {
		httputil.Error(w, errors.BadRequest("rejection reason is required"))
		return
	}

	reviewerID := r.Header.Get("X-User-ID")
	if reviewerID == "" {
		httputil.Error(w, errors.BadRequest("reviewer ID required"))
		return
	}

	if err := h.service.Reject(r.Context(), id, reviewerID, req.Reason, req.Note); err != nil {
		httputil.Error(w, err)
		return
	}

	// Get updated absence
	absence, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, absence)
}

// GetEmployeeAbsences gets absences for a specific employee
func (h *AbsenceHandler) GetEmployeeAbsences(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "employeeId")

	// Parse date range (default to current year)
	now := time.Now()
	startDate := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(now.Year(), 12, 31, 23, 59, 59, 0, time.UTC)

	if sd := r.URL.Query().Get("start_date"); sd != "" {
		if t, err := time.Parse("2006-01-02", sd); err == nil {
			startDate = t
		}
	}
	if ed := r.URL.Query().Get("end_date"); ed != "" {
		if t, err := time.Parse("2006-01-02", ed); err == nil {
			endDate = t
		}
	}

	absences, err := h.service.GetEmployeeAbsences(r.Context(), employeeID, startDate, endDate)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, absences)
}

// ============================================================================
// VACATION INFO
// ============================================================================

// ListVacationBalances lists vacation balances for all employees
func (h *AbsenceHandler) ListVacationBalances(w http.ResponseWriter, r *http.Request) {
	year := time.Now().Year()
	if y, _ := strconv.Atoi(r.URL.Query().Get("year")); y > 0 {
		year = y
	}

	balances, err := h.service.ListVacationBalances(r.Context(), year)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, balances)
}

// GetEmployeeVacationBalance gets vacation balance for a specific employee
func (h *AbsenceHandler) GetEmployeeVacationBalance(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "employeeId")

	year := time.Now().Year()
	if y, _ := strconv.Atoi(r.URL.Query().Get("year")); y > 0 {
		year = y
	}

	balance, err := h.service.GetVacationBalance(r.Context(), employeeID, year)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	// Calculate available days
	response := VacationInfoResponse{
		EmployeeID:         balance.EmployeeID,
		Year:               balance.Year,
		TotalEntitlement:   balance.AnnualEntitlement + balance.CarryoverFromPrevious + balance.AdditionalGranted,
		AnnualEntitlement:  balance.AnnualEntitlement,
		CarryoverDays:      balance.CarryoverFromPrevious,
		AdditionalGranted:  balance.AdditionalGranted,
		Taken:              balance.Taken,
		Planned:            balance.Planned,
		Pending:            balance.Pending,
		Available:          balance.Available(),
	}

	httputil.JSON(w, http.StatusOK, response)
}

// SetVacationEntitlement sets vacation entitlement for a specific employee
func (h *AbsenceHandler) SetVacationEntitlement(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "employeeId")

	var req SetVacationEntitlementRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	year := req.Year
	if year == 0 {
		year = time.Now().Year()
	}

	if err := h.service.SetVacationEntitlement(r.Context(), employeeID, year, req.Entitlement); err != nil {
		httputil.Error(w, err)
		return
	}

	// Return updated balance
	balance, err := h.service.GetVacationBalance(r.Context(), employeeID, year)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	response := VacationInfoResponse{
		EmployeeID:         balance.EmployeeID,
		Year:               balance.Year,
		TotalEntitlement:   balance.AnnualEntitlement + balance.CarryoverFromPrevious + balance.AdditionalGranted,
		AnnualEntitlement:  balance.AnnualEntitlement,
		CarryoverDays:      balance.CarryoverFromPrevious,
		AdditionalGranted:  balance.AdditionalGranted,
		Taken:              balance.Taken,
		Planned:            balance.Planned,
		Pending:            balance.Pending,
		Available:          balance.Available(),
	}

	httputil.JSON(w, http.StatusOK, response)
}

// ============================================================================
// REQUEST/RESPONSE TYPES
// ============================================================================

// CreateAbsenceRequest is the request body for creating an absence
type CreateAbsenceRequest struct {
	EmployeeID   string  `json:"employee_id"`
	StartDate    string  `json:"start_date"`    // YYYY-MM-DD
	EndDate      string  `json:"end_date"`      // YYYY-MM-DD
	AbsenceType  string  `json:"absence_type"`  // vacation, sick, etc.
	EmployeeNote *string `json:"employee_note,omitempty"`
}

// UpdateAbsenceRequest is the request body for updating an absence
type UpdateAbsenceRequest struct {
	StartDate    string  `json:"start_date,omitempty"`    // YYYY-MM-DD
	EndDate      string  `json:"end_date,omitempty"`      // YYYY-MM-DD
	AbsenceType  string  `json:"absence_type,omitempty"`
	EmployeeNote *string `json:"employee_note,omitempty"`
}

// ApproveRejectRequest is the request body for approving/rejecting an absence
type ApproveRejectRequest struct {
	Reason string  `json:"reason,omitempty"` // Required for rejection
	Note   *string `json:"note,omitempty"`   // Optional manager note
}

// SetVacationEntitlementRequest is the request body for setting vacation entitlement
type SetVacationEntitlementRequest struct {
	Year        int     `json:"year,omitempty"`
	Entitlement float64 `json:"entitlement"`
}

// VacationInfoResponse is the response for vacation info endpoints
type VacationInfoResponse struct {
	EmployeeID        string  `json:"employee_id"`
	Year              int     `json:"year"`
	TotalEntitlement  float64 `json:"total_entitlement"`
	AnnualEntitlement float64 `json:"annual_entitlement"`
	CarryoverDays     float64 `json:"carryover_days"`
	AdditionalGranted float64 `json:"additional_granted"`
	Taken             float64 `json:"taken"`
	Planned           float64 `json:"planned"`
	Pending           float64 `json:"pending"`
	Available         float64 `json:"available"`
}
