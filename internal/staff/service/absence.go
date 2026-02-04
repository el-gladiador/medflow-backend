package service

import (
	"context"
	"math"
	"time"

	"github.com/medflow/medflow-backend/internal/staff/events"
	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// AbsenceService handles absence-related business logic
type AbsenceService struct {
	absenceRepo *repository.AbsenceRepository
	publisher   *events.StaffEventPublisher
	logger      *logger.Logger
}

// NewAbsenceService creates a new absence service
func NewAbsenceService(
	absenceRepo *repository.AbsenceRepository,
	publisher *events.StaffEventPublisher,
	log *logger.Logger,
) *AbsenceService {
	return &AbsenceService{
		absenceRepo: absenceRepo,
		publisher:   publisher,
		logger:      log,
	}
}

// ============================================================================
// ABSENCES
// ============================================================================

// Create creates a new absence request
func (s *AbsenceService) Create(ctx context.Context, absence *repository.Absence) error {
	// Set requested_at if not set
	if absence.RequestedAt.IsZero() {
		absence.RequestedAt = time.Now()
	}

	// Calculate vacation days if this is a vacation request
	if absence.AbsenceType == "vacation" {
		days := s.calculateWorkingDays(absence.StartDate, absence.EndDate)
		absence.VacationDaysUsed = &days
	}

	if err := s.absenceRepo.Create(ctx, absence); err != nil {
		return err
	}

	// Update vacation balance pending count if it's a vacation
	if absence.AbsenceType == "vacation" && absence.VacationDaysUsed != nil {
		year := absence.StartDate.Year()
		if err := s.updatePendingVacation(ctx, absence.EmployeeID, year); err != nil {
			s.logger.Error().Err(err).Msg("failed to update pending vacation balance")
		}
	}

	// Publish event
	s.publisher.PublishAbsenceCreated(ctx, absence)

	s.logger.Info().
		Str("absence_id", absence.ID).
		Str("employee_id", absence.EmployeeID).
		Str("type", absence.AbsenceType).
		Time("start_date", absence.StartDate).
		Time("end_date", absence.EndDate).
		Msg("absence created")

	return nil
}

// GetByID gets an absence by ID
func (s *AbsenceService) GetByID(ctx context.Context, id string) (*repository.Absence, error) {
	return s.absenceRepo.GetByID(ctx, id)
}

// List lists absences with filters
func (s *AbsenceService) List(ctx context.Context, params repository.AbsenceListParams) ([]*repository.Absence, int64, error) {
	return s.absenceRepo.List(ctx, params)
}

// GetEmployeeAbsences gets absences for an employee in a date range
func (s *AbsenceService) GetEmployeeAbsences(ctx context.Context, employeeID string, startDate, endDate time.Time) ([]*repository.Absence, error) {
	return s.absenceRepo.GetAbsencesForDateRange(ctx, employeeID, startDate, endDate)
}

// ListPending lists all pending absence requests
func (s *AbsenceService) ListPending(ctx context.Context) ([]*repository.Absence, error) {
	return s.absenceRepo.ListPending(ctx)
}

// Update updates an absence
func (s *AbsenceService) Update(ctx context.Context, absence *repository.Absence) error {
	// Recalculate vacation days if this is a vacation request
	if absence.AbsenceType == "vacation" {
		days := s.calculateWorkingDays(absence.StartDate, absence.EndDate)
		absence.VacationDaysUsed = &days
	}

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return err
	}

	// Update vacation balance if it's a vacation
	if absence.AbsenceType == "vacation" {
		year := absence.StartDate.Year()
		if err := s.RecalculateBalance(ctx, absence.EmployeeID, year); err != nil {
			s.logger.Error().Err(err).Msg("failed to recalculate vacation balance")
		}
	}

	// Publish event
	s.publisher.PublishAbsenceUpdated(ctx, absence)

	s.logger.Info().
		Str("absence_id", absence.ID).
		Msg("absence updated")

	return nil
}

// Approve approves an absence request
func (s *AbsenceService) Approve(ctx context.Context, id, reviewerID string, note *string) error {
	// Get the absence first
	absence, err := s.absenceRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.absenceRepo.Approve(ctx, id, reviewerID, note); err != nil {
		return err
	}

	// Update vacation balance if it's a vacation
	if absence.AbsenceType == "vacation" {
		year := absence.StartDate.Year()
		if err := s.RecalculateBalance(ctx, absence.EmployeeID, year); err != nil {
			s.logger.Error().Err(err).Msg("failed to recalculate vacation balance after approval")
		}
	}

	// Publish event
	s.publisher.PublishAbsenceApproved(ctx, id, reviewerID)

	s.logger.Info().
		Str("absence_id", id).
		Str("reviewer_id", reviewerID).
		Msg("absence approved")

	return nil
}

// Reject rejects an absence request
func (s *AbsenceService) Reject(ctx context.Context, id, reviewerID, reason string, note *string) error {
	// Get the absence first
	absence, err := s.absenceRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.absenceRepo.Reject(ctx, id, reviewerID, reason, note); err != nil {
		return err
	}

	// Update vacation balance if it's a vacation
	if absence.AbsenceType == "vacation" {
		year := absence.StartDate.Year()
		if err := s.RecalculateBalance(ctx, absence.EmployeeID, year); err != nil {
			s.logger.Error().Err(err).Msg("failed to recalculate vacation balance after rejection")
		}
	}

	// Publish event
	s.publisher.PublishAbsenceRejected(ctx, id, reviewerID, reason)

	s.logger.Info().
		Str("absence_id", id).
		Str("reviewer_id", reviewerID).
		Str("reason", reason).
		Msg("absence rejected")

	return nil
}

// Delete deletes an absence
func (s *AbsenceService) Delete(ctx context.Context, id string) error {
	// Get the absence first
	absence, err := s.absenceRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.absenceRepo.Delete(ctx, id); err != nil {
		return err
	}

	// Update vacation balance if it's a vacation
	if absence.AbsenceType == "vacation" {
		year := absence.StartDate.Year()
		if err := s.RecalculateBalance(ctx, absence.EmployeeID, year); err != nil {
			s.logger.Error().Err(err).Msg("failed to recalculate vacation balance after deletion")
		}
	}

	// Publish event
	s.publisher.PublishAbsenceDeleted(ctx, id)

	s.logger.Info().
		Str("absence_id", id).
		Msg("absence deleted")

	return nil
}

// ============================================================================
// VACATION BALANCE
// ============================================================================

// GetVacationBalance gets the vacation balance for an employee for a year
func (s *AbsenceService) GetVacationBalance(ctx context.Context, employeeID string, year int) (*repository.VacationBalance, error) {
	balance, err := s.absenceRepo.GetVacationBalance(ctx, employeeID, year)
	if err != nil {
		return nil, err
	}

	// If no balance exists, return a default balance
	if balance == nil {
		balance = &repository.VacationBalance{
			EmployeeID:            employeeID,
			Year:                  year,
			AnnualEntitlement:     30, // German default: 30 days for full-time
			CarryoverFromPrevious: 0,
			AdditionalGranted:     0,
			Taken:                 0,
			Planned:               0,
			Pending:               0,
		}
	}

	return balance, nil
}

// SetVacationEntitlement sets or updates the vacation entitlement for an employee
func (s *AbsenceService) SetVacationEntitlement(ctx context.Context, employeeID string, year int, entitlement float64) error {
	// Get existing balance or create new
	balance, err := s.GetVacationBalance(ctx, employeeID, year)
	if err != nil {
		return err
	}

	balance.AnnualEntitlement = entitlement

	if err := s.absenceRepo.CreateOrUpdateVacationBalance(ctx, balance); err != nil {
		return err
	}

	s.logger.Info().
		Str("employee_id", employeeID).
		Int("year", year).
		Float64("entitlement", entitlement).
		Msg("vacation entitlement set")

	return nil
}

// RecalculateBalance recalculates the vacation balance based on absences
func (s *AbsenceService) RecalculateBalance(ctx context.Context, employeeID string, year int) error {
	// Get all vacation absences for the year
	startOfYear := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	endOfYear := time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC)

	params := repository.AbsenceListParams{
		EmployeeID:  &employeeID,
		StartDate:   &startOfYear,
		EndDate:     &endOfYear,
		AbsenceType: strPtr("vacation"),
		PerPage:     1000, // Get all
	}

	absences, _, err := s.absenceRepo.List(ctx, params)
	if err != nil {
		return err
	}

	var taken, planned, pending float64
	now := time.Now()

	for _, a := range absences {
		days := 0.0
		if a.VacationDaysUsed != nil {
			days = *a.VacationDaysUsed
		} else {
			days = s.calculateWorkingDays(a.StartDate, a.EndDate)
		}

		switch a.Status {
		case "approved":
			if a.EndDate.Before(now) {
				taken += days
			} else {
				planned += days
			}
		case "pending":
			pending += days
		}
	}

	// Get existing balance or create new
	balance, err := s.GetVacationBalance(ctx, employeeID, year)
	if err != nil {
		return err
	}

	balance.Taken = taken
	balance.Planned = planned
	balance.Pending = pending

	return s.absenceRepo.CreateOrUpdateVacationBalance(ctx, balance)
}

// ListVacationBalances lists vacation balances for all employees for a year
func (s *AbsenceService) ListVacationBalances(ctx context.Context, year int) ([]*repository.VacationBalance, error) {
	// This would need a new repository method to list all balances
	// For now, return empty - will need to implement in repository
	return []*repository.VacationBalance{}, nil
}

// ============================================================================
// COMPLIANCE
// ============================================================================

// CreateComplianceLog creates a compliance violation log entry
func (s *AbsenceService) CreateComplianceLog(ctx context.Context, log *repository.ArbzgComplianceLog) error {
	return s.absenceRepo.CreateComplianceLog(ctx, log)
}

// ListComplianceLogs lists compliance violations
func (s *AbsenceService) ListComplianceLogs(ctx context.Context, employeeID *string, startDate, endDate *time.Time, unacknowledgedOnly bool, page, perPage int) ([]*repository.ArbzgComplianceLog, int64, error) {
	return s.absenceRepo.ListComplianceLogs(ctx, employeeID, startDate, endDate, unacknowledgedOnly, page, perPage)
}

// AcknowledgeViolation acknowledges a compliance violation
func (s *AbsenceService) AcknowledgeViolation(ctx context.Context, id, acknowledgedBy string, resolutionNote *string) error {
	return s.absenceRepo.AcknowledgeViolation(ctx, id, acknowledgedBy, resolutionNote)
}

// ============================================================================
// HELPERS
// ============================================================================

// calculateWorkingDays calculates the number of working days between two dates
// Excludes weekends (Saturday and Sunday)
func (s *AbsenceService) calculateWorkingDays(startDate, endDate time.Time) float64 {
	if endDate.Before(startDate) {
		return 0
	}

	days := 0.0
	current := startDate

	for !current.After(endDate) {
		weekday := current.Weekday()
		if weekday != time.Saturday && weekday != time.Sunday {
			days++
		}
		current = current.AddDate(0, 0, 1)
	}

	// Round to 2 decimal places
	return math.Round(days*100) / 100
}

// updatePendingVacation updates the pending vacation count
func (s *AbsenceService) updatePendingVacation(ctx context.Context, employeeID string, year int) error {
	return s.RecalculateBalance(ctx, employeeID, year)
}

func strPtr(s string) *string {
	return &s
}
