package consumers

import (
	"context"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/messaging"
	"github.com/medflow/medflow-backend/pkg/tenant"
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
		Str("name", data.FullName()).
		Msg("received user created event")

	// Create tenant context from event data
	ctx = tenant.WithTenantContext(ctx, data.TenantID, data.TenantSlug)

	return c.userCacheRepo.Set(ctx, &repository.CachedUser{
		UserID:    data.UserID,
		FirstName: data.FirstName,
		LastName:  data.LastName,
		Email:     &data.Email,
		RoleName:  &data.RoleName,
		TenantID:  data.TenantID,
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

	// Create tenant context from event data
	ctx = tenant.WithTenantContext(ctx, data.TenantID, data.TenantSlug)

	existing, _ := c.userCacheRepo.Get(ctx, data.UserID)
	if existing == nil {
		return nil
	}

	// Update fields that changed
	if firstName, ok := data.Fields["first_name"].(map[string]interface{}); ok {
		if newName, ok := firstName["to"].(string); ok {
			existing.FirstName = newName
		}
	}
	if lastName, ok := data.Fields["last_name"].(map[string]interface{}); ok {
		if newName, ok := lastName["to"].(string); ok {
			existing.LastName = newName
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

	// Create tenant context from event data
	ctx = tenant.WithTenantContext(ctx, data.TenantID, data.TenantSlug)

	return c.userCacheRepo.Delete(ctx, data.UserID)
}
