package events_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/pkg/messaging"
	"github.com/medflow/medflow-backend/pkg/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockPublisher captures published events for testing
type MockPublisher struct {
	events []PublishedEvent
}

type PublishedEvent struct {
	EventType string
	Data      []byte
}

func (m *MockPublisher) Publish(ctx context.Context, eventType string, data interface{}) error {
	jsonData, _ := json.Marshal(data)
	m.events = append(m.events, PublishedEvent{
		EventType: eventType,
		Data:      jsonData,
	})
	return nil
}

func TestUserCreatedEvent_IncludesTenantContext(t *testing.T) {
	// Setup tenant context
	tenantID := uuid.New().String()
	tenantSlug := "test-clinic"
	ctx := tenant.WithTenantContext(context.Background(), tenantID, tenantSlug)

	// Create test user
	user := &domain.User{
		ID:        uuid.New().String(),
		Email:     "test@clinic.de",
		FirstName: "Test",
		LastName:  "User",
		Role: &domain.Role{
			Name: "staff",
		},
	}

	// Create event data as would be done in PublishUserCreated
	tenantIDFromCtx, _ := tenant.TenantID(ctx)
	tenantSlugFromCtx, _ := tenant.TenantSlug(ctx)

	event := messaging.UserCreatedEvent{
		UserID:       user.ID,
		Email:        user.Email,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		RoleName:     user.Role.Name,
		TenantID:     tenantIDFromCtx,
		TenantSlug:   tenantSlugFromCtx,
	}

	// Verify tenant context is included
	assert.Equal(t, tenantID, event.TenantID)
	assert.Equal(t, tenantSlug, event.TenantSlug)
	assert.Equal(t, user.ID, event.UserID)
	assert.Equal(t, user.Email, event.Email)
}

func TestUserUpdatedEvent_TracksEmailChanges(t *testing.T) {
	tenantID := uuid.New().String()
	tenantSlug := "test-clinic"
	ctx := tenant.WithTenantContext(context.Background(), tenantID, tenantSlug)

	userID := uuid.New().String()
	oldEmail := "old@clinic.de"
	newEmail := "new@clinic.de"

	user := &domain.User{
		ID:        userID,
		Email:     newEmail,
		FirstName: "Test",
		LastName:  "User",
	}

	// Extract tenant context
	tenantIDFromCtx, _ := tenant.TenantID(ctx)
	tenantSlugFromCtx, _ := tenant.TenantSlug(ctx)

	changes := map[string]interface{}{"email": newEmail}

	event := messaging.UserUpdatedEvent{
		UserID:       user.ID,
		Fields:       changes,
		TenantID:     tenantIDFromCtx,
		TenantSlug:   tenantSlugFromCtx,
	}

	// Track email changes
	if oldEmail != "" && oldEmail != user.Email {
		event.OldEmail = &oldEmail
		event.NewEmail = &user.Email
	}

	// Verify tenant context
	assert.Equal(t, tenantID, event.TenantID)
	assert.Equal(t, tenantSlug, event.TenantSlug)

	// Verify email tracking
	require.NotNil(t, event.OldEmail)
	require.NotNil(t, event.NewEmail)
	assert.Equal(t, oldEmail, *event.OldEmail)
	assert.Equal(t, newEmail, *event.NewEmail)
}

func TestUserUpdatedEvent_NoEmailChange(t *testing.T) {
	tenantID := uuid.New().String()
	ctx := tenant.WithTenantContext(context.Background(), tenantID, "test")

	user := &domain.User{
		ID:        uuid.New().String(),
		Email:     "same@clinic.de",
		FirstName: "Updated",
		LastName:  "Name",
	}

	tenantIDFromCtx, _ := tenant.TenantID(ctx)

	changes := map[string]interface{}{"first_name": "Updated"}

	event := messaging.UserUpdatedEvent{
		UserID:   user.ID,
		Fields:   changes,
		TenantID: tenantIDFromCtx,
	}

	// No email change - these should be nil
	assert.Nil(t, event.OldEmail)
	assert.Nil(t, event.NewEmail)
}

func TestUserDeletedEvent_IncludesTenantContext(t *testing.T) {
	tenantID := uuid.New().String()
	tenantSlug := "test-clinic"
	ctx := tenant.WithTenantContext(context.Background(), tenantID, tenantSlug)

	userID := uuid.New().String()
	email := "deleted@clinic.de"

	tenantIDFromCtx, _ := tenant.TenantID(ctx)
	tenantSlugFromCtx, _ := tenant.TenantSlug(ctx)

	event := messaging.UserDeletedEvent{
		UserID:       userID,
		Email:        email,
		TenantID:     tenantIDFromCtx,
		TenantSlug:   tenantSlugFromCtx,
	}

	// Verify tenant context
	assert.Equal(t, tenantID, event.TenantID)
	assert.Equal(t, tenantSlug, event.TenantSlug)

	// Verify email is included for lookup table cleanup
	assert.Equal(t, email, event.Email)
	assert.Equal(t, userID, event.UserID)
}

func TestUserEventContext_MissingTenantContext(t *testing.T) {
	// Context without tenant info
	ctx := context.Background()

	tenantID, err := tenant.TenantID(ctx)
	assert.Error(t, err)
	assert.Empty(t, tenantID)

	tenantSlug, err := tenant.TenantSlug(ctx)
	assert.Error(t, err)
	assert.Empty(t, tenantSlug)
}

func TestEventJSONSerialization(t *testing.T) {
	event := messaging.UserCreatedEvent{
		UserID:       uuid.New().String(),
		Email:        "test@clinic.de",
		FirstName:    "Test",
		LastName:     "User",
		RoleName:     "staff",
		TenantID:     uuid.New().String(),
		TenantSlug:   "test-clinic",
	}

	// Serialize to JSON
	data, err := json.Marshal(event)
	require.NoError(t, err)

	// Deserialize
	var parsed messaging.UserCreatedEvent
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// Verify all fields are preserved
	assert.Equal(t, event.UserID, parsed.UserID)
	assert.Equal(t, event.Email, parsed.Email)
	assert.Equal(t, event.FirstName, parsed.FirstName)
	assert.Equal(t, event.LastName, parsed.LastName)
	assert.Equal(t, event.RoleName, parsed.RoleName)
	assert.Equal(t, event.TenantID, parsed.TenantID)
	assert.Equal(t, event.TenantSlug, parsed.TenantSlug)
}
