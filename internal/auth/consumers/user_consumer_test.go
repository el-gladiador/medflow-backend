package consumers_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/auth/consumers"
	"github.com/medflow/medflow-backend/internal/auth/repository"
	"github.com/medflow/medflow-backend/pkg/messaging"
	"github.com/medflow/medflow-backend/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var suite *testutil.IntegrationSuite

func TestMain(m *testing.M) {
	ctx := context.Background()
	var err error

	suite, err = testutil.NewIntegrationSuite(ctx)
	if err != nil {
		panic("failed to create integration suite: " + err.Error())
	}
	defer suite.Cleanup(ctx)

	// Create user_tenant_lookup table in public schema
	err = createLookupTable(ctx)
	if err != nil {
		panic("failed to create lookup table: " + err.Error())
	}

	code := m.Run()
	os.Exit(code)
}

func createLookupTable(ctx context.Context) error {
	_, err := suite.RawDB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS public.user_tenant_lookup (
			email VARCHAR(255) PRIMARY KEY,
			user_id UUID NOT NULL,
			tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
			tenant_slug VARCHAR(100) NOT NULL,
			tenant_schema VARCHAR(100) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_user_tenant_lookup_user_id ON public.user_tenant_lookup(user_id);
		CREATE INDEX IF NOT EXISTS idx_user_tenant_lookup_tenant_id ON public.user_tenant_lookup(tenant_id);
	`)
	return err
}

func cleanupLookupTable(ctx context.Context, t *testing.T) {
	_, err := suite.RawDB.ExecContext(ctx, "DELETE FROM public.user_tenant_lookup")
	require.NoError(t, err)
}

// TestUserEventHandler tests the event handling logic directly without RabbitMQ
func TestUserEventHandler_HandleUserCreated(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "consumer-test-created")
	lookupRepo := repository.NewUserTenantLookupRepository(suite.DB)
	handler := consumers.NewUserEventHandler(lookupRepo, suite.Logger)

	t.Run("creates lookup entry on user.created event", func(t *testing.T) {
		userID := uuid.New().String()
		email := "created@test.de"

		eventData := messaging.UserCreatedEvent{
			UserID:       userID,
			Email:        email,
			FirstName:    "Test",
			LastName:     "User",
			RoleName:     "staff",
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
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

		// Verify lookup entry was created
		lookup, err := lookupRepo.GetByEmail(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, email, lookup.Email)
		assert.Equal(t, userID, lookup.UserID)
		assert.Equal(t, tenant.ID, lookup.TenantID)
		assert.Equal(t, tenant.Slug, lookup.TenantSlug)
		assert.Equal(t, tenant.SchemaName, lookup.TenantSchema)
	})

	t.Run("returns error for missing tenant context", func(t *testing.T) {
		eventData := messaging.UserCreatedEvent{
			UserID:    uuid.New().String(),
			Email:     "notenant@test.de",
			FirstName: "Test",
			LastName:  "User",
			RoleName:  "staff",
			// Missing tenant fields
		}

		payload, err := json.Marshal(eventData)
		require.NoError(t, err)

		event := &messaging.Event{
			Type:      messaging.EventUserCreated,
			Timestamp: time.Now(),
			Data:      payload,
		}

		err = handler.HandleEvent(ctx, event)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tenant")
	})
}

func TestUserEventHandler_HandleUserUpdated(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "consumer-test-updated")
	lookupRepo := repository.NewUserTenantLookupRepository(suite.DB)
	handler := consumers.NewUserEventHandler(lookupRepo, suite.Logger)

	t.Run("updates lookup when email changes", func(t *testing.T) {
		userID := uuid.New().String()
		oldEmail := "old@test.de"
		newEmail := "new@test.de"

		// Create initial lookup entry
		err := lookupRepo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        oldEmail,
			UserID:       userID,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		})
		require.NoError(t, err)

		// Send update event with email change
		eventData := messaging.UserUpdatedEvent{
			UserID:       userID,
			Fields:       map[string]any{"email": newEmail},
			OldEmail:     &oldEmail,
			NewEmail:     &newEmail,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		}

		payload, err := json.Marshal(eventData)
		require.NoError(t, err)

		event := &messaging.Event{
			Type:      messaging.EventUserUpdated,
			Timestamp: time.Now(),
			Data:      payload,
		}

		err = handler.HandleEvent(ctx, event)
		require.NoError(t, err)

		// Verify old email is gone
		_, err = lookupRepo.GetByEmail(ctx, oldEmail)
		require.Error(t, err)

		// Verify new email exists
		lookup, err := lookupRepo.GetByEmail(ctx, newEmail)
		require.NoError(t, err)
		assert.Equal(t, newEmail, lookup.Email)
		assert.Equal(t, userID, lookup.UserID)
	})

	t.Run("ignores update without email change", func(t *testing.T) {
		userID := uuid.New().String()
		email := "nochange@test.de"

		// Create initial lookup entry
		err := lookupRepo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        email,
			UserID:       userID,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		})
		require.NoError(t, err)

		// Send update event without email change
		eventData := messaging.UserUpdatedEvent{
			UserID:       userID,
			Fields:       map[string]any{"first_name": "Updated"},
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		}

		payload, err := json.Marshal(eventData)
		require.NoError(t, err)

		event := &messaging.Event{
			Type:      messaging.EventUserUpdated,
			Timestamp: time.Now(),
			Data:      payload,
		}

		err = handler.HandleEvent(ctx, event)
		require.NoError(t, err)

		// Verify entry still exists unchanged
		lookup, err := lookupRepo.GetByEmail(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, email, lookup.Email)
	})
}

func TestUserEventHandler_HandleUserDeleted(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "consumer-test-deleted")
	lookupRepo := repository.NewUserTenantLookupRepository(suite.DB)
	handler := consumers.NewUserEventHandler(lookupRepo, suite.Logger)

	t.Run("deletes lookup entry on user.deleted event", func(t *testing.T) {
		userID := uuid.New().String()
		email := "deleted@test.de"

		// Create lookup entry
		err := lookupRepo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        email,
			UserID:       userID,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		})
		require.NoError(t, err)

		// Send delete event
		eventData := messaging.UserDeletedEvent{
			UserID:       userID,
			Email:        email,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		}

		payload, err := json.Marshal(eventData)
		require.NoError(t, err)

		event := &messaging.Event{
			Type:      messaging.EventUserDeleted,
			Timestamp: time.Now(),
			Data:      payload,
		}

		err = handler.HandleEvent(ctx, event)
		require.NoError(t, err)

		// Verify entry was deleted
		_, err = lookupRepo.GetByEmail(ctx, email)
		require.Error(t, err)
	})

	t.Run("falls back to user_id deletion if email not provided", func(t *testing.T) {
		userID := uuid.New().String()
		email := "fallback@test.de"

		// Create lookup entry
		err := lookupRepo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        email,
			UserID:       userID,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		})
		require.NoError(t, err)

		// Send delete event without email
		eventData := messaging.UserDeletedEvent{
			UserID:       userID,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		}

		payload, err := json.Marshal(eventData)
		require.NoError(t, err)

		event := &messaging.Event{
			Type:      messaging.EventUserDeleted,
			Timestamp: time.Now(),
			Data:      payload,
		}

		err = handler.HandleEvent(ctx, event)
		require.NoError(t, err)

		// Verify entry was deleted
		results, err := lookupRepo.GetByUserID(ctx, userID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestUserEventHandler_UnknownEventType(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	lookupRepo := repository.NewUserTenantLookupRepository(suite.DB)
	handler := consumers.NewUserEventHandler(lookupRepo, suite.Logger)

	t.Run("logs warning for unknown event type", func(t *testing.T) {
		event := &messaging.Event{
			Type:      "unknown.event",
			Timestamp: time.Now(),
			Data:      []byte(`{}`),
		}

		// Should not error, just log
		err := handler.HandleEvent(ctx, event)
		require.NoError(t, err)
	})
}

func TestUserEventHandler_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	lookupRepo := repository.NewUserTenantLookupRepository(suite.DB)
	handler := consumers.NewUserEventHandler(lookupRepo, suite.Logger)

	t.Run("returns error for invalid JSON payload", func(t *testing.T) {
		event := &messaging.Event{
			Type:      messaging.EventUserCreated,
			Timestamp: time.Now(),
			Data:      []byte(`{invalid json`),
		}

		err := handler.HandleEvent(ctx, event)
		require.Error(t, err)
	})
}
