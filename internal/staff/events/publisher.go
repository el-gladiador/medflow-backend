package events

import (
	"context"

	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/messaging"
)

// StaffEventPublisher publishes staff-related events
type StaffEventPublisher struct {
	publisher *messaging.Publisher
	logger    *logger.Logger
}

// NewStaffEventPublisher creates a new staff event publisher
func NewStaffEventPublisher(rmq *messaging.RabbitMQ, log *logger.Logger) (*StaffEventPublisher, error) {
	publisher, err := messaging.NewPublisher(rmq, messaging.ExchangeStaffEvents, "staff-service", log)
	if err != nil {
		return nil, err
	}

	return &StaffEventPublisher{
		publisher: publisher,
		logger:    log,
	}, nil
}

// PublishEmployeeCreated publishes an employee created event
func (p *StaffEventPublisher) PublishEmployeeCreated(ctx context.Context, emp *repository.Employee) {
	data := messaging.EmployeeCreatedEvent{
		EmployeeID: emp.ID,
		UserID:     emp.UserID,
		Name:       emp.FirstName + " " + emp.LastName,
	}

	if err := p.publisher.Publish(ctx, messaging.EventEmployeeCreated, data); err != nil {
		p.logger.Error().Err(err).Str("employee_id", emp.ID).Msg("failed to publish employee created event")
	}
}

// PublishEmployeeUpdated publishes an employee updated event
func (p *StaffEventPublisher) PublishEmployeeUpdated(ctx context.Context, emp *repository.Employee) {
	data := messaging.EmployeeUpdatedEvent{
		EmployeeID: emp.ID,
		Fields:     map[string]any{"name": emp.FirstName + " " + emp.LastName},
	}

	if err := p.publisher.Publish(ctx, messaging.EventEmployeeUpdated, data); err != nil {
		p.logger.Error().Err(err).Str("employee_id", emp.ID).Msg("failed to publish employee updated event")
	}
}

// PublishEmployeeDeleted publishes an employee deleted event
func (p *StaffEventPublisher) PublishEmployeeDeleted(ctx context.Context, employeeID string) {
	data := messaging.EmployeeDeletedEvent{
		EmployeeID: employeeID,
	}

	if err := p.publisher.Publish(ctx, messaging.EventEmployeeDeleted, data); err != nil {
		p.logger.Error().Err(err).Str("employee_id", employeeID).Msg("failed to publish employee deleted event")
	}
}

// ============================================================================
// SHIFT EVENTS
// ============================================================================

// PublishShiftCreated publishes a shift created event
func (p *StaffEventPublisher) PublishShiftCreated(ctx context.Context, shift *repository.ShiftAssignment) {
	data := messaging.ShiftCreatedEvent{
		ShiftID:    shift.ID,
		EmployeeID: shift.EmployeeID,
		ShiftDate:  shift.ShiftDate,
		StartTime:  shift.StartTime,
		EndTime:    shift.EndTime,
		ShiftType:  shift.ShiftType,
	}

	if err := p.publisher.Publish(ctx, messaging.EventShiftCreated, data); err != nil {
		p.logger.Error().Err(err).Str("shift_id", shift.ID).Msg("failed to publish shift created event")
	}
}

// PublishShiftUpdated publishes a shift updated event
func (p *StaffEventPublisher) PublishShiftUpdated(ctx context.Context, shift *repository.ShiftAssignment) {
	data := messaging.ShiftUpdatedEvent{
		ShiftID:    shift.ID,
		EmployeeID: shift.EmployeeID,
		Fields: map[string]any{
			"shift_date": shift.ShiftDate,
			"start_time": shift.StartTime,
			"end_time":   shift.EndTime,
			"status":     shift.Status,
		},
	}

	if err := p.publisher.Publish(ctx, messaging.EventShiftUpdated, data); err != nil {
		p.logger.Error().Err(err).Str("shift_id", shift.ID).Msg("failed to publish shift updated event")
	}
}

// PublishShiftDeleted publishes a shift deleted event
func (p *StaffEventPublisher) PublishShiftDeleted(ctx context.Context, shift *repository.ShiftAssignment) {
	data := messaging.ShiftDeletedEvent{
		ShiftID:    shift.ID,
		EmployeeID: shift.EmployeeID,
	}

	if err := p.publisher.Publish(ctx, messaging.EventShiftDeleted, data); err != nil {
		p.logger.Error().Err(err).Str("shift_id", shift.ID).Msg("failed to publish shift deleted event")
	}
}

// ============================================================================
// ABSENCE EVENTS
// ============================================================================

// PublishAbsenceCreated publishes an absence created event
func (p *StaffEventPublisher) PublishAbsenceCreated(ctx context.Context, absence *repository.Absence) {
	data := messaging.AbsenceCreatedEvent{
		AbsenceID:   absence.ID,
		EmployeeID:  absence.EmployeeID,
		AbsenceType: absence.AbsenceType,
		StartDate:   absence.StartDate,
		EndDate:     absence.EndDate,
		Status:      absence.Status,
	}

	if err := p.publisher.Publish(ctx, messaging.EventAbsenceCreated, data); err != nil {
		p.logger.Error().Err(err).Str("absence_id", absence.ID).Msg("failed to publish absence created event")
	}
}

// PublishAbsenceUpdated publishes an absence updated event
func (p *StaffEventPublisher) PublishAbsenceUpdated(ctx context.Context, absence *repository.Absence) {
	data := messaging.AbsenceUpdatedEvent{
		AbsenceID:  absence.ID,
		EmployeeID: absence.EmployeeID,
		Fields: map[string]any{
			"start_date":   absence.StartDate,
			"end_date":     absence.EndDate,
			"absence_type": absence.AbsenceType,
			"status":       absence.Status,
		},
	}

	if err := p.publisher.Publish(ctx, messaging.EventAbsenceUpdated, data); err != nil {
		p.logger.Error().Err(err).Str("absence_id", absence.ID).Msg("failed to publish absence updated event")
	}
}

// PublishAbsenceApproved publishes an absence approved event
func (p *StaffEventPublisher) PublishAbsenceApproved(ctx context.Context, absenceID, reviewerID string) {
	data := messaging.AbsenceApprovedEvent{
		AbsenceID:  absenceID,
		ReviewerID: reviewerID,
	}

	if err := p.publisher.Publish(ctx, messaging.EventAbsenceApproved, data); err != nil {
		p.logger.Error().Err(err).Str("absence_id", absenceID).Msg("failed to publish absence approved event")
	}
}

// PublishAbsenceRejected publishes an absence rejected event
func (p *StaffEventPublisher) PublishAbsenceRejected(ctx context.Context, absenceID, reviewerID, reason string) {
	data := messaging.AbsenceRejectedEvent{
		AbsenceID:  absenceID,
		ReviewerID: reviewerID,
		Reason:     reason,
	}

	if err := p.publisher.Publish(ctx, messaging.EventAbsenceRejected, data); err != nil {
		p.logger.Error().Err(err).Str("absence_id", absenceID).Msg("failed to publish absence rejected event")
	}
}

// PublishAbsenceDeleted publishes an absence deleted event
func (p *StaffEventPublisher) PublishAbsenceDeleted(ctx context.Context, absenceID string) {
	data := messaging.AbsenceDeletedEvent{
		AbsenceID: absenceID,
	}

	if err := p.publisher.Publish(ctx, messaging.EventAbsenceDeleted, data); err != nil {
		p.logger.Error().Err(err).Str("absence_id", absenceID).Msg("failed to publish absence deleted event")
	}
}

// ============================================================================
// TIME TRACKING EVENTS
// ============================================================================

// PublishTimeClockIn publishes a time clock in event
func (p *StaffEventPublisher) PublishTimeClockIn(ctx context.Context, entry *repository.TimeEntry) {
	data := messaging.TimeClockInEvent{
		TimeEntryID: entry.ID,
		EmployeeID:  entry.EmployeeID,
		ClockIn:     entry.ClockIn,
		IsManual:    entry.IsManualEntry,
	}

	if err := p.publisher.Publish(ctx, messaging.EventTimeClockIn, data); err != nil {
		p.logger.Error().Err(err).Str("time_entry_id", entry.ID).Msg("failed to publish time clock in event")
	}
}

// PublishTimeClockOut publishes a time clock out event
func (p *StaffEventPublisher) PublishTimeClockOut(ctx context.Context, entry *repository.TimeEntry) {
	if entry.ClockOut == nil {
		p.logger.Warn().Str("time_entry_id", entry.ID).Msg("attempted to publish clock out event with nil clock_out")
		return
	}

	data := messaging.TimeClockOutEvent{
		TimeEntryID:       entry.ID,
		EmployeeID:        entry.EmployeeID,
		ClockIn:           entry.ClockIn,
		ClockOut:          *entry.ClockOut,
		TotalWorkMinutes:  entry.TotalWorkMinutes,
		TotalBreakMinutes: entry.TotalBreakMinutes,
		IsManual:          entry.IsManualEntry,
	}

	if err := p.publisher.Publish(ctx, messaging.EventTimeClockOut, data); err != nil {
		p.logger.Error().Err(err).Str("time_entry_id", entry.ID).Msg("failed to publish time clock out event")
	}
}

// PublishTimeBreakStart publishes a time break start event
func (p *StaffEventPublisher) PublishTimeBreakStart(ctx context.Context, brk *repository.TimeBreak, employeeID string) {
	data := messaging.TimeBreakStartEvent{
		TimeBreakID: brk.ID,
		TimeEntryID: brk.TimeEntryID,
		EmployeeID:  employeeID,
		StartTime:   brk.StartTime,
	}

	if err := p.publisher.Publish(ctx, messaging.EventTimeBreakStart, data); err != nil {
		p.logger.Error().Err(err).Str("time_break_id", brk.ID).Msg("failed to publish time break start event")
	}
}

// PublishTimeBreakEnd publishes a time break end event
func (p *StaffEventPublisher) PublishTimeBreakEnd(ctx context.Context, brk *repository.TimeBreak, employeeID string) {
	if brk.EndTime == nil {
		p.logger.Warn().Str("time_break_id", brk.ID).Msg("attempted to publish break end event with nil end_time")
		return
	}

	durationMins := int(brk.EndTime.Sub(brk.StartTime).Minutes())

	data := messaging.TimeBreakEndEvent{
		TimeBreakID:  brk.ID,
		TimeEntryID:  brk.TimeEntryID,
		EmployeeID:   employeeID,
		StartTime:    brk.StartTime,
		EndTime:      *brk.EndTime,
		DurationMins: durationMins,
	}

	if err := p.publisher.Publish(ctx, messaging.EventTimeBreakEnd, data); err != nil {
		p.logger.Error().Err(err).Str("time_break_id", brk.ID).Msg("failed to publish time break end event")
	}
}
