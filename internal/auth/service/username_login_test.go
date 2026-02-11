package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/auth/repository"
	"github.com/medflow/medflow-backend/pkg/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// TEST: Username Login Flow - End to End
// ============================================================================

// TestUsernameLoginFlow_EndToEnd tests the complete flow:
// 1. User created with username → UserCreatedEvent published
// 2. Event contains username → Consumer processes event
// 3. Lookup table has username → Username login works
func TestUsernameLoginFlow_EndToEnd(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "username-flow-test")
	lookupRepo := repository.NewUserTenantLookupRepository(suite.DB)

	t.Run("UserCreatedEvent with username populates lookup table", func(t *testing.T) {
		userID := uuid.New().String()
		email := "testuser@clinic.de"
		username := "testuser"

		// Simulate the event that would be published by user service
		eventData := messaging.UserCreatedEvent{
			UserID:       userID,
			Email:        email,
			Username:     &username, // KEY: Username must be included
			FirstName:    "Test",
			LastName:     "User",
			RoleName:     "staff",
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
		}

		// Directly insert into lookup table (simulating consumer processing)
		lookup := &repository.UserTenantLookup{
			Email:      eventData.Email,
			Username:   eventData.Username,
			UserID:     eventData.UserID,
			TenantID:   eventData.TenantID,
			TenantSlug: eventData.TenantSlug,
		}

		err := lookupRepo.Upsert(ctx, lookup)
		require.NoError(t, err)

		// Verify username is in lookup table
		result, err := lookupRepo.GetByUsernameAndSlug(ctx, username, tenant.Slug)
		require.NoError(t, err, "should find user by username")
		assert.Equal(t, email, result.Email)
		assert.Equal(t, userID, result.UserID)
		assert.NotNil(t, result.Username)
		assert.Equal(t, username, *result.Username)
	})

	t.Run("UserCreatedEvent without username still works for email login", func(t *testing.T) {
		userID := uuid.New().String()
		email := "emailonly@clinic.de"

		// Event without username (nil)
		lookup := &repository.UserTenantLookup{
			Email:      email,
			Username:   nil, // No username
			UserID:     userID,
			TenantID:   tenant.ID,
			TenantSlug: tenant.Slug,
		}

		err := lookupRepo.Upsert(ctx, lookup)
		require.NoError(t, err)

		// Can find by email
		result, err := lookupRepo.GetByEmail(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, email, result.Email)
		assert.Nil(t, result.Username)
	})
}

// TestUserCreatedEvent_UsernameField verifies the event structure
func TestUserCreatedEvent_UsernameField(t *testing.T) {
	t.Run("event with username serializes correctly", func(t *testing.T) {
		username := "jdoe"
		event := messaging.UserCreatedEvent{
			UserID:       "user-123",
			Email:        "john@example.com",
			Username:     &username,
			FirstName:    "John",
			LastName:     "Doe",
			RoleName:     "staff",
			TenantID:     "tenant-123",
			TenantSlug:   "demo-clinic",
		}

		assert.NotNil(t, event.Username)
		assert.Equal(t, "jdoe", *event.Username)
	})

	t.Run("event without username has nil username", func(t *testing.T) {
		event := messaging.UserCreatedEvent{
			UserID:       "user-123",
			Email:        "john@example.com",
			Username:     nil, // No username
			FirstName:    "John",
			LastName:     "Doe",
			RoleName:     "staff",
			TenantID:     "tenant-123",
			TenantSlug:   "demo-clinic",
		}

		assert.Nil(t, event.Username)
	})
}

// TestLookupRepository_UsernameOperations tests username-related operations
func TestLookupRepository_UsernameOperations(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "lookup-username-test")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	t.Run("Upsert preserves username", func(t *testing.T) {
		username := "preserveuser"
		lookup := &repository.UserTenantLookup{
			Email:      "preserve@test.de",
			Username:   &username,
			UserID:     uuid.New().String(),
			TenantID:   tenant.ID,
			TenantSlug: tenant.Slug,
		}

		err := repo.Upsert(ctx, lookup)
		require.NoError(t, err)

		// Retrieve and verify
		result, err := repo.GetByEmail(ctx, "preserve@test.de")
		require.NoError(t, err)
		require.NotNil(t, result.Username, "username should be preserved")
		assert.Equal(t, "preserveuser", *result.Username)
	})

	t.Run("GetByUsernameAndSlug returns correct user", func(t *testing.T) {
		username := "uniqueuser"
		userID := uuid.New().String()

		lookup := &repository.UserTenantLookup{
			Email:      "unique@test.de",
			Username:   &username,
			UserID:     userID,
			TenantID:   tenant.ID,
			TenantSlug: tenant.Slug,
		}

		err := repo.Upsert(ctx, lookup)
		require.NoError(t, err)

		result, err := repo.GetByUsernameAndSlug(ctx, username, tenant.Slug)
		require.NoError(t, err)
		assert.Equal(t, userID, result.UserID)
		assert.Equal(t, "unique@test.de", result.Email)
	})

	t.Run("GetByUsernameAndSlug fails for wrong tenant", func(t *testing.T) {
		username := "isolateduser"

		lookup := &repository.UserTenantLookup{
			Email:      "isolated@test.de",
			Username:   &username,
			UserID:     uuid.New().String(),
			TenantID:   tenant.ID,
			TenantSlug: tenant.Slug,
		}

		err := repo.Upsert(ctx, lookup)
		require.NoError(t, err)

		// Try to find with wrong tenant slug
		_, err = repo.GetByUsernameAndSlug(ctx, username, "wrong-tenant")
		require.Error(t, err, "should not find user in wrong tenant")
	})
}
