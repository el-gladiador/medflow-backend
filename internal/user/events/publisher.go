package events

import (
	"context"

	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/messaging"
	"github.com/medflow/medflow-backend/pkg/tenant"
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
	// Extract tenant context for user-tenant lookup table sync
	tenantID, _ := tenant.TenantID(ctx)
	tenantSlug, _ := tenant.TenantSlug(ctx)
	tenantSchema, _ := tenant.TenantSchema(ctx)

	roleName := ""
	if user.Role != nil {
		roleName = user.Role.Name
	}

	data := messaging.UserCreatedEvent{
		UserID:       user.ID,
		Email:        user.Email,
		Username:     user.Username,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		RoleName:     roleName,
		TenantID:     tenantID,
		TenantSlug:   tenantSlug,
		TenantSchema: tenantSchema,
	}

	if err := p.publisher.Publish(ctx, messaging.EventUserCreated, data); err != nil {
		p.logger.Error().Err(err).Str("user_id", user.ID).Msg("failed to publish user created event")
	}
}

// PublishUserUpdated publishes a user updated event
// If oldEmail is provided (non-empty), it indicates an email change
func (p *UserEventPublisher) PublishUserUpdated(ctx context.Context, user *domain.User, changes map[string]interface{}, oldEmail string) {
	// Extract tenant context for user-tenant lookup table sync
	tenantID, _ := tenant.TenantID(ctx)
	tenantSlug, _ := tenant.TenantSlug(ctx)
	tenantSchema, _ := tenant.TenantSchema(ctx)

	data := messaging.UserUpdatedEvent{
		UserID:       user.ID,
		Fields:       changes,
		TenantID:     tenantID,
		TenantSlug:   tenantSlug,
		TenantSchema: tenantSchema,
	}

	// Track email changes for lookup table updates
	if oldEmail != "" && oldEmail != user.Email {
		data.OldEmail = &oldEmail
		data.NewEmail = &user.Email
	}

	if err := p.publisher.Publish(ctx, messaging.EventUserUpdated, data); err != nil {
		p.logger.Error().Err(err).Str("user_id", user.ID).Msg("failed to publish user updated event")
	}
}

// PublishUserDeleted publishes a user deleted event
// email is required for removing the user from the tenant lookup table
func (p *UserEventPublisher) PublishUserDeleted(ctx context.Context, userID, email string) {
	// Extract tenant context for user-tenant lookup table sync
	tenantID, _ := tenant.TenantID(ctx)
	tenantSlug, _ := tenant.TenantSlug(ctx)
	tenantSchema, _ := tenant.TenantSchema(ctx)

	data := messaging.UserDeletedEvent{
		UserID:       userID,
		Email:        email,
		TenantID:     tenantID,
		TenantSlug:   tenantSlug,
		TenantSchema: tenantSchema,
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
