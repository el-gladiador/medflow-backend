package service

import (
	"context"
	"fmt"
	"time"

	"github.com/medflow/medflow-backend/internal/staff/events"
	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// TimeTrackingService handles time tracking business logic
type TimeTrackingService struct {
	repo      *repository.TimeTrackingRepository
	publisher *events.StaffEventPublisher
	logger    *logger.Logger
}

// NewTimeTrackingService creates a new time tracking service
func NewTimeTrackingService(
	repo *repository.TimeTrackingRepository,
	publisher *events.StaffEventPublisher,
	log *logger.Logger,
) *TimeTrackingService {
	return &TimeTrackingService{
		repo:      repo,
		publisher: publisher,
		logger:    log,
	}
}

// ClockIn clocks in an employee
func (s *TimeTrackingService) ClockIn(ctx context.Context, employeeID string) (*repository.TimeEntry, error) {
	// Verify employee exists
	exists, err := s.repo.CheckEmployeeExists(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NotFound("employee")
	}

	// Check if already clocked in
	activeEntry, err := s.repo.GetActiveEntryByEmployeeID(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if activeEntry != nil {
		return nil, errors.BadRequest("already clocked in")
	}

	// Create new time entry
	now := time.Now()
	entry := &repository.TimeEntry{
		EmployeeID:    employeeID,
		EntryDate:     time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
		ClockIn:       now,
		IsManualEntry: false,
	}

	if err := s.repo.CreateEntry(ctx, entry); err != nil {
		return nil, err
	}

	// Publish event
	s.publisher.PublishTimeClockIn(ctx, entry)

	return entry, nil
}

// ClockOut clocks out an employee
func (s *TimeTrackingService) ClockOut(ctx context.Context, employeeID string) (*repository.TimeEntry, error) {
	// Get active entry
	entry, err := s.repo.GetActiveEntryByEmployeeID(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, errors.BadRequest("not clocked in")
	}

	// End any active break first
	activeBreak, err := s.repo.GetActiveBreak(ctx, entry.ID)
	if err != nil {
		return nil, err
	}
	if activeBreak != nil {
		now := time.Now()
		activeBreak.EndTime = &now
		if err := s.repo.UpdateBreak(ctx, activeBreak); err != nil {
			return nil, err
		}
		s.publisher.PublishTimeBreakEnd(ctx, activeBreak, employeeID)
	}

	// Calculate totals
	now := time.Now()
	entry.ClockOut = &now

	totalBreakMinutes, err := s.repo.CalculateTotalBreakMinutes(ctx, entry.ID)
	if err != nil {
		return nil, err
	}
	entry.TotalBreakMinutes = totalBreakMinutes

	// Calculate work minutes: (clock_out - clock_in) - breaks
	totalMinutes := int(entry.ClockOut.Sub(entry.ClockIn).Minutes())
	entry.TotalWorkMinutes = totalMinutes - totalBreakMinutes
	if entry.TotalWorkMinutes < 0 {
		entry.TotalWorkMinutes = 0
	}

	if err := s.repo.UpdateEntry(ctx, entry); err != nil {
		return nil, err
	}

	// Publish event
	s.publisher.PublishTimeClockOut(ctx, entry)

	return entry, nil
}

// StartBreak starts a break for an employee
func (s *TimeTrackingService) StartBreak(ctx context.Context, employeeID string) (*repository.TimeEntry, error) {
	// Get active entry
	entry, err := s.repo.GetActiveEntryByEmployeeID(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, errors.BadRequest("not clocked in")
	}

	// Check if already on break
	activeBreak, err := s.repo.GetActiveBreak(ctx, entry.ID)
	if err != nil {
		return nil, err
	}
	if activeBreak != nil {
		return nil, errors.BadRequest("already on break")
	}

	// Create new break
	brk := &repository.TimeBreak{
		TimeEntryID: entry.ID,
		StartTime:   time.Now(),
	}

	if err := s.repo.CreateBreak(ctx, brk); err != nil {
		return nil, err
	}

	// Publish event
	s.publisher.PublishTimeBreakStart(ctx, brk, employeeID)

	// Refresh entry to get updated state
	entry, err = s.repo.GetEntryByID(ctx, entry.ID)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// EndBreak ends a break for an employee
func (s *TimeTrackingService) EndBreak(ctx context.Context, employeeID string) (*repository.TimeEntry, error) {
	// Get active entry
	entry, err := s.repo.GetActiveEntryByEmployeeID(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, errors.BadRequest("not clocked in")
	}

	// Get active break
	activeBreak, err := s.repo.GetActiveBreak(ctx, entry.ID)
	if err != nil {
		return nil, err
	}
	if activeBreak == nil {
		return nil, errors.BadRequest("not on break")
	}

	// End the break
	now := time.Now()
	activeBreak.EndTime = &now

	if err := s.repo.UpdateBreak(ctx, activeBreak); err != nil {
		return nil, err
	}

	// Publish event
	s.publisher.PublishTimeBreakEnd(ctx, activeBreak, employeeID)

	// Refresh entry
	entry, err = s.repo.GetEntryByID(ctx, entry.ID)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// ManualClockIn creates a manual clock in entry (for manager corrections)
func (s *TimeTrackingService) ManualClockIn(ctx context.Context, employeeID string, clockInTime time.Time, userID string) (*repository.TimeEntry, error) {
	// Verify employee exists
	exists, err := s.repo.CheckEmployeeExists(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NotFound("employee")
	}

	// Check if already has an entry for this date
	entryDate := time.Date(clockInTime.Year(), clockInTime.Month(), clockInTime.Day(), 0, 0, 0, 0, clockInTime.Location())
	existingEntry, err := s.repo.GetEntryByEmployeeAndDate(ctx, employeeID, entryDate)
	if err != nil {
		return nil, err
	}
	if existingEntry != nil {
		return nil, errors.BadRequest("time entry already exists for this date")
	}

	// Create new time entry
	entry := &repository.TimeEntry{
		EmployeeID:    employeeID,
		EntryDate:     entryDate,
		ClockIn:       clockInTime,
		IsManualEntry: true,
		CreatedBy:     &userID,
	}

	if err := s.repo.CreateEntry(ctx, entry); err != nil {
		return nil, err
	}

	// Publish event
	s.publisher.PublishTimeClockIn(ctx, entry)

	return entry, nil
}

// ManualClockOut creates a manual clock out for an existing entry (for manager corrections)
func (s *TimeTrackingService) ManualClockOut(ctx context.Context, employeeID string, clockOutTime time.Time, userID string) (*repository.TimeEntry, error) {
	// Get entry for the date
	entryDate := time.Date(clockOutTime.Year(), clockOutTime.Month(), clockOutTime.Day(), 0, 0, 0, 0, clockOutTime.Location())
	entry, err := s.repo.GetEntryByEmployeeAndDate(ctx, employeeID, entryDate)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, errors.NotFound("time_entry")
	}

	// Validate clock out time is after clock in
	if clockOutTime.Before(entry.ClockIn) {
		return nil, errors.BadRequest("clock out time must be after clock in time")
	}

	// Update entry
	entry.ClockOut = &clockOutTime
	entry.IsManualEntry = true
	entry.UpdatedBy = &userID

	// Calculate totals
	totalBreakMinutes, err := s.repo.CalculateTotalBreakMinutes(ctx, entry.ID)
	if err != nil {
		return nil, err
	}
	entry.TotalBreakMinutes = totalBreakMinutes

	totalMinutes := int(entry.ClockOut.Sub(entry.ClockIn).Minutes())
	entry.TotalWorkMinutes = totalMinutes - totalBreakMinutes
	if entry.TotalWorkMinutes < 0 {
		entry.TotalWorkMinutes = 0
	}

	if err := s.repo.UpdateEntry(ctx, entry); err != nil {
		return nil, err
	}

	// Publish event
	s.publisher.PublishTimeClockOut(ctx, entry)

	return entry, nil
}

// CreateCorrection creates a time correction record
func (s *TimeTrackingService) CreateCorrection(ctx context.Context, corr *repository.TimeCorrection) error {
	// Verify employee exists
	exists, err := s.repo.CheckEmployeeExists(ctx, corr.EmployeeID)
	if err != nil {
		return err
	}
	if !exists {
		return errors.NotFound("employee")
	}

	// Validate reason is provided
	if corr.Reason == "" {
		return errors.BadRequest("reason is required for corrections")
	}

	return s.repo.CreateCorrection(ctx, corr)
}

// GetEmployeeCorrections gets corrections for an employee
func (s *TimeTrackingService) GetEmployeeCorrections(ctx context.Context, employeeID string, startDate, endDate time.Time) ([]*repository.TimeCorrection, error) {
	// Verify employee exists
	exists, err := s.repo.CheckEmployeeExists(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NotFound("employee")
	}

	return s.repo.ListCorrectionsForEmployee(ctx, employeeID, startDate, endDate)
}

// GetAllStatuses gets time tracking status for all employees
func (s *TimeTrackingService) GetAllStatuses(ctx context.Context) ([]*repository.EmployeeTimeStatus, error) {
	return s.repo.GetAllEmployeeStatuses(ctx)
}

// GetEmployeeHistory gets time tracking history for an employee
func (s *TimeTrackingService) GetEmployeeHistory(ctx context.Context, employeeID string, startDate, endDate time.Time) (*repository.TimePeriodSummary, error) {
	// Verify employee exists
	exists, err := s.repo.CheckEmployeeExists(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NotFound("employee")
	}

	return s.repo.GetEmployeeTimeSummary(ctx, employeeID, startDate, endDate)
}

// GetEntriesByDate gets all time entries for a specific date
func (s *TimeTrackingService) GetEntriesByDate(ctx context.Context, date time.Time) ([]*repository.TimeEntry, error) {
	return s.repo.ListEntriesByDate(ctx, date)
}

// GetEntryByID gets a time entry by ID
func (s *TimeTrackingService) GetEntryByID(ctx context.Context, id string) (*repository.TimeEntry, error) {
	return s.repo.GetEntryByID(ctx, id)
}

// UpdateEntry updates a time entry (for partial updates)
func (s *TimeTrackingService) UpdateEntry(ctx context.Context, id string, updates map[string]interface{}, userID string) (*repository.TimeEntry, error) {
	// Get existing entry
	entry, err := s.repo.GetEntryByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if clockIn, ok := updates["clock_in"].(time.Time); ok {
		entry.ClockIn = clockIn
	}
	if clockOut, ok := updates["clock_out"].(time.Time); ok {
		entry.ClockOut = &clockOut
	}
	if notes, ok := updates["notes"].(string); ok {
		entry.Notes = &notes
	}

	entry.IsManualEntry = true
	entry.UpdatedBy = &userID

	// Recalculate if clock_out is set
	if entry.ClockOut != nil {
		totalBreakMinutes, err := s.repo.CalculateTotalBreakMinutes(ctx, entry.ID)
		if err != nil {
			return nil, err
		}
		entry.TotalBreakMinutes = totalBreakMinutes

		totalMinutes := int(entry.ClockOut.Sub(entry.ClockIn).Minutes())
		entry.TotalWorkMinutes = totalMinutes - totalBreakMinutes
		if entry.TotalWorkMinutes < 0 {
			entry.TotalWorkMinutes = 0
		}
	}

	if err := s.repo.UpdateEntry(ctx, entry); err != nil {
		return nil, err
	}

	return entry, nil
}

// DeleteEntry soft deletes a time entry
func (s *TimeTrackingService) DeleteEntry(ctx context.Context, id string) error {
	return s.repo.SoftDeleteEntry(ctx, id)
}

// GetActiveEntry gets the active time entry for an employee
func (s *TimeTrackingService) GetActiveEntry(ctx context.Context, employeeID string) (*repository.TimeEntry, error) {
	return s.repo.GetActiveEntryByEmployeeID(ctx, employeeID)
}

// GetActiveBreak gets the active break for a time entry
func (s *TimeTrackingService) GetActiveBreak(ctx context.Context, timeEntryID string) (*repository.TimeBreak, error) {
	return s.repo.GetActiveBreak(ctx, timeEntryID)
}

// GetEmployeeStatus gets the current status of an employee
func (s *TimeTrackingService) GetEmployeeStatus(ctx context.Context, employeeID string) (*repository.EmployeeTimeStatus, error) {
	// Verify employee exists
	exists, err := s.repo.CheckEmployeeExists(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NotFound("employee")
	}

	// Get active entry
	entry, err := s.repo.GetActiveEntryByEmployeeID(ctx, employeeID)
	if err != nil {
		return nil, err
	}

	status := &repository.EmployeeTimeStatus{
		EmployeeID: employeeID,
	}

	if entry == nil {
		status.Status = "clocked_out"
	} else {
		status.TimeEntryID = &entry.ID
		status.ClockIn = &entry.ClockIn

		// Check for active break
		activeBreak, err := s.repo.GetActiveBreak(ctx, entry.ID)
		if err != nil {
			return nil, err
		}

		if activeBreak != nil {
			status.Status = "on_break"
			status.BreakStart = &activeBreak.StartTime
		} else {
			status.Status = "clocked_in"
		}
	}

	// Get today's minutes
	todayEntries, err := s.repo.ListEntriesForEmployee(ctx, employeeID,
		time.Now().Truncate(24*time.Hour), time.Now())
	if err != nil {
		return nil, err
	}
	for _, e := range todayEntries {
		status.TodayMinutes += e.TotalWorkMinutes
	}

	// Get week minutes
	weekMinutes, err := s.repo.GetTotalWorkMinutesForWeek(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	status.WeekMinutes = weekMinutes

	return status, nil
}

// FormatDuration formats minutes as HH:MM string
func FormatDuration(minutes int) string {
	hours := minutes / 60
	mins := minutes % 60
	return fmt.Sprintf("%02d:%02d", hours, mins)
}
