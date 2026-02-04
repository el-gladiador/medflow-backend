package handler

import (
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
	service *service.TimeTrackingService
	logger  *logger.Logger
}

// NewTimeTrackingHandler creates a new time tracking handler
func NewTimeTrackingHandler(svc *service.TimeTrackingService, log *logger.Logger) *TimeTrackingHandler {
	return &TimeTrackingHandler{
		service: svc,
		logger:  log,
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
func (h *TimeTrackingHandler) UpdateEntry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		ClockIn  *string `json:"clock_in"`
		ClockOut *string `json:"clock_out"`
		Notes    *string `json:"notes"`
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
	if req.ClockOut != nil {
		clockOut, err := time.Parse(time.RFC3339, *req.ClockOut)
		if err != nil {
			httputil.Error(w, errors.BadRequest("invalid clock_out format"))
			return
		}
		updates["clock_out"] = clockOut
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
