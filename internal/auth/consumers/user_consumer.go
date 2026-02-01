package consumers

import (
	"context"
	"fmt"

	"github.com/medflow/medflow-backend/internal/auth/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/messaging"
)

// UserEventHandler handles user events for the lookup table (testable without RabbitMQ)
type UserEventHandler struct {
	lookupRepo *repository.UserTenantLookupRepository
	logger     *logger.Logger
}

// NewUserEventHandler creates a new handler for testing purposes
func NewUserEventHandler(lookupRepo *repository.UserTenantLookupRepository, log *logger.Logger) *UserEventHandler {
	return &UserEventHandler{
		lookupRepo: lookupRepo,
		logger:     log,
	}
}

// HandleEvent processes a user event and updates the lookup table
func (h *UserEventHandler) HandleEvent(ctx context.Context, event *messaging.Event) error {
	switch event.Type {
	case messaging.EventUserCreated:
		return h.handleUserCreated(ctx, event)
	case messaging.EventUserUpdated:
		return h.handleUserUpdated(ctx, event)
	case messaging.EventUserDeleted:
		return h.handleUserDeleted(ctx, event)
	default:
		h.logger.Warn().Str("event_type", event.Type).Msg("unknown event type received")
		return nil
	}
}

// UserEventConsumer consumes user events to sync the user-tenant lookup table
type UserEventConsumer struct {
	consumer   *messaging.Consumer
	handler    *UserEventHandler
	lookupRepo *repository.UserTenantLookupRepository
	logger     *logger.Logger
}

// NewUserEventConsumer creates a new user event consumer for auth service
func NewUserEventConsumer(rmq *messaging.RabbitMQ, lookupRepo *repository.UserTenantLookupRepository, log *logger.Logger) (*UserEventConsumer, error) {
	consumer, err := messaging.NewConsumer(rmq, "auth-service.user-events", log)
	if err != nil {
		return nil, err
	}

	// Subscribe to user events with pattern user.#
	if err := consumer.Subscribe(messaging.ExchangeUserEvents, "user.#"); err != nil {
		return nil, err
	}

	handler := NewUserEventHandler(lookupRepo, log)

	c := &UserEventConsumer{
		consumer:   consumer,
		handler:    handler,
		lookupRepo: lookupRepo,
		logger:     log,
	}

	// Register handlers for lookup table sync
	consumer.RegisterHandler(messaging.EventUserCreated, handler.handleUserCreated)
	consumer.RegisterHandler(messaging.EventUserUpdated, handler.handleUserUpdated)
	consumer.RegisterHandler(messaging.EventUserDeleted, handler.handleUserDeleted)

	return c, nil
}

// Start starts consuming messages
func (c *UserEventConsumer) Start(ctx context.Context) error {
	return c.consumer.Start(ctx)
}

// handleUserCreated adds a new entry to the user-tenant lookup table
func (h *UserEventHandler) handleUserCreated(ctx context.Context, event *messaging.Event) error {
	var data messaging.UserCreatedEvent
	if err := event.UnmarshalData(&data); err != nil {
		h.logger.Error().Err(err).Msg("failed to unmarshal UserCreatedEvent")
		return err
	}

	// Validate required tenant fields
	if data.TenantID == "" || data.TenantSchema == "" {
		h.logger.Warn().
			Str("user_id", data.UserID).
			Str("email", data.Email).
			Msg("user.created event missing tenant context, skipping lookup table update")
		return fmt.Errorf("missing tenant context in user.created event")
	}

	h.logger.Info().
		Str("user_id", data.UserID).
		Str("email", data.Email).
		Str("tenant_id", data.TenantID).
		Str("tenant_schema", data.TenantSchema).
		Msg("syncing user to lookup table")

	lookup := &repository.UserTenantLookup{
		Email:        data.Email,
		UserID:       data.UserID,
		TenantID:     data.TenantID,
		TenantSlug:   data.TenantSlug,
		TenantSchema: data.TenantSchema,
	}

	if err := h.lookupRepo.Upsert(ctx, lookup); err != nil {
		h.logger.Error().
			Err(err).
			Str("email", data.Email).
			Str("user_id", data.UserID).
			Msg("failed to upsert lookup table entry")
		return err
	}

	h.logger.Info().
		Str("email", data.Email).
		Str("user_id", data.UserID).
		Msg("user-tenant lookup entry created")

	return nil
}

// handleUserUpdated updates the lookup table when email changes
func (h *UserEventHandler) handleUserUpdated(ctx context.Context, event *messaging.Event) error {
	var data messaging.UserUpdatedEvent
	if err := event.UnmarshalData(&data); err != nil {
		h.logger.Error().Err(err).Msg("failed to unmarshal UserUpdatedEvent")
		return err
	}

	// Only process if email changed
	if data.OldEmail == nil || data.NewEmail == nil {
		h.logger.Debug().
			Str("user_id", data.UserID).
			Msg("no email change in update, skipping lookup table update")
		return nil
	}

	// Validate required tenant fields
	if data.TenantID == "" || data.TenantSchema == "" {
		h.logger.Warn().
			Str("user_id", data.UserID).
			Msg("user.updated event missing tenant context, skipping lookup table update")
		return nil
	}

	h.logger.Info().
		Str("user_id", data.UserID).
		Str("old_email", *data.OldEmail).
		Str("new_email", *data.NewEmail).
		Str("tenant_id", data.TenantID).
		Msg("updating lookup table for email change")

	// Delete old entry
	if err := h.lookupRepo.DeleteByEmail(ctx, *data.OldEmail); err != nil {
		h.logger.Warn().
			Err(err).
			Str("old_email", *data.OldEmail).
			Msg("failed to delete old email from lookup table (may not exist)")
		// Continue anyway to create new entry
	}

	// Create new entry with updated email
	lookup := &repository.UserTenantLookup{
		Email:        *data.NewEmail,
		UserID:       data.UserID,
		TenantID:     data.TenantID,
		TenantSlug:   data.TenantSlug,
		TenantSchema: data.TenantSchema,
	}

	if err := h.lookupRepo.Upsert(ctx, lookup); err != nil {
		h.logger.Error().
			Err(err).
			Str("new_email", *data.NewEmail).
			Str("user_id", data.UserID).
			Msg("failed to upsert new email to lookup table")
		return err
	}

	h.logger.Info().
		Str("old_email", *data.OldEmail).
		Str("new_email", *data.NewEmail).
		Str("user_id", data.UserID).
		Msg("user-tenant lookup entry updated for email change")

	return nil
}

// handleUserDeleted removes the entry from the user-tenant lookup table
func (h *UserEventHandler) handleUserDeleted(ctx context.Context, event *messaging.Event) error {
	var data messaging.UserDeletedEvent
	if err := event.UnmarshalData(&data); err != nil {
		h.logger.Error().Err(err).Msg("failed to unmarshal UserDeletedEvent")
		return err
	}

	h.logger.Info().
		Str("user_id", data.UserID).
		Str("email", data.Email).
		Msg("removing user from lookup table")

	// Delete by email if provided
	if data.Email != "" {
		if err := h.lookupRepo.DeleteByEmail(ctx, data.Email); err != nil {
			h.logger.Warn().
				Err(err).
				Str("email", data.Email).
				Msg("failed to delete by email from lookup table")
		} else {
			h.logger.Info().
				Str("email", data.Email).
				Str("user_id", data.UserID).
				Msg("user-tenant lookup entry deleted")
			return nil
		}
	}

	// Fallback: delete by user ID
	if err := h.lookupRepo.DeleteByUserID(ctx, data.UserID); err != nil {
		h.logger.Error().
			Err(err).
			Str("user_id", data.UserID).
			Msg("failed to delete by user_id from lookup table")
		return err
	}

	h.logger.Info().
		Str("user_id", data.UserID).
		Msg("user-tenant lookup entry deleted by user_id")

	return nil
}
