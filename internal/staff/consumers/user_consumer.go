package consumers

import (
	"context"
	"strings"
	"time"

	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/internal/staff/service"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/messaging"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// UserEventConsumer consumes user events
type UserEventConsumer struct {
	consumer      *messaging.Consumer
	userCacheRepo *repository.UserCacheRepository
	employeeRepo  *repository.EmployeeRepository
	staffService  *service.StaffService
	logger        *logger.Logger
}

// NewUserEventConsumer creates a new user event consumer
func NewUserEventConsumer(
	rmq *messaging.RabbitMQ,
	userCacheRepo *repository.UserCacheRepository,
	employeeRepo *repository.EmployeeRepository,
	staffService *service.StaffService,
	log *logger.Logger,
) (*UserEventConsumer, error) {
	consumer, err := messaging.NewConsumer(rmq, "staff-service.user-events", log)
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
		employeeRepo:  employeeRepo,
		staffService:  staffService,
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
	ctx = tenant.WithTenantContext(ctx, data.TenantID, data.TenantSlug, data.TenantSchema)

	// Update cache
	if err := c.userCacheRepo.Set(ctx, &repository.CachedUser{
		UserID:    data.UserID,
		FirstName: data.FirstName,
		LastName:  data.LastName,
		Email:     &data.Email,
		RoleName:  &data.RoleName,
		TenantID:  data.TenantID,
	}); err != nil {
		c.logger.Error().Err(err).Str("user_id", data.UserID).Msg("failed to update user cache")
		// Continue even if cache update fails
	}

	// Create employee record automatically
	if err := c.createEmployeeForUser(ctx, &data); err != nil {
		c.logger.Error().Err(err).Str("user_id", data.UserID).Msg("failed to create employee record")
		// Don't fail the event - employee can be created manually later
		// This ensures user creation isn't blocked
	}

	return nil
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
	ctx = tenant.WithTenantContext(ctx, data.TenantID, data.TenantSlug, data.TenantSchema)

	// Get existing cache entry
	existing, _ := c.userCacheRepo.Get(ctx, data.UserID)
	if existing == nil {
		return nil // User not in cache, ignore
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
	ctx = tenant.WithTenantContext(ctx, data.TenantID, data.TenantSlug, data.TenantSchema)

	// Remove from cache
	return c.userCacheRepo.Delete(ctx, data.UserID)
}

// createEmployeeForUser creates an employee record for a newly created user
func (c *UserEventConsumer) createEmployeeForUser(ctx context.Context, userData *messaging.UserCreatedEvent) error {
	// Check if employee already exists for this user (idempotency)
	existing, _ := c.employeeRepo.GetByUserID(ctx, userData.UserID)
	if existing != nil {
		c.logger.Info().
			Str("user_id", userData.UserID).
			Msg("employee already exists, skipping creation")
		return nil // Idempotent - don't fail if already exists
	}

	// Map user data to employee
	jobTitle := mapRoleToJobTitle(userData.RoleName)
	hireDate := time.Now() // User creation date as hire date

	employee := &repository.Employee{
		UserID:         &userData.UserID,
		FirstName:      userData.FirstName,
		LastName:       userData.LastName,
		Email:          &userData.Email,
		EmploymentType: "full_time",
		HireDate:       hireDate,
		Status:         "active",
		JobTitle:       &jobTitle,
		// Department and EmployeeNumber remain null (set manually later)
	}

	c.logger.Info().
		Str("user_id", userData.UserID).
		Str("job_title", jobTitle).
		Msg("creating employee record for user")

	return c.staffService.Create(ctx, employee)
}

// mapRoleToJobTitle maps user role names to employee job titles
func mapRoleToJobTitle(roleName string) string {
	switch roleName {
	case "admin":
		return "Administrator"
	case "manager":
		return "Manager"
	case "staff":
		return "Staff Member"
	case "viewer":
		return "Viewer"
	default:
		// Capitalize the role name for unknown roles
		return strings.Title(roleName)
	}
}
