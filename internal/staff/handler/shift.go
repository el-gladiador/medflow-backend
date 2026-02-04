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

// ShiftHandler handles shift-related endpoints
type ShiftHandler struct {
	service *service.ShiftService
	logger  *logger.Logger
}

// NewShiftHandler creates a new shift handler
func NewShiftHandler(svc *service.ShiftService, log *logger.Logger) *ShiftHandler {
	return &ShiftHandler{
		service: svc,
		logger:  log,
	}
}

// ============================================================================
// SHIFT TEMPLATES
// ============================================================================

// ListTemplates lists all shift templates
func (h *ShiftHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active_only") == "true"

	templates, err := h.service.ListTemplates(r.Context(), activeOnly)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, templates)
}

// GetTemplate gets a shift template by ID
func (h *ShiftHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	tmpl, err := h.service.GetTemplateByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, tmpl)
}

// CreateTemplate creates a new shift template
func (h *ShiftHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var tmpl repository.ShiftTemplate
	if err := httputil.DecodeJSON(r, &tmpl); err != nil {
		httputil.Error(w, err)
		return
	}

	// Set creator from header
	userID := r.Header.Get("X-User-ID")
	if userID != "" {
		tmpl.CreatedBy = &userID
	}

	if err := h.service.CreateTemplate(r.Context(), &tmpl); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, tmpl)
}

// UpdateTemplate updates a shift template
func (h *ShiftHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var tmpl repository.ShiftTemplate
	if err := httputil.DecodeJSON(r, &tmpl); err != nil {
		httputil.Error(w, err)
		return
	}

	tmpl.ID = id

	if err := h.service.UpdateTemplate(r.Context(), &tmpl); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, tmpl)
}

// DeleteTemplate deletes a shift template
func (h *ShiftHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteTemplate(r.Context(), id); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// ============================================================================
// SHIFT ASSIGNMENTS
// ============================================================================

// List lists shift assignments with filters
func (h *ShiftHandler) List(w http.ResponseWriter, r *http.Request) {
	params := repository.ShiftListParams{
		Page:    1,
		PerPage: 50,
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
	if shiftType := r.URL.Query().Get("shift_type"); shiftType != "" {
		params.ShiftType = &shiftType
	}

	shifts, total, err := h.service.ListAssignments(r.Context(), params)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / params.PerPage
	if int(total)%params.PerPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, shifts, &httputil.Meta{
		Page:       params.Page,
		PerPage:    params.PerPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// Get gets a shift assignment by ID
func (h *ShiftHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	shift, err := h.service.GetAssignmentByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, shift)
}

// Create creates a new shift assignment
func (h *ShiftHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateShiftRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	// Parse date
	shiftDate, err := time.Parse("2006-01-02", req.ShiftDate)
	if err != nil {
		httputil.Error(w, errors.BadRequest("invalid shift_date format, expected YYYY-MM-DD"))
		return
	}

	shift := &repository.ShiftAssignment{
		EmployeeID:           req.EmployeeID,
		ShiftTemplateID:      req.ShiftTemplateID,
		ShiftDate:            shiftDate,
		StartTime:            req.StartTime,
		EndTime:              req.EndTime,
		BreakDurationMinutes: req.BreakDurationMinutes,
		ShiftType:            req.ShiftType,
		Notes:                req.Notes,
	}

	// Set creator from header
	userID := r.Header.Get("X-User-ID")
	if userID != "" {
		shift.CreatedBy = &userID
	}

	if err := h.service.CreateAssignment(r.Context(), shift); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, shift)
}

// Update updates a shift assignment
func (h *ShiftHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req UpdateShiftRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	// Get existing shift
	shift, err := h.service.GetAssignmentByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	// Update fields
	if req.ShiftTemplateID != nil {
		shift.ShiftTemplateID = req.ShiftTemplateID
	}
	if req.ShiftDate != "" {
		if t, err := time.Parse("2006-01-02", req.ShiftDate); err == nil {
			shift.ShiftDate = t
		}
	}
	if req.StartTime != "" {
		shift.StartTime = req.StartTime
	}
	if req.EndTime != "" {
		shift.EndTime = req.EndTime
	}
	if req.BreakDurationMinutes != nil {
		shift.BreakDurationMinutes = *req.BreakDurationMinutes
	}
	if req.ShiftType != "" {
		shift.ShiftType = req.ShiftType
	}
	if req.Status != "" {
		shift.Status = req.Status
	}
	if req.Notes != nil {
		shift.Notes = req.Notes
	}

	// Set updater from header
	userID := r.Header.Get("X-User-ID")
	if userID != "" {
		shift.UpdatedBy = &userID
	}

	if err := h.service.UpdateAssignment(r.Context(), shift); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, shift)
}

// Delete deletes a shift assignment
func (h *ShiftHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteAssignment(r.Context(), id); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// BulkCreate creates multiple shift assignments
func (h *ShiftHandler) BulkCreate(w http.ResponseWriter, r *http.Request) {
	var req BulkCreateShiftsRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	userID := r.Header.Get("X-User-ID")

	var shifts []*repository.ShiftAssignment
	for _, s := range req.Shifts {
		shiftDate, err := time.Parse("2006-01-02", s.ShiftDate)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid shift_date format, expected YYYY-MM-DD"))
			return
		}

		shift := &repository.ShiftAssignment{
			EmployeeID:           s.EmployeeID,
			ShiftTemplateID:      s.ShiftTemplateID,
			ShiftDate:            shiftDate,
			StartTime:            s.StartTime,
			EndTime:              s.EndTime,
			BreakDurationMinutes: s.BreakDurationMinutes,
			ShiftType:            s.ShiftType,
			Notes:                s.Notes,
		}
		if userID != "" {
			shift.CreatedBy = &userID
		}
		shifts = append(shifts, shift)
	}

	if err := h.service.BulkCreateAssignments(r.Context(), shifts); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, map[string]interface{}{
		"created": len(shifts),
		"shifts":  shifts,
	})
}

// GetEmployeeShifts gets shifts for a specific employee
func (h *ShiftHandler) GetEmployeeShifts(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "employeeId")

	// Parse date range (default to current month)
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, -1)

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

	shifts, err := h.service.GetEmployeeShifts(r.Context(), employeeID, startDate, endDate)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, shifts)
}

// ============================================================================
// REQUEST/RESPONSE TYPES
// ============================================================================

// CreateShiftRequest is the request body for creating a shift
type CreateShiftRequest struct {
	EmployeeID           string  `json:"employee_id"`
	ShiftTemplateID      *string `json:"shift_template_id,omitempty"`
	ShiftDate            string  `json:"shift_date"` // YYYY-MM-DD
	StartTime            string  `json:"start_time"` // HH:MM or HH:MM:SS
	EndTime              string  `json:"end_time"`   // HH:MM or HH:MM:SS
	BreakDurationMinutes int     `json:"break_duration_minutes"`
	ShiftType            string  `json:"shift_type,omitempty"`
	Notes                *string `json:"notes,omitempty"`
}

// UpdateShiftRequest is the request body for updating a shift
type UpdateShiftRequest struct {
	ShiftTemplateID      *string `json:"shift_template_id,omitempty"`
	ShiftDate            string  `json:"shift_date,omitempty"` // YYYY-MM-DD
	StartTime            string  `json:"start_time,omitempty"` // HH:MM or HH:MM:SS
	EndTime              string  `json:"end_time,omitempty"`   // HH:MM or HH:MM:SS
	BreakDurationMinutes *int    `json:"break_duration_minutes,omitempty"`
	ShiftType            string  `json:"shift_type,omitempty"`
	Status               string  `json:"status,omitempty"`
	Notes                *string `json:"notes,omitempty"`
}

// BulkCreateShiftsRequest is the request body for bulk creating shifts
type BulkCreateShiftsRequest struct {
	Shifts []CreateShiftRequest `json:"shifts"`
}
