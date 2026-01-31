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
