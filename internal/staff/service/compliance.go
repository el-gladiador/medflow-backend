package service

import (
	"context"
	"fmt"
	"time"

	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// ComplianceService handles ArbZG (German Labor Law) compliance logic
type ComplianceService struct {
	complianceRepo   *repository.ComplianceRepository
	timeTrackingRepo *repository.TimeTrackingRepository
	shiftRepo        *repository.ShiftRepository
	logger           *logger.Logger
}

// NewComplianceService creates a new compliance service
func NewComplianceService(
	complianceRepo *repository.ComplianceRepository,
	timeTrackingRepo *repository.TimeTrackingRepository,
	shiftRepo *repository.ShiftRepository,
	log *logger.Logger,
) *ComplianceService {
	return &ComplianceService{
		complianceRepo:   complianceRepo,
		timeTrackingRepo: timeTrackingRepo,
		shiftRepo:        shiftRepo,
		logger:           log,
	}
}

// ============================================================================
// BREAK VALIDATION (ArbZG §4)
// ============================================================================

// BreakValidationResult contains the result of break validation
type BreakValidationResult struct {
	Allowed            bool   `json:"allowed"`
	MinimumMinutes     int    `json:"minimum_minutes"`
	CurrentBreakMinutes int   `json:"current_break_minutes"`
	TotalWorkMinutes   int    `json:"total_work_minutes"`
	RemainingMinutes   int    `json:"remaining_minutes"`
	Message            string `json:"message"`
}

// CheckBreakEndAllowed validates if an employee can end their break
// ArbZG §4: 30min break for 6-9h work, 45min break for >9h work
func (s *ComplianceService) CheckBreakEndAllowed(ctx context.Context, employeeID string) (*BreakValidationResult, error) {
	// Get settings
	settings, err := s.complianceRepo.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	// Get active time entry
	entry, err := s.timeTrackingRepo.GetActiveEntryByEmployeeID(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, errors.BadRequest("not clocked in")
	}

	// Get active break
	activeBreak, err := s.timeTrackingRepo.GetActiveBreak(ctx, entry.ID)
	if err != nil {
		return nil, err
	}
	if activeBreak == nil {
		return nil, errors.BadRequest("not on break")
	}

	// Calculate current work duration (excluding current break)
	now := time.Now()
	totalWorkMinutes := int(now.Sub(entry.ClockIn).Minutes())

	// Get already taken breaks
	existingBreakMinutes, err := s.timeTrackingRepo.CalculateTotalBreakMinutes(ctx, entry.ID)
	if err != nil {
		return nil, err
	}

	// Calculate current break duration
	currentBreakMinutes := int(now.Sub(activeBreak.StartTime).Minutes())

	// Net work time = total time - existing breaks (current break is still active)
	netWorkMinutes := totalWorkMinutes - existingBreakMinutes

	// Determine required break based on work hours
	var requiredBreakMinutes int
	if netWorkMinutes > 9*60 { // > 9 hours
		requiredBreakMinutes = settings.MinBreak9hMinutes // 45 min
	} else if netWorkMinutes > 6*60 { // > 6 hours
		requiredBreakMinutes = settings.MinBreak6hMinutes // 30 min
	} else {
		// Under 6 hours - no minimum break required
		requiredBreakMinutes = 0
	}

	// Total break taken including current break
	totalBreakTaken := existingBreakMinutes + currentBreakMinutes

	// Check if minimum is met
	remainingMinutes := requiredBreakMinutes - totalBreakTaken
	if remainingMinutes < 0 {
		remainingMinutes = 0
	}

	result := &BreakValidationResult{
		Allowed:            remainingMinutes == 0,
		MinimumMinutes:     requiredBreakMinutes,
		CurrentBreakMinutes: currentBreakMinutes,
		TotalWorkMinutes:   netWorkMinutes,
		RemainingMinutes:   remainingMinutes,
	}

	if !result.Allowed {
		result.Message = fmt.Sprintf(
			"Sie müssen noch %d Minuten Pause machen (gesetzliche Mindestpause: %d Minuten für %d Stunden Arbeit)",
			remainingMinutes, requiredBreakMinutes, netWorkMinutes/60,
		)
	}

	return result, nil
}

// CheckBreakSegmentValid validates if a break segment meets minimum duration
// ArbZG §4: Break segments must be at least 15 minutes
func (s *ComplianceService) CheckBreakSegmentValid(ctx context.Context, breakMinutes int) (bool, string, error) {
	settings, err := s.complianceRepo.GetSettings(ctx)
	if err != nil {
		return false, "", err
	}

	if breakMinutes < settings.MinBreakSegmentMinutes {
		return false, fmt.Sprintf(
			"Pausenabschnitte müssen mindestens %d Minuten betragen (ArbZG §4)",
			settings.MinBreakSegmentMinutes,
		), nil
	}

	return true, "", nil
}

// ============================================================================
// CLOCK OUT VALIDATION
// ============================================================================

// ClockOutValidationResult contains compliance checks for clock out
type ClockOutValidationResult struct {
	Allowed          bool                          `json:"allowed"`
	Warnings         []string                      `json:"warnings"`
	Violations       []*repository.ComplianceViolation `json:"violations,omitempty"`
	TotalWorkMinutes int                           `json:"total_work_minutes"`
	TotalBreakMinutes int                          `json:"total_break_minutes"`
}

// CheckClockOutCompliance checks for compliance issues at clock out
func (s *ComplianceService) CheckClockOutCompliance(ctx context.Context, employeeID string) (*ClockOutValidationResult, error) {
	settings, err := s.complianceRepo.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	entry, err := s.timeTrackingRepo.GetActiveEntryByEmployeeID(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, errors.BadRequest("not clocked in")
	}

	now := time.Now()
	totalMinutes := int(now.Sub(entry.ClockIn).Minutes())

	breakMinutes, err := s.timeTrackingRepo.CalculateTotalBreakMinutes(ctx, entry.ID)
	if err != nil {
		return nil, err
	}

	workMinutes := totalMinutes - breakMinutes

	result := &ClockOutValidationResult{
		Allowed:          true,
		Warnings:         []string{},
		Violations:       []*repository.ComplianceViolation{},
		TotalWorkMinutes: workMinutes,
		TotalBreakMinutes: breakMinutes,
	}

	// Check 1: Daily max hours (ArbZG §3) - 10 hours max
	if workMinutes > settings.MaxDailyHours*60 {
		violation := &repository.ComplianceViolation{
			EmployeeID:    employeeID,
			ViolationType: repository.ViolationMaxDailyHoursExceeded,
			ViolationDate: now,
			TimeEntryID:   &entry.ID,
			ExpectedValue: ptrString(fmt.Sprintf("%d Stunden", settings.MaxDailyHours)),
			ActualValue:   ptrString(fmt.Sprintf("%d:%02d Stunden", workMinutes/60, workMinutes%60)),
			Description:   ptrString("Tägliche Höchstarbeitszeit überschritten (ArbZG §3)"),
		}
		result.Violations = append(result.Violations, violation)
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"Tägliche Höchstarbeitszeit überschritten: %d:%02d (max %d Stunden)",
			workMinutes/60, workMinutes%60, settings.MaxDailyHours,
		))
	}

	// Check 2: Break compliance (ArbZG §4)
	var requiredBreak int
	if workMinutes > 9*60 {
		requiredBreak = settings.MinBreak9hMinutes
	} else if workMinutes > 6*60 {
		requiredBreak = settings.MinBreak6hMinutes
	}

	if requiredBreak > 0 && breakMinutes < requiredBreak {
		violationType := repository.ViolationBreakTooShort6h
		if workMinutes > 9*60 {
			violationType = repository.ViolationBreakTooShort9h
		}

		violation := &repository.ComplianceViolation{
			EmployeeID:    employeeID,
			ViolationType: violationType,
			ViolationDate: now,
			TimeEntryID:   &entry.ID,
			ExpectedValue: ptrString(fmt.Sprintf("%d Minuten", requiredBreak)),
			ActualValue:   ptrString(fmt.Sprintf("%d Minuten", breakMinutes)),
			Description:   ptrString("Gesetzliche Mindestpause nicht eingehalten (ArbZG §4)"),
		}
		result.Violations = append(result.Violations, violation)
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"Mindestpause nicht eingehalten: %d Minuten (min %d Minuten erforderlich)",
			breakMinutes, requiredBreak,
		))
	}

	// Check 3: Missing break after 6 hours
	if workMinutes > 6*60 && breakMinutes == 0 {
		violation := &repository.ComplianceViolation{
			EmployeeID:    employeeID,
			ViolationType: repository.ViolationMissingBreak,
			ViolationDate: now,
			TimeEntryID:   &entry.ID,
			ExpectedValue: ptrString("Pause erforderlich"),
			ActualValue:   ptrString("Keine Pause"),
			Description:   ptrString("Keine Pause bei mehr als 6 Stunden Arbeit (ArbZG §4)"),
		}
		result.Violations = append(result.Violations, violation)
		result.Warnings = append(result.Warnings,
			"Keine Pause eingetragen bei mehr als 6 Stunden Arbeitszeit",
		)
	}

	return result, nil
}

// RecordClockOutViolations saves any violations found during clock out
func (s *ComplianceService) RecordClockOutViolations(ctx context.Context, employeeID string) error {
	result, err := s.CheckClockOutCompliance(ctx, employeeID)
	if err != nil {
		return err
	}

	for _, violation := range result.Violations {
		if err := s.complianceRepo.CreateViolation(ctx, violation); err != nil {
			s.logger.Error().Err(err).Str("type", violation.ViolationType).Msg("failed to create violation")
		}
	}

	return nil
}

// ============================================================================
// SHIFT PLANNING VALIDATION (ArbZG §5)
// ============================================================================

// ShiftValidationResult contains validation results for shift planning
type ShiftValidationResult struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings"`
	Errors   []string `json:"errors"`
}

// ValidateShiftAssignment validates a shift assignment against ArbZG
func (s *ComplianceService) ValidateShiftAssignment(ctx context.Context, employeeID string, shiftStart, shiftEnd time.Time) (*ShiftValidationResult, error) {
	settings, err := s.complianceRepo.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	result := &ShiftValidationResult{
		Valid:    true,
		Warnings: []string{},
		Errors:   []string{},
	}

	// Calculate shift duration
	shiftMinutes := int(shiftEnd.Sub(shiftStart).Minutes())
	shiftHours := float64(shiftMinutes) / 60

	// Check 1: Maximum daily hours
	if shiftMinutes > settings.MaxDailyHours*60 {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf(
			"Schicht überschreitet maximale Tagesarbeitszeit (%.1f Stunden, max %d Stunden erlaubt)",
			shiftHours, settings.MaxDailyHours,
		))
	}

	// Check 2: Warn if approaching max hours
	if shiftMinutes > (settings.MaxDailyHours-1)*60 && shiftMinutes <= settings.MaxDailyHours*60 {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"Schicht nähert sich der maximalen Tagesarbeitszeit (%.1f von %d Stunden)",
			shiftHours, settings.MaxDailyHours,
		))
	}

	// Check 3: Rest period from previous shift (ArbZG §5)
	// Get the employee's last completed shift/entry before this one
	previousEnd, err := s.getLastWorkEndTime(ctx, employeeID, shiftStart)
	if err != nil {
		return nil, err
	}

	if previousEnd != nil {
		restHours := shiftStart.Sub(*previousEnd).Hours()
		requiredRest := float64(settings.MinRestBetweenShiftsHours)

		if restHours < requiredRest {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf(
				"Ruhezeit nicht eingehalten: %.1f Stunden zwischen Schichten (min %.0f Stunden erforderlich, ArbZG §5)",
				restHours, requiredRest,
			))
		} else if restHours < requiredRest+1 {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"Ruhezeit knapp: %.1f Stunden zwischen Schichten (min %.0f Stunden empfohlen)",
				restHours, requiredRest,
			))
		}
	}

	// Check 4: Weekly hours check
	weekStart := getWeekStart(shiftStart)
	weekEnd := weekStart.AddDate(0, 0, 7)

	weeklyMinutes, err := s.getPlannedWeeklyMinutes(ctx, employeeID, weekStart, weekEnd)
	if err != nil {
		return nil, err
	}

	totalWeeklyWithNew := weeklyMinutes + shiftMinutes

	if totalWeeklyWithNew > settings.MaxWeeklyHours*60 {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf(
			"Wöchentliche Höchstarbeitszeit würde überschritten werden (%d:%02d von %d Stunden)",
			totalWeeklyWithNew/60, totalWeeklyWithNew%60, settings.MaxWeeklyHours,
		))
	} else if totalWeeklyWithNew > (settings.MaxWeeklyHours-4)*60 {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"Wochenarbeitszeit nähert sich dem Maximum (%d:%02d von %d Stunden)",
			totalWeeklyWithNew/60, totalWeeklyWithNew%60, settings.MaxWeeklyHours,
		))
	}

	return result, nil
}

// ============================================================================
// REAL-TIME COMPLIANCE MONITORING
// ============================================================================

// CheckActiveEmployeeCompliance checks compliance for a currently clocked-in employee
func (s *ComplianceService) CheckActiveEmployeeCompliance(ctx context.Context, employeeID string) error {
	settings, err := s.complianceRepo.GetSettings(ctx)
	if err != nil {
		return err
	}

	entry, err := s.timeTrackingRepo.GetActiveEntryByEmployeeID(ctx, employeeID)
	if err != nil {
		return err
	}
	if entry == nil {
		return nil // Not clocked in, nothing to check
	}

	now := time.Now()
	totalMinutes := int(now.Sub(entry.ClockIn).Minutes())

	breakMinutes, err := s.timeTrackingRepo.CalculateTotalBreakMinutes(ctx, entry.ID)
	if err != nil {
		return err
	}

	// Check if currently on break
	activeBreak, err := s.timeTrackingRepo.GetActiveBreak(ctx, entry.ID)
	if err != nil {
		return err
	}

	workMinutes := totalMinutes - breakMinutes

	// Alert 1: No break after 6 hours
	if activeBreak == nil && workMinutes >= settings.AlertNoBreakAfterMinutes && breakMinutes == 0 {
		alert := &repository.ComplianceAlert{
			EmployeeID:  employeeID,
			AlertType:   repository.AlertNoBreakWarning,
			Severity:    repository.SeverityWarning,
			Message:     fmt.Sprintf("Mitarbeiter arbeitet seit %d:%02d ohne Pause", workMinutes/60, workMinutes%60),
			ActionLabel: ptrString("Pause einleiten"),
		}

		// Deactivate old alerts of same type first
		s.complianceRepo.DeactivateAlertsForEmployee(ctx, employeeID, repository.AlertNoBreakWarning)

		if err := s.complianceRepo.CreateAlert(ctx, alert); err != nil {
			s.logger.Error().Err(err).Msg("failed to create no-break alert")
		}
	}

	// Alert 2: Break too long
	if activeBreak != nil {
		breakDuration := int(now.Sub(activeBreak.StartTime).Minutes())
		if breakDuration > settings.AlertBreakTooLongMinutes {
			alert := &repository.ComplianceAlert{
				EmployeeID:  employeeID,
				AlertType:   repository.AlertBreakTooLong,
				Severity:    repository.SeverityInfo,
				Message:     fmt.Sprintf("Mitarbeiter ist seit %d Minuten in Pause", breakDuration),
				ActionLabel: ptrString("Pause beenden"),
			}

			s.complianceRepo.DeactivateAlertsForEmployee(ctx, employeeID, repository.AlertBreakTooLong)

			if err := s.complianceRepo.CreateAlert(ctx, alert); err != nil {
				s.logger.Error().Err(err).Msg("failed to create break-too-long alert")
			}
		}
	}

	// Alert 3: Approaching max hours
	remainingMinutes := settings.MaxDailyHours*60 - workMinutes
	if remainingMinutes <= settings.AlertApproachingMaxHoursMinutes && remainingMinutes > 0 {
		alert := &repository.ComplianceAlert{
			EmployeeID:  employeeID,
			AlertType:   repository.AlertMaxHoursApproaching,
			Severity:    repository.SeverityWarning,
			Message:     fmt.Sprintf("Maximale Arbeitszeit in %d Minuten erreicht", remainingMinutes),
			ActionLabel: ptrString("Ausstempeln"),
		}

		s.complianceRepo.DeactivateAlertsForEmployee(ctx, employeeID, repository.AlertMaxHoursApproaching)

		if err := s.complianceRepo.CreateAlert(ctx, alert); err != nil {
			s.logger.Error().Err(err).Msg("failed to create approaching-max alert")
		}
	}

	// Alert 4: Max hours exceeded
	if workMinutes > settings.MaxDailyHours*60 {
		alert := &repository.ComplianceAlert{
			EmployeeID:  employeeID,
			AlertType:   repository.AlertMaxHoursExceeded,
			Severity:    repository.SeverityCritical,
			Message:     fmt.Sprintf("Maximale Arbeitszeit überschritten (%d:%02d)", workMinutes/60, workMinutes%60),
			ActionLabel: ptrString("Sofort ausstempeln"),
		}

		s.complianceRepo.DeactivateAlertsForEmployee(ctx, employeeID, repository.AlertMaxHoursExceeded)

		if err := s.complianceRepo.CreateAlert(ctx, alert); err != nil {
			s.logger.Error().Err(err).Msg("failed to create max-exceeded alert")
		}
	}

	return nil
}

// CheckAllActiveEmployees runs compliance checks for all clocked-in employees
func (s *ComplianceService) CheckAllActiveEmployees(ctx context.Context) error {
	statuses, err := s.timeTrackingRepo.GetAllEmployeeStatuses(ctx)
	if err != nil {
		return err
	}

	for _, status := range statuses {
		if status.Status != "clocked_out" {
			if err := s.CheckActiveEmployeeCompliance(ctx, status.EmployeeID); err != nil {
				s.logger.Error().Err(err).Str("employee_id", status.EmployeeID).Msg("failed compliance check")
			}
		}
	}

	return nil
}

// ============================================================================
// SETTINGS & DATA ACCESS
// ============================================================================

// GetSettings gets the tenant's compliance settings
func (s *ComplianceService) GetSettings(ctx context.Context) (*repository.ComplianceSettings, error) {
	return s.complianceRepo.GetSettings(ctx)
}

// UpdateSettings updates compliance settings (validates against legal minimums)
func (s *ComplianceService) UpdateSettings(ctx context.Context, settings *repository.ComplianceSettings) error {
	// Enforce legal minimums - cannot be less restrictive than law
	if settings.MinBreak6hMinutes < 30 {
		return errors.BadRequest("Mindestpause für 6h kann nicht unter 30 Minuten liegen (ArbZG §4)")
	}
	if settings.MinBreak9hMinutes < 45 {
		return errors.BadRequest("Mindestpause für 9h kann nicht unter 45 Minuten liegen (ArbZG §4)")
	}
	if settings.MinBreakSegmentMinutes < 15 {
		return errors.BadRequest("Pausenabschnitte können nicht unter 15 Minuten liegen (ArbZG §4)")
	}
	if settings.MaxDailyHours > 10 {
		return errors.BadRequest("Tägliche Höchstarbeitszeit darf 10 Stunden nicht überschreiten (ArbZG §3)")
	}
	if settings.MaxWeeklyHours > 48 {
		return errors.BadRequest("Wöchentliche Höchstarbeitszeit darf 48 Stunden nicht überschreiten (ArbZG §3)")
	}
	if settings.MinRestBetweenShiftsHours < 10 {
		return errors.BadRequest("Ruhezeit zwischen Schichten muss mindestens 10 Stunden betragen (ArbZG §5)")
	}

	return s.complianceRepo.UpdateSettings(ctx, settings)
}

// GetActiveAlerts gets all active compliance alerts
func (s *ComplianceService) GetActiveAlerts(ctx context.Context) ([]*repository.ComplianceAlert, error) {
	return s.complianceRepo.ListActiveAlerts(ctx)
}

// DismissAlert dismisses an alert
func (s *ComplianceService) DismissAlert(ctx context.Context, alertID, userID string) error {
	return s.complianceRepo.DismissAlert(ctx, alertID, userID)
}

// GetViolations gets violations with filters
func (s *ComplianceService) GetViolations(ctx context.Context, employeeID *string, startDate, endDate *time.Time, status *string) ([]*repository.ComplianceViolation, error) {
	return s.complianceRepo.ListViolations(ctx, employeeID, startDate, endDate, status)
}

// AcknowledgeViolation marks a violation as acknowledged
func (s *ComplianceService) AcknowledgeViolation(ctx context.Context, violationID, userID string) error {
	return s.complianceRepo.AcknowledgeViolation(ctx, violationID, userID)
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// getLastWorkEndTime finds the last clock out or shift end time before a given time
func (s *ComplianceService) getLastWorkEndTime(ctx context.Context, employeeID string, before time.Time) (*time.Time, error) {
	// Check time entries first
	startDate := before.AddDate(0, 0, -7) // Look back 7 days
	entries, err := s.timeTrackingRepo.ListEntriesForEmployee(ctx, employeeID, startDate, before)
	if err != nil {
		return nil, err
	}

	var lastEnd *time.Time
	for _, entry := range entries {
		if entry.ClockOut != nil && entry.ClockOut.Before(before) {
			if lastEnd == nil || entry.ClockOut.After(*lastEnd) {
				clockOut := *entry.ClockOut
				lastEnd = &clockOut
			}
		}
	}

	// Also check shift assignments
	if s.shiftRepo != nil {
		assignments, err := s.shiftRepo.ListAssignmentsByEmployeeAndDateRange(ctx, employeeID, startDate, before)
		if err != nil {
			return nil, err
		}

		for _, assignment := range assignments {
			if assignment.ShiftEnd.Before(before) {
				if lastEnd == nil || assignment.ShiftEnd.After(*lastEnd) {
					endTime := assignment.ShiftEnd
					lastEnd = &endTime
				}
			}
		}
	}

	return lastEnd, nil
}

// getPlannedWeeklyMinutes calculates total planned shift minutes for a week
func (s *ComplianceService) getPlannedWeeklyMinutes(ctx context.Context, employeeID string, weekStart, weekEnd time.Time) (int, error) {
	totalMinutes := 0

	// Get shift assignments
	if s.shiftRepo != nil {
		assignments, err := s.shiftRepo.ListAssignmentsByEmployeeAndDateRange(ctx, employeeID, weekStart, weekEnd)
		if err != nil {
			return 0, err
		}

		for _, assignment := range assignments {
			shiftMinutes := int(assignment.ShiftEnd.Sub(assignment.ShiftStart).Minutes())
			totalMinutes += shiftMinutes
		}
	}

	// Also add actual time entries
	entries, err := s.timeTrackingRepo.ListEntriesForEmployee(ctx, employeeID, weekStart, weekEnd)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		totalMinutes += entry.TotalWorkMinutes
	}

	return totalMinutes, nil
}

// getWeekStart returns the Monday of the week containing the given time
func getWeekStart(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday
	}
	daysFromMonday := weekday - 1
	return time.Date(t.Year(), t.Month(), t.Day()-daysFromMonday, 0, 0, 0, 0, t.Location())
}

// ptrString returns a pointer to a string
func ptrString(s string) *string {
	return &s
}

// ============================================================================
// TIME CORRECTION REQUESTS
// ============================================================================

// CreateCorrectionRequest creates a new time correction request from an employee
func (s *ComplianceService) CreateCorrectionRequest(ctx context.Context, req *repository.CorrectionRequest) error {
	// Validate required fields
	if req.EmployeeID == "" {
		return errors.BadRequest("employee_id is required")
	}
	if req.RequestType == "" {
		return errors.BadRequest("request_type is required")
	}
	if req.Reason == "" {
		return errors.BadRequest("reason is required for audit purposes")
	}

	return s.complianceRepo.CreateCorrectionRequest(ctx, req)
}

// GetCorrectionRequest gets a correction request by ID
func (s *ComplianceService) GetCorrectionRequest(ctx context.Context, id string) (*repository.CorrectionRequest, error) {
	return s.complianceRepo.GetCorrectionRequestByID(ctx, id)
}

// ListPendingCorrectionRequests lists all pending correction requests (for managers)
func (s *ComplianceService) ListPendingCorrectionRequests(ctx context.Context) ([]*repository.CorrectionRequest, error) {
	return s.complianceRepo.ListPendingCorrectionRequests(ctx)
}

// ListEmployeeCorrectionRequests lists correction requests for an employee
func (s *ComplianceService) ListEmployeeCorrectionRequests(ctx context.Context, employeeID string) ([]*repository.CorrectionRequest, error) {
	return s.complianceRepo.ListCorrectionRequestsByEmployee(ctx, employeeID)
}

// ApproveCorrectionRequest approves a correction request and applies the changes
func (s *ComplianceService) ApproveCorrectionRequest(ctx context.Context, requestID string, reviewerID string) error {
	// Get the request
	req, err := s.complianceRepo.GetCorrectionRequestByID(ctx, requestID)
	if err != nil {
		return err
	}

	if req.Status != repository.CorrectionStatusPending {
		return errors.BadRequest("request is not pending")
	}

	// Apply the correction based on type
	switch req.RequestType {
	case repository.CorrectionTypeClockIn:
		if req.RequestedClockIn != nil && req.TimeEntryID != nil {
			entry, err := s.timeTrackingRepo.GetEntryByID(ctx, *req.TimeEntryID)
			if err != nil {
				return err
			}
			entry.ClockIn = *req.RequestedClockIn
			entry.IsManualEntry = true
			entry.UpdatedBy = &reviewerID
			if err := s.timeTrackingRepo.UpdateEntry(ctx, entry); err != nil {
				return err
			}
		}

	case repository.CorrectionTypeClockOut:
		if req.RequestedClockOut != nil && req.TimeEntryID != nil {
			entry, err := s.timeTrackingRepo.GetEntryByID(ctx, *req.TimeEntryID)
			if err != nil {
				return err
			}
			entry.ClockOut = req.RequestedClockOut
			entry.IsManualEntry = true
			entry.UpdatedBy = &reviewerID

			// Recalculate totals
			totalBreakMinutes, err := s.timeTrackingRepo.CalculateTotalBreakMinutes(ctx, entry.ID)
			if err != nil {
				return err
			}
			entry.TotalBreakMinutes = totalBreakMinutes

			if entry.ClockOut != nil {
				totalMinutes := int(entry.ClockOut.Sub(entry.ClockIn).Minutes())
				entry.TotalWorkMinutes = totalMinutes - totalBreakMinutes
				if entry.TotalWorkMinutes < 0 {
					entry.TotalWorkMinutes = 0
				}
			}

			if err := s.timeTrackingRepo.UpdateEntry(ctx, entry); err != nil {
				return err
			}
		}

	case repository.CorrectionTypeMissedEntry:
		if req.RequestedClockIn != nil {
			entry := &repository.TimeEntry{
				EmployeeID:    req.EmployeeID,
				EntryDate:     req.RequestedDate,
				ClockIn:       *req.RequestedClockIn,
				IsManualEntry: true,
				CreatedBy:     &reviewerID,
			}
			if req.RequestedClockOut != nil {
				entry.ClockOut = req.RequestedClockOut
				totalMinutes := int(req.RequestedClockOut.Sub(*req.RequestedClockIn).Minutes())
				entry.TotalWorkMinutes = totalMinutes
			}
			if err := s.timeTrackingRepo.CreateEntry(ctx, entry); err != nil {
				return err
			}
		}

	case repository.CorrectionTypeDeleteEntry:
		if req.TimeEntryID != nil {
			if err := s.timeTrackingRepo.SoftDeleteEntry(ctx, *req.TimeEntryID); err != nil {
				return err
			}
		}
	}

	// Update request status
	return s.complianceRepo.UpdateCorrectionRequestStatus(ctx, requestID, repository.CorrectionStatusApproved, reviewerID, nil)
}

// RejectCorrectionRequest rejects a correction request
func (s *ComplianceService) RejectCorrectionRequest(ctx context.Context, requestID string, reviewerID string, reason string) error {
	// Get the request
	req, err := s.complianceRepo.GetCorrectionRequestByID(ctx, requestID)
	if err != nil {
		return err
	}

	if req.Status != repository.CorrectionStatusPending {
		return errors.BadRequest("request is not pending")
	}

	return s.complianceRepo.UpdateCorrectionRequestStatus(ctx, requestID, repository.CorrectionStatusRejected, reviewerID, &reason)
}
