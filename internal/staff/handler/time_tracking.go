package handler

import (
	"encoding/json"
	stderrors "errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/internal/staff/service"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// TimeTrackingHandler handles time tracking endpoints
type TimeTrackingHandler struct {
	service      *service.TimeTrackingService
	staffService *service.StaffService
	logger       *logger.Logger
}

// NewTimeTrackingHandler creates a new time tracking handler
func NewTimeTrackingHandler(svc *service.TimeTrackingService, staffSvc *service.StaffService, log *logger.Logger) *TimeTrackingHandler {
	return &TimeTrackingHandler{
		service:      svc,
		staffService: staffSvc,
		logger:       log,
	}
}

// GetAllStatuses returns the time tracking status for all employees
// GET /time-tracking/statuses
func (h *TimeTrackingHandler) GetAllStatuses(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.service.GetAllStatuses(r.Context())
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, statuses)
}

// MyTimeStatus represents the current user's time tracking status for the personal clock bar
type MyTimeStatus struct {
	EmployeeID       string     `json:"employee_id"`
	EmployeeName     string     `json:"employee_name"`
	Status           string     `json:"status"` // clocked_out, clocked_in, on_break
	ClockIn          *time.Time `json:"clock_in,omitempty"`
	BreakStart       *time.Time `json:"break_start,omitempty"`
	TodayWorkMinutes int        `json:"today_work_minutes"`
	TodayBreakMinutes int       `json:"today_break_minutes"`
	WeekTotalMinutes int        `json:"week_total_minutes"`
}

// GetMyStatus returns the current user's time tracking status
// GET /time-tracking/my-status
func (h *TimeTrackingHandler) GetMyStatus(w http.ResponseWriter, r *http.Request) {
	// Get user ID from header (set by API Gateway from JWT)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		httputil.Error(w, errors.Unauthorized("user not authenticated"))
		return
	}

	// Get employee record for this user
	employee, err := h.staffService.GetByUserID(r.Context(), userID)
	if err != nil {
		// If employee not found, user is not linked to an employee record
		// Return a "not an employee" status
		if stderrors.Is(err, errors.ErrNotFound) {
			httputil.JSON(w, http.StatusOK, MyTimeStatus{
				Status: "not_employee",
			})
			return
		}
		httputil.Error(w, err)
		return
	}

	// Get time status for this employee
	status, err := h.service.GetEmployeeStatus(r.Context(), employee.ID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	// Build response with calculated real-time minutes
	response := MyTimeStatus{
		EmployeeID:       employee.ID,
		EmployeeName:     employee.FirstName + " " + employee.LastName,
		Status:           status.Status,
		ClockIn:          status.ClockIn,
		BreakStart:       status.BreakStart,
		WeekTotalMinutes: status.WeekTotalMinutes,
	}

	// Calculate real-time work minutes for today
	// If currently clocked in, add elapsed time since clock_in
	if status.Status == "clocked_in" && status.ClockIn != nil {
		elapsedMinutes := int(time.Since(*status.ClockIn).Minutes())
		// Get completed breaks for today from the active entry
		if status.TimeEntryID != nil {
			entry, err := h.service.GetEntryByID(r.Context(), *status.TimeEntryID)
			if err == nil && entry != nil {
				response.TodayBreakMinutes = entry.TotalBreakMinutes
				response.TodayWorkMinutes = elapsedMinutes - entry.TotalBreakMinutes
				if response.TodayWorkMinutes < 0 {
					response.TodayWorkMinutes = 0
				}
				// Include today's real-time work in week total
				response.WeekTotalMinutes = status.WeekTotalMinutes + response.TodayWorkMinutes - status.TodayWorkMinutes
			}
		} else {
			response.TodayWorkMinutes = elapsedMinutes
			response.WeekTotalMinutes = status.WeekTotalMinutes + elapsedMinutes - status.TodayWorkMinutes
		}
	} else if status.Status == "on_break" && status.ClockIn != nil {
		// On break: work time is elapsed since clock_in minus all breaks
		elapsedMinutes := int(time.Since(*status.ClockIn).Minutes())
		if status.TimeEntryID != nil {
			entry, err := h.service.GetEntryByID(r.Context(), *status.TimeEntryID)
			if err == nil && entry != nil {
				// Calculate current break duration
				currentBreakMinutes := 0
				if status.BreakStart != nil {
					currentBreakMinutes = int(time.Since(*status.BreakStart).Minutes())
				}
				response.TodayBreakMinutes = entry.TotalBreakMinutes + currentBreakMinutes
				response.TodayWorkMinutes = elapsedMinutes - response.TodayBreakMinutes
				if response.TodayWorkMinutes < 0 {
					response.TodayWorkMinutes = 0
				}
				// Include today's real-time work in week total
				response.WeekTotalMinutes = status.WeekTotalMinutes + response.TodayWorkMinutes - status.TodayWorkMinutes
			}
		}
	} else {
		// Clocked out - use stored totals
		response.TodayWorkMinutes = status.TodayWorkMinutes
		response.TodayBreakMinutes = status.TodayBreakMinutes
	}

	httputil.JSON(w, http.StatusOK, response)
}

// GetEntriesByDate returns all time entries for a specific date
// GET /time-tracking/entries?date=YYYY-MM-DD
func (h *TimeTrackingHandler) GetEntriesByDate(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		httputil.Error(w, errors.BadRequest("invalid date format, expected YYYY-MM-DD"))
		return
	}

	entries, err := h.service.GetEntriesByDate(r.Context(), date)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, entries)
}

// UpdateEntry updates a time entry
// PATCH /time-tracking/entries/{id}
// Supports clock_out: null to clear the clock-out time (resets employee to "working")
func (h *TimeTrackingHandler) UpdateEntry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Use json.RawMessage for clock_out to distinguish between absent, null, and string value
	var req struct {
		ClockIn  *string          `json:"clock_in"`
		ClockOut json.RawMessage  `json:"clock_out"`
		Notes    *string          `json:"notes"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	updates := make(map[string]interface{})
	if req.ClockIn != nil {
		clockIn, err := time.Parse(time.RFC3339, *req.ClockIn)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid clock_in format"))
			return
		}
		updates["clock_in"] = clockIn
	}

	// Handle clock_out: absent (not in JSON) vs null vs string value
	if req.ClockOut != nil {
		if string(req.ClockOut) == "null" {
			// Explicit null — clear the clock-out time
			updates["clock_out_clear"] = true
		} else {
			// String value — parse as RFC3339
			var clockOutStr string
			if err := json.Unmarshal(req.ClockOut, &clockOutStr); err != nil {
				httputil.Error(w, errors.BadRequest("invalid clock_out format"))
				return
			}
			clockOut, err := time.Parse(time.RFC3339, clockOutStr)
			if err != nil {
				httputil.Error(w, errors.BadRequest("invalid clock_out format"))
				return
			}
			updates["clock_out"] = clockOut
		}
	}

	if req.Notes != nil {
		updates["notes"] = *req.Notes
	}

	userID := r.Header.Get("X-User-ID")

	entry, err := h.service.UpdateEntry(r.Context(), id, updates, userID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, entry)
}

// UpdateEntryBreaks replaces all breaks for a time entry
// PATCH /time-tracking/entries/{id}/breaks
func (h *TimeTrackingHandler) UpdateEntryBreaks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Breaks []struct {
			ID        *string `json:"id"`
			StartTime string  `json:"start_time"`
			EndTime   *string `json:"end_time"`
		} `json:"breaks"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	// Parse break inputs
	var breaks []service.BreakInput
	for _, b := range req.Breaks {
		startTime, err := time.Parse(time.RFC3339, b.StartTime)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid start_time format for break"))
			return
		}

		bi := service.BreakInput{
			StartTime: startTime,
		}
		if b.ID != nil {
			bi.ID = *b.ID
		}
		if b.EndTime != nil && *b.EndTime != "" {
			endTime, err := time.Parse(time.RFC3339, *b.EndTime)
			if err != nil {
				httputil.Error(w, errors.BadRequest("invalid end_time format for break"))
				return
			}
			bi.EndTime = &endTime
		}

		breaks = append(breaks, bi)
	}

	userID := r.Header.Get("X-User-ID")

	entry, err := h.service.ReplaceBreaksForEntry(r.Context(), id, breaks, userID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, entry)
}

// DeleteEntry soft deletes a time entry
// DELETE /time-tracking/entries/{id}
func (h *TimeTrackingHandler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteEntry(r.Context(), id); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// ClockIn clocks in an employee
// POST /time-tracking/employees/{id}/clock-in
func (h *TimeTrackingHandler) ClockIn(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	entry, err := h.service.ClockIn(r.Context(), employeeID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, entry)
}

// ClockOut clocks out an employee
// POST /time-tracking/employees/{id}/clock-out
func (h *TimeTrackingHandler) ClockOut(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	entry, err := h.service.ClockOut(r.Context(), employeeID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, entry)
}

// StartBreak starts a break for an employee
// POST /time-tracking/employees/{id}/break/start
func (h *TimeTrackingHandler) StartBreak(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	entry, err := h.service.StartBreak(r.Context(), employeeID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, entry)
}

// EndBreak ends a break for an employee
// POST /time-tracking/employees/{id}/break/end
func (h *TimeTrackingHandler) EndBreak(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	entry, err := h.service.EndBreak(r.Context(), employeeID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, entry)
}

// ManualClockIn creates a manual clock in entry
// POST /time-tracking/employees/{id}/manual-clock-in
func (h *TimeTrackingHandler) ManualClockIn(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	var req struct {
		Time string `json:"time"` // Format: "HH:mm" or full timestamp
		Date string `json:"date"` // Optional: "YYYY-MM-DD", defaults to today
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	// Parse date (default to today)
	var date time.Time
	if req.Date != "" {
		var err error
		date, err = time.Parse("2006-01-02", req.Date)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid date format, expected YYYY-MM-DD"))
			return
		}
	} else {
		date = time.Now().Truncate(24 * time.Hour)
	}

	// Parse time
	var clockInTime time.Time
	if len(req.Time) == 5 { // HH:mm format
		t, err := time.Parse("15:04", req.Time)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid time format, expected HH:mm"))
			return
		}
		clockInTime = time.Date(date.Year(), date.Month(), date.Day(),
			t.Hour(), t.Minute(), 0, 0, time.Local)
	} else { // Full timestamp
		var err error
		clockInTime, err = time.Parse(time.RFC3339, req.Time)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid time format"))
			return
		}
	}

	userID := r.Header.Get("X-User-ID")

	entry, err := h.service.ManualClockIn(r.Context(), employeeID, clockInTime, userID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, entry)
}

// ManualClockOut creates a manual clock out entry
// POST /time-tracking/employees/{id}/manual-clock-out
func (h *TimeTrackingHandler) ManualClockOut(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	var req struct {
		Time string `json:"time"` // Format: "HH:mm" or full timestamp
		Date string `json:"date"` // Optional: "YYYY-MM-DD", defaults to today
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	// Parse date (default to today)
	var date time.Time
	if req.Date != "" {
		var err error
		date, err = time.Parse("2006-01-02", req.Date)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid date format, expected YYYY-MM-DD"))
			return
		}
	} else {
		date = time.Now().Truncate(24 * time.Hour)
	}

	// Parse time
	var clockOutTime time.Time
	if len(req.Time) == 5 { // HH:mm format
		t, err := time.Parse("15:04", req.Time)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid time format, expected HH:mm"))
			return
		}
		clockOutTime = time.Date(date.Year(), date.Month(), date.Day(),
			t.Hour(), t.Minute(), 0, 0, time.Local)
	} else { // Full timestamp
		var err error
		clockOutTime, err = time.Parse(time.RFC3339, req.Time)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid time format"))
			return
		}
	}

	userID := r.Header.Get("X-User-ID")

	entry, err := h.service.ManualClockOut(r.Context(), employeeID, clockOutTime, userID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, entry)
}

// GetEmployeeHistory returns time tracking history for an employee
// GET /time-tracking/employees/{id}/history?start=YYYY-MM-DD&end=YYYY-MM-DD
func (h *TimeTrackingHandler) GetEmployeeHistory(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	// Parse date range (default to current month)
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var startDate, endDate time.Time
	var err error

	if startStr != "" {
		startDate, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid start date format"))
			return
		}
	} else {
		// Default to start of current month
		now := time.Now()
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}

	if endStr != "" {
		endDate, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid end date format"))
			return
		}
	} else {
		// Default to end of current month
		now := time.Now()
		endDate = time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	}

	summary, err := h.service.GetEmployeeHistory(r.Context(), employeeID, startDate, endDate)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, summary)
}

// GetEmployeeCorrections returns corrections for an employee
// GET /time-tracking/employees/{id}/corrections?start=YYYY-MM-DD&end=YYYY-MM-DD
func (h *TimeTrackingHandler) GetEmployeeCorrections(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	// Parse date range (default to last 30 days)
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var startDate, endDate time.Time
	var err error

	if startStr != "" {
		startDate, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid start date format"))
			return
		}
	} else {
		startDate = time.Now().AddDate(0, 0, -30)
	}

	if endStr != "" {
		endDate, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid end date format"))
			return
		}
	} else {
		endDate = time.Now()
	}

	corrections, err := h.service.GetEmployeeCorrections(r.Context(), employeeID, startDate, endDate)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, corrections)
}

// CreateCorrection creates a time correction
// POST /time-tracking/corrections
func (h *TimeTrackingHandler) CreateCorrection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EmployeeID string  `json:"employee_id"`
		Date       string  `json:"date"`    // YYYY-MM-DD
		ClockIn    *string `json:"clock_in"` // HH:mm or RFC3339
		ClockOut   *string `json:"clock_out"` // HH:mm or RFC3339
		Reason     string  `json:"reason"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	// Parse date
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		httputil.Error(w, errors.BadRequest("invalid date format, expected YYYY-MM-DD"))
		return
	}

	userID := r.Header.Get("X-User-ID")

	corr := &repository.TimeCorrection{
		EmployeeID:     req.EmployeeID,
		CorrectionDate: date,
		Reason:         req.Reason,
		CorrectedBy:    userID,
	}

	// Parse clock in time if provided
	if req.ClockIn != nil {
		clockIn, err := parseTimeWithDate(*req.ClockIn, date)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid clock_in format"))
			return
		}
		corr.CorrectedClockIn = &clockIn
	}

	// Parse clock out time if provided
	if req.ClockOut != nil {
		clockOut, err := parseTimeWithDate(*req.ClockOut, date)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid clock_out format"))
			return
		}
		corr.CorrectedClockOut = &clockOut
	}

	if err := h.service.CreateCorrection(r.Context(), corr); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, corr)
}

// parseTimeWithDate parses a time string (HH:mm or RFC3339) and combines with a date
func parseTimeWithDate(timeStr string, date time.Time) (time.Time, error) {
	if len(timeStr) == 5 { // HH:mm format
		t, err := time.Parse("15:04", timeStr)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(date.Year(), date.Month(), date.Day(),
			t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}
	// Full timestamp
	return time.Parse(time.RFC3339, timeStr)
}
