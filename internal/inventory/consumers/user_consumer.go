package consumers

import (
	"context"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/messaging"
)

// UserEventConsumer consumes user events
type UserEventConsumer struct {
	consumer      *messaging.Consumer
	userCacheRepo *repository.UserCacheRepository
	logger        *logger.Logger
}

// NewUserEventConsumer creates a new user event consumer
func NewUserEventConsumer(rmq *messaging.RabbitMQ, userCacheRepo *repository.UserCacheRepository, log *logger.Logger) (*UserEventConsumer, error) {
	consumer, err := messaging.NewConsumer(rmq, "inventory-service.user-events", log)
	if err != nil {
		return nil, err
	}

	// Subscribe to user events
	if err := consumer.Subscribe(messaging.ExchangeUserEvents, "user.#"); err != nil {
		return nil, err
	}

	c := &UserEventConsumer{
		consumer:      consumer,
		userCacheRepo: userCacheRepo,
		logger:        log,
	}

	// Register handlers
	consumer.RegisterHandler(messaging.EventUserCreated, c.handleUserCreated)
	consumer.RegisterHandler(messaging.EventUserUpdated, c.handleUserUpdated)
	consumer.RegisterHandler(messaging.EventUserDeleted, c.handleUserDeleted)

	return c, nil
}

// Start starts consuming messages
func (c *UserEventConsumer) Start(ctx context.Context) error {
	return c.consumer.Start(ctx)
}

func (c *UserEventConsumer) handleUserCreated(ctx context.Context, event *messaging.Event) error {
	var data messaging.UserCreatedEvent
	if err := event.UnmarshalData(&data); err != nil {
		return err
	}

	c.logger.Info().
		Str("user_id", data.UserID).
		Str("name", data.Name).
		Msg("received user created event")

	return c.userCacheRepo.Set(ctx, &repository.CachedUser{
		UserID:   data.UserID,
		Name:     data.Name,
		Email:    &data.Email,
		RoleName: &data.RoleName,
	})
}

func (c *UserEventConsumer) handleUserUpdated(ctx context.Context, event *messaging.Event) error {
	var data messaging.UserUpdatedEvent
	if err := event.UnmarshalData(&data); err != nil {
		return err
	}

	c.logger.Info().
		Str("user_id", data.UserID).
		Msg("received user updated event")

	existing, _ := c.userCacheRepo.Get(ctx, data.UserID)
	if existing == nil {
		return nil
	}

	if name, ok := data.Fields["name"].(map[string]interface{}); ok {
		if newName, ok := name["to"].(string); ok {
			existing.Name = newName
		}
	}

	return c.userCacheRepo.Set(ctx, existing)
}

func (c *UserEventConsumer) handleUserDeleted(ctx context.Context, event *messaging.Event) error {
	var data messaging.UserDeletedEvent
	if err := event.UnmarshalData(&data); err != nil {
		return err
	}

	c.logger.Info().
		Str("user_id", data.UserID).
		Msg("received user deleted event")

	return c.userCacheRepo.Delete(ctx, data.UserID)
}
