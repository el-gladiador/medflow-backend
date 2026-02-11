package consumers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/auth/consumers"
	"github.com/medflow/medflow-backend/internal/auth/repository"
	"github.com/medflow/medflow-backend/pkg/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// TEST: UserEventHandler Username Processing
// ============================================================================

// TestUserEventHandler_UsernameInEvent verifies that username from events
// is correctly stored in the lookup table
func TestUserEventHandler_UsernameInEvent(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "consumer-username-test")
	lookupRepo := repository.NewUserTenantLookupRepository(suite.DB)
	handler := consumers.NewUserEventHandler(lookupRepo, suite.Logger)

	t.Run("user.created event WITH username stores username in lookup", func(t *testing.T) {
		userID := uuid.New().String()
		email := "withusername@test.de"
		username := "testlogin"

		eventData := messaging.UserCreatedEvent{
			UserID:       userID,
			Email:        email,
			Username:     &username, // Username provided
			FirstName:    "Test",
			LastName:     "Login",
			RoleName:     "staff",
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
		}

		payload, err := json.Marshal(eventData)
		require.NoError(t, err)

		event := &messaging.Event{
			Type:      messaging.EventUserCreated,
			Timestamp: time.Now(),
			Data:      payload,
		}

		err = handler.HandleEvent(ctx, event)
		require.NoError(t, err)

		// Verify username was stored
		lookup, err := lookupRepo.GetByEmail(ctx, email)
		require.NoError(t, err)
		require.NotNil(t, lookup.Username, "username should NOT be nil")
		assert.Equal(t, username, *lookup.Username)

		// Verify can find by username + tenant
		lookupByUsername, err := lookupRepo.GetByUsernameAndSlug(ctx, username, tenant.Slug)
		require.NoError(t, err)
		assert.Equal(t, email, lookupByUsername.Email)
		assert.Equal(t, userID, lookupByUsername.UserID)
	})

	t.Run("user.created event WITHOUT username stores nil username", func(t *testing.T) {
		userID := uuid.New().String()
		email := "nousername@test.de"

		eventData := messaging.UserCreatedEvent{
			UserID:       userID,
			Email:        email,
			Username:     nil, // No username
			FirstName:    "No",
			LastName:     "Username",
			RoleName:     "staff",
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
		}

		payload, err := json.Marshal(eventData)
		require.NoError(t, err)

		event := &messaging.Event{
			Type:      messaging.EventUserCreated,
			Timestamp: time.Now(),
			Data:      payload,
		}

		err = handler.HandleEvent(ctx, event)
		require.NoError(t, err)

		// Verify entry created without username
		lookup, err := lookupRepo.GetByEmail(ctx, email)
		require.NoError(t, err)
		assert.Nil(t, lookup.Username, "username should be nil")

		// Cannot find by username
		_, err = lookupRepo.GetByUsernameAndSlug(ctx, "", tenant.Slug)
		require.Error(t, err, "should not find empty username")
	})

	t.Run("same username in different tenants is isolated", func(t *testing.T) {
		tenant2 := suite.SetupUserTenant(t, ctx, "consumer-username-test-2")

		commonUsername := "admin"
		user1ID := uuid.New().String()
		user2ID := uuid.New().String()

		// User in tenant 1
		event1Data := messaging.UserCreatedEvent{
			UserID:       user1ID,
			Email:        "admin@tenant1.de",
			Username:     &commonUsername,
			FirstName:    "Admin",
			LastName:     "Tenant1",
			RoleName:     "admin",
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
		}

		payload1, _ := json.Marshal(event1Data)
		event1 := &messaging.Event{
			Type:      messaging.EventUserCreated,
			Timestamp: time.Now(),
			Data:      payload1,
		}

		err := handler.HandleEvent(ctx, event1)
		require.NoError(t, err)

		// User in tenant 2 with same username
		event2Data := messaging.UserCreatedEvent{
			UserID:       user2ID,
			Email:        "admin@tenant2.de",
			Username:     &commonUsername,
			FirstName:    "Admin",
			LastName:     "Tenant2",
			RoleName:     "admin",
			TenantID:     tenant2.ID,
			TenantSlug:   tenant2.Slug,
		}

		payload2, _ := json.Marshal(event2Data)
		event2 := &messaging.Event{
			Type:      messaging.EventUserCreated,
			Timestamp: time.Now(),
			Data:      payload2,
		}

		err = handler.HandleEvent(ctx, event2)
		require.NoError(t, err)

		// Verify tenant isolation
		lookup1, err := lookupRepo.GetByUsernameAndSlug(ctx, commonUsername, tenant.Slug)
		require.NoError(t, err)
		assert.Equal(t, user1ID, lookup1.UserID)
		assert.Equal(t, "admin@tenant1.de", lookup1.Email)

		lookup2, err := lookupRepo.GetByUsernameAndSlug(ctx, commonUsername, tenant2.Slug)
		require.NoError(t, err)
		assert.Equal(t, user2ID, lookup2.UserID)
		assert.Equal(t, "admin@tenant2.de", lookup2.Email)

		// They are different users
		assert.NotEqual(t, lookup1.UserID, lookup2.UserID)
	})
}
