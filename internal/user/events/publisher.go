package events

import (
	"context"

	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/messaging"
)

// UserEventPublisher publishes user-related events
type UserEventPublisher struct {
	publisher *messaging.Publisher
	logger    *logger.Logger
}

// NewUserEventPublisher creates a new user event publisher
func NewUserEventPublisher(rmq *messaging.RabbitMQ, log *logger.Logger) (*UserEventPublisher, error) {
	publisher, err := messaging.NewPublisher(rmq, messaging.ExchangeUserEvents, "user-service", log)
	if err != nil {
		return nil, err
	}

	return &UserEventPublisher{
		publisher: publisher,
		logger:    log,
	}, nil
}

// PublishUserCreated publishes a user created event
func (p *UserEventPublisher) PublishUserCreated(ctx context.Context, user *domain.User) {
	data := messaging.UserCreatedEvent{
		UserID:   user.ID,
		Email:    user.Email,
		Name:     user.Name,
		RoleName: user.Role.Name,
	}

	if err := p.publisher.Publish(ctx, messaging.EventUserCreated, data); err != nil {
		p.logger.Error().Err(err).Str("user_id", user.ID).Msg("failed to publish user created event")
	}
}

// PublishUserUpdated publishes a user updated event
func (p *UserEventPublisher) PublishUserUpdated(ctx context.Context, user *domain.User, changes map[string]interface{}) {
	data := messaging.UserUpdatedEvent{
		UserID: user.ID,
		Fields: changes,
	}

	if err := p.publisher.Publish(ctx, messaging.EventUserUpdated, data); err != nil {
		p.logger.Error().Err(err).Str("user_id", user.ID).Msg("failed to publish user updated event")
	}
}

// PublishUserDeleted publishes a user deleted event
func (p *UserEventPublisher) PublishUserDeleted(ctx context.Context, userID string) {
	data := messaging.UserDeletedEvent{
		UserID: userID,
	}

	if err := p.publisher.Publish(ctx, messaging.EventUserDeleted, data); err != nil {
		p.logger.Error().Err(err).Str("user_id", userID).Msg("failed to publish user deleted event")
	}
}

// PublishUserRoleChanged publishes a user role changed event
func (p *UserEventPublisher) PublishUserRoleChanged(ctx context.Context, userID, oldRole, newRole string) {
	data := messaging.UserRoleChangedEvent{
		UserID:      userID,
		OldRoleName: oldRole,
		NewRoleName: newRole,
	}

	if err := p.publisher.Publish(ctx, messaging.EventUserRoleChanged, data); err != nil {
		p.logger.Error().Err(err).Str("user_id", userID).Msg("failed to publish user role changed event")
	}
}
