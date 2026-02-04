package service

import (
	"context"
	"time"

	"github.com/medflow/medflow-backend/internal/staff/events"
	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// ShiftService handles shift-related business logic
type ShiftService struct {
	shiftRepo *repository.ShiftRepository
	publisher *events.StaffEventPublisher
	logger    *logger.Logger
}

// NewShiftService creates a new shift service
func NewShiftService(
	shiftRepo *repository.ShiftRepository,
	publisher *events.StaffEventPublisher,
	log *logger.Logger,
) *ShiftService {
	return &ShiftService{
		shiftRepo: shiftRepo,
		publisher: publisher,
		logger:    log,
	}
}

// ============================================================================
// SHIFT TEMPLATES
// ============================================================================

// CreateTemplate creates a new shift template
func (s *ShiftService) CreateTemplate(ctx context.Context, tmpl *repository.ShiftTemplate) error {
	if err := s.shiftRepo.CreateTemplate(ctx, tmpl); err != nil {
		return err
	}

	s.logger.Info().
		Str("template_id", tmpl.ID).
		Str("name", tmpl.Name).
		Msg("shift template created")

	return nil
}

// GetTemplateByID gets a shift template by ID
func (s *ShiftService) GetTemplateByID(ctx context.Context, id string) (*repository.ShiftTemplate, error) {
	return s.shiftRepo.GetTemplateByID(ctx, id)
}

// ListTemplates lists all shift templates
func (s *ShiftService) ListTemplates(ctx context.Context, activeOnly bool) ([]*repository.ShiftTemplate, error) {
	return s.shiftRepo.ListTemplates(ctx, activeOnly)
}

// UpdateTemplate updates a shift template
func (s *ShiftService) UpdateTemplate(ctx context.Context, tmpl *repository.ShiftTemplate) error {
	if err := s.shiftRepo.UpdateTemplate(ctx, tmpl); err != nil {
		return err
	}

	s.logger.Info().
		Str("template_id", tmpl.ID).
		Str("name", tmpl.Name).
		Msg("shift template updated")

	return nil
}

// DeleteTemplate deletes a shift template
func (s *ShiftService) DeleteTemplate(ctx context.Context, id string) error {
	if err := s.shiftRepo.DeleteTemplate(ctx, id); err != nil {
		return err
	}

	s.logger.Info().
		Str("template_id", id).
		Msg("shift template deleted")

	return nil
}

// ============================================================================
// SHIFT ASSIGNMENTS
// ============================================================================

// CreateAssignment creates a new shift assignment with conflict detection
func (s *ShiftService) CreateAssignment(ctx context.Context, shift *repository.ShiftAssignment) error {
	// Check for conflicts
	hasConflict, reason, err := s.shiftRepo.CheckForConflicts(
		ctx,
		shift.EmployeeID,
		shift.ShiftDate,
		shift.StartTime,
		shift.EndTime,
		nil,
	)
	if err != nil {
		return err
	}

	if hasConflict {
		shift.HasConflict = true
		shift.ConflictReason = &reason
	}

	if err := s.shiftRepo.CreateAssignment(ctx, shift); err != nil {
		return err
	}

	// Publish event
	s.publisher.PublishShiftCreated(ctx, shift)

	s.logger.Info().
		Str("shift_id", shift.ID).
		Str("employee_id", shift.EmployeeID).
		Time("shift_date", shift.ShiftDate).
		Bool("has_conflict", shift.HasConflict).
		Msg("shift assignment created")

	return nil
}

// GetAssignmentByID gets a shift assignment by ID
func (s *ShiftService) GetAssignmentByID(ctx context.Context, id string) (*repository.ShiftAssignment, error) {
	return s.shiftRepo.GetAssignmentByID(ctx, id)
}

// ListAssignments lists shift assignments with filters
func (s *ShiftService) ListAssignments(ctx context.Context, params repository.ShiftListParams) ([]*repository.ShiftAssignment, int64, error) {
	return s.shiftRepo.ListAssignments(ctx, params)
}

// GetEmployeeShifts gets shifts for an employee in a date range
func (s *ShiftService) GetEmployeeShifts(ctx context.Context, employeeID string, startDate, endDate time.Time) ([]*repository.ShiftAssignment, error) {
	return s.shiftRepo.GetEmployeeShiftsForDateRange(ctx, employeeID, startDate, endDate)
}

// GetAssignmentsForDate gets all shifts for a specific date
func (s *ShiftService) GetAssignmentsForDate(ctx context.Context, date time.Time) ([]*repository.ShiftAssignment, error) {
	return s.shiftRepo.GetAssignmentsForDate(ctx, date)
}

// UpdateAssignment updates a shift assignment with conflict detection
func (s *ShiftService) UpdateAssignment(ctx context.Context, shift *repository.ShiftAssignment) error {
	// Check for conflicts (exclude current shift)
	hasConflict, reason, err := s.shiftRepo.CheckForConflicts(
		ctx,
		shift.EmployeeID,
		shift.ShiftDate,
		shift.StartTime,
		shift.EndTime,
		&shift.ID,
	)
	if err != nil {
		return err
	}

	if hasConflict {
		shift.HasConflict = true
		shift.ConflictReason = &reason
	} else {
		shift.HasConflict = false
		shift.ConflictReason = nil
	}

	if err := s.shiftRepo.UpdateAssignment(ctx, shift); err != nil {
		return err
	}

	// Publish event
	s.publisher.PublishShiftUpdated(ctx, shift)

	s.logger.Info().
		Str("shift_id", shift.ID).
		Str("employee_id", shift.EmployeeID).
		Time("shift_date", shift.ShiftDate).
		Bool("has_conflict", shift.HasConflict).
		Msg("shift assignment updated")

	return nil
}

// DeleteAssignment deletes a shift assignment
func (s *ShiftService) DeleteAssignment(ctx context.Context, id string) error {
	// Get the shift first for event publishing
	shift, err := s.shiftRepo.GetAssignmentByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.shiftRepo.DeleteAssignment(ctx, id); err != nil {
		return err
	}

	// Publish event
	s.publisher.PublishShiftDeleted(ctx, shift)

	s.logger.Info().
		Str("shift_id", id).
		Msg("shift assignment deleted")

	return nil
}

// BulkCreateAssignments creates multiple shift assignments
func (s *ShiftService) BulkCreateAssignments(ctx context.Context, shifts []*repository.ShiftAssignment) error {
	// Check for conflicts for each shift
	for _, shift := range shifts {
		hasConflict, reason, err := s.shiftRepo.CheckForConflicts(
			ctx,
			shift.EmployeeID,
			shift.ShiftDate,
			shift.StartTime,
			shift.EndTime,
			nil,
		)
		if err != nil {
			return err
		}

		if hasConflict {
			shift.HasConflict = true
			shift.ConflictReason = &reason
		}
	}

	if err := s.shiftRepo.BulkCreateAssignments(ctx, shifts); err != nil {
		return err
	}

	// Publish events for each shift
	for _, shift := range shifts {
		s.publisher.PublishShiftCreated(ctx, shift)
	}

	s.logger.Info().
		Int("count", len(shifts)).
		Msg("bulk shift assignments created")

	return nil
}
