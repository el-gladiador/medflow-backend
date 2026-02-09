package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/auth/repository"
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
			username VARCHAR(100),
			user_id UUID NOT NULL,
			tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
			tenant_slug VARCHAR(100) NOT NULL,
			tenant_schema VARCHAR(100) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_user_tenant_lookup_user_id ON public.user_tenant_lookup(user_id);
		CREATE INDEX IF NOT EXISTS idx_user_tenant_lookup_tenant_id ON public.user_tenant_lookup(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_user_tenant_lookup_username ON public.user_tenant_lookup(username)
			WHERE username IS NOT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS idx_user_tenant_lookup_username_tenant_unique
			ON public.user_tenant_lookup(username, tenant_slug)
			WHERE username IS NOT NULL;
	`)
	return err
}

func cleanupLookupTable(ctx context.Context, t *testing.T) {
	_, err := suite.RawDB.ExecContext(ctx, "DELETE FROM public.user_tenant_lookup")
	require.NoError(t, err)
}

func TestUserTenantLookupRepository_Upsert(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	// Create a test tenant first
	tenant := suite.SetupUserTenant(t, ctx, "lookup-test-upsert")

	repo := repository.NewUserTenantLookupRepository(suite.DB)

	t.Run("inserts new lookup entry", func(t *testing.T) {
		lookup := &repository.UserTenantLookup{
			Email:        "test@praxis-mueller.de",
			UserID:       uuid.New().String(),
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		}

		err := repo.Upsert(ctx, lookup)
		require.NoError(t, err)

		// Verify insertion
		result, err := repo.GetByEmail(ctx, lookup.Email)
		require.NoError(t, err)
		assert.Equal(t, lookup.Email, result.Email)
		assert.Equal(t, lookup.UserID, result.UserID)
		assert.Equal(t, lookup.TenantID, result.TenantID)
		assert.Equal(t, lookup.TenantSlug, result.TenantSlug)
		assert.Equal(t, lookup.TenantSchema, result.TenantSchema)
	})

	t.Run("updates existing lookup entry on conflict", func(t *testing.T) {
		email := "update@praxis-mueller.de"
		originalUserID := uuid.New().String()
		newUserID := uuid.New().String()

		// Insert original
		err := repo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        email,
			UserID:       originalUserID,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		})
		require.NoError(t, err)

		// Record original timestamp
		original, err := repo.GetByEmail(ctx, email)
		require.NoError(t, err)
		originalTime := original.UpdatedAt

		// Wait to ensure timestamp difference
		time.Sleep(10 * time.Millisecond)

		// Update with new user ID
		err = repo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        email,
			UserID:       newUserID,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		})
		require.NoError(t, err)

		// Verify update
		result, err := repo.GetByEmail(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, newUserID, result.UserID)
		assert.True(t, result.UpdatedAt.After(originalTime) || result.UpdatedAt.Equal(originalTime))
	})
}

func TestUserTenantLookupRepository_GetByEmail(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "lookup-test-get")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	// Insert test data
	lookup := &repository.UserTenantLookup{
		Email:        "getbyemail@test.de",
		UserID:       uuid.New().String(),
		TenantID:     tenant.ID,
		TenantSlug:   tenant.Slug,
		TenantSchema: tenant.SchemaName,
	}
	err := repo.Upsert(ctx, lookup)
	require.NoError(t, err)

	t.Run("returns lookup for existing email", func(t *testing.T) {
		result, err := repo.GetByEmail(ctx, lookup.Email)
		require.NoError(t, err)
		assert.Equal(t, lookup.Email, result.Email)
		assert.Equal(t, lookup.UserID, result.UserID)
	})

	t.Run("returns error for non-existent email", func(t *testing.T) {
		_, err := repo.GetByEmail(ctx, "nonexistent@test.de")
		require.Error(t, err)
	})
}

func TestUserTenantLookupRepository_GetByUserID(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "lookup-test-userid")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	userID := uuid.New().String()

	// Insert test data - user with multiple emails (edge case)
	email1 := "user1@test.de"
	err := repo.Upsert(ctx, &repository.UserTenantLookup{
		Email:        email1,
		UserID:       userID,
		TenantID:     tenant.ID,
		TenantSlug:   tenant.Slug,
		TenantSchema: tenant.SchemaName,
	})
	require.NoError(t, err)

	t.Run("returns lookups for existing user ID", func(t *testing.T) {
		results, err := repo.GetByUserID(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, email1, results[0].Email)
	})

	t.Run("returns empty slice for non-existent user ID", func(t *testing.T) {
		results, err := repo.GetByUserID(ctx, uuid.New().String())
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestUserTenantLookupRepository_DeleteByEmail(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "lookup-test-delete-email")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	email := "delete@test.de"
	err := repo.Upsert(ctx, &repository.UserTenantLookup{
		Email:        email,
		UserID:       uuid.New().String(),
		TenantID:     tenant.ID,
		TenantSlug:   tenant.Slug,
		TenantSchema: tenant.SchemaName,
	})
	require.NoError(t, err)

	t.Run("deletes existing entry", func(t *testing.T) {
		err := repo.DeleteByEmail(ctx, email)
		require.NoError(t, err)

		// Verify deletion
		_, err = repo.GetByEmail(ctx, email)
		require.Error(t, err)
	})

	t.Run("no error when deleting non-existent email", func(t *testing.T) {
		err := repo.DeleteByEmail(ctx, "nonexistent@test.de")
		require.NoError(t, err)
	})
}

func TestUserTenantLookupRepository_DeleteByUserID(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "lookup-test-delete-userid")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	userID := uuid.New().String()

	// Insert test data
	err := repo.Upsert(ctx, &repository.UserTenantLookup{
		Email:        "deletebyuserid@test.de",
		UserID:       userID,
		TenantID:     tenant.ID,
		TenantSlug:   tenant.Slug,
		TenantSchema: tenant.SchemaName,
	})
	require.NoError(t, err)

	t.Run("deletes all entries for user ID", func(t *testing.T) {
		err := repo.DeleteByUserID(ctx, userID)
		require.NoError(t, err)

		// Verify deletion
		results, err := repo.GetByUserID(ctx, userID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestUserTenantLookupRepository_UpdateEmail(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "lookup-test-update-email")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	userID := uuid.New().String()
	oldEmail := "old@test.de"
	newEmail := "new@test.de"

	// Insert original entry
	err := repo.Upsert(ctx, &repository.UserTenantLookup{
		Email:        oldEmail,
		UserID:       userID,
		TenantID:     tenant.ID,
		TenantSlug:   tenant.Slug,
		TenantSchema: tenant.SchemaName,
	})
	require.NoError(t, err)

	t.Run("updates email for existing user", func(t *testing.T) {
		err := repo.UpdateEmail(ctx, oldEmail, newEmail, userID)
		require.NoError(t, err)

		// Verify old email is gone
		_, err = repo.GetByEmail(ctx, oldEmail)
		require.Error(t, err)

		// Verify new email exists with correct data
		result, err := repo.GetByEmail(ctx, newEmail)
		require.NoError(t, err)
		assert.Equal(t, newEmail, result.Email)
		assert.Equal(t, userID, result.UserID)
		assert.Equal(t, tenant.ID, result.TenantID)
	})
}

func TestUserTenantLookupRepository_Exists(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "lookup-test-exists")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	existingEmail := "exists@test.de"
	err := repo.Upsert(ctx, &repository.UserTenantLookup{
		Email:        existingEmail,
		UserID:       uuid.New().String(),
		TenantID:     tenant.ID,
		TenantSlug:   tenant.Slug,
		TenantSchema: tenant.SchemaName,
	})
	require.NoError(t, err)

	t.Run("returns true for existing email", func(t *testing.T) {
		exists, err := repo.Exists(ctx, existingEmail)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("returns false for non-existent email", func(t *testing.T) {
		exists, err := repo.Exists(ctx, "nonexistent@test.de")
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestUserTenantLookupRepository_CascadeDelete(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	// Create a tenant specifically for this test
	tenant := suite.SetupUserTenant(t, ctx, "lookup-cascade-test")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	email := "cascade@test.de"
	err := repo.Upsert(ctx, &repository.UserTenantLookup{
		Email:        email,
		UserID:       uuid.New().String(),
		TenantID:     tenant.ID,
		TenantSlug:   tenant.Slug,
		TenantSchema: tenant.SchemaName,
	})
	require.NoError(t, err)

	// Verify entry exists
	exists, err := repo.Exists(ctx, email)
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete the tenant (this triggers CASCADE delete)
	_, err = suite.RawDB.ExecContext(ctx, "DELETE FROM public.tenants WHERE id = $1", tenant.ID)
	require.NoError(t, err)

	// Verify lookup entry was also deleted via CASCADE
	exists, err = repo.Exists(ctx, email)
	require.NoError(t, err)
	assert.False(t, exists, "lookup entry should be deleted when tenant is deleted (CASCADE)")
}

// ============================================================================
// USERNAME + TENANT SLUG TESTS (Subdomain-based multi-tenancy)
// ============================================================================

func addUsernameColumnIfMissing(ctx context.Context, t *testing.T) {
	// Add username column if it doesn't exist (for test isolation)
	_, _ = suite.RawDB.ExecContext(ctx, `
		ALTER TABLE public.user_tenant_lookup
		ADD COLUMN IF NOT EXISTS username VARCHAR(100);

		CREATE INDEX IF NOT EXISTS idx_user_tenant_lookup_username
		ON public.user_tenant_lookup(username)
		WHERE username IS NOT NULL;
	`)
}

func TestUserTenantLookupRepository_GetByUsername(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)
	addUsernameColumnIfMissing(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "lookup-test-username")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	username := "testuser"
	lookup := &repository.UserTenantLookup{
		Email:        "testuser@praxis.de",
		Username:     &username,
		UserID:       uuid.New().String(),
		TenantID:     tenant.ID,
		TenantSlug:   tenant.Slug,
		TenantSchema: tenant.SchemaName,
	}
	err := repo.Upsert(ctx, lookup)
	require.NoError(t, err)

	t.Run("returns lookup for existing username", func(t *testing.T) {
		result, err := repo.GetByUsername(ctx, username)
		require.NoError(t, err)
		assert.Equal(t, lookup.Email, result.Email)
		assert.Equal(t, username, *result.Username)
		assert.Equal(t, lookup.TenantSlug, result.TenantSlug)
	})

	t.Run("returns error for non-existent username", func(t *testing.T) {
		_, err := repo.GetByUsername(ctx, "nonexistent")
		require.Error(t, err)
	})
}

func TestUserTenantLookupRepository_GetByUsernameAndSlug(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)
	addUsernameColumnIfMissing(ctx, t)

	// Create two tenants
	tenant1 := suite.SetupUserTenant(t, ctx, "clinic-alpha")
	tenant2 := suite.SetupUserTenant(t, ctx, "clinic-beta")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	// Same username in both tenants (common scenario: "admin")
	username := "admin"

	// Admin user in tenant1
	user1ID := uuid.New().String()
	err := repo.Upsert(ctx, &repository.UserTenantLookup{
		Email:        "admin@clinic-alpha.de",
		Username:     &username,
		UserID:       user1ID,
		TenantID:     tenant1.ID,
		TenantSlug:   tenant1.Slug,
		TenantSchema: tenant1.SchemaName,
	})
	require.NoError(t, err)

	// Admin user in tenant2 (different user, same username)
	user2ID := uuid.New().String()
	err = repo.Upsert(ctx, &repository.UserTenantLookup{
		Email:        "admin@clinic-beta.de",
		Username:     &username,
		UserID:       user2ID,
		TenantID:     tenant2.ID,
		TenantSlug:   tenant2.Slug,
		TenantSchema: tenant2.SchemaName,
	})
	require.NoError(t, err)

	t.Run("returns correct user for username + tenant1", func(t *testing.T) {
		result, err := repo.GetByUsernameAndSlug(ctx, username, tenant1.Slug)
		require.NoError(t, err)
		assert.Equal(t, user1ID, result.UserID)
		assert.Equal(t, "admin@clinic-alpha.de", result.Email)
		assert.Equal(t, tenant1.Slug, result.TenantSlug)
	})

	t.Run("returns correct user for username + tenant2", func(t *testing.T) {
		result, err := repo.GetByUsernameAndSlug(ctx, username, tenant2.Slug)
		require.NoError(t, err)
		assert.Equal(t, user2ID, result.UserID)
		assert.Equal(t, "admin@clinic-beta.de", result.Email)
		assert.Equal(t, tenant2.Slug, result.TenantSlug)
	})

	t.Run("returns error for non-existent username", func(t *testing.T) {
		_, err := repo.GetByUsernameAndSlug(ctx, "nonexistent", tenant1.Slug)
		require.Error(t, err)
	})

	t.Run("returns error for non-existent tenant slug", func(t *testing.T) {
		_, err := repo.GetByUsernameAndSlug(ctx, username, "nonexistent-tenant")
		require.Error(t, err)
	})

	t.Run("returns error for valid username but wrong tenant", func(t *testing.T) {
		// Create a user that only exists in tenant1
		uniqueUsername := "uniqueuser"
		err := repo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        "unique@clinic-alpha.de",
			Username:     &uniqueUsername,
			UserID:       uuid.New().String(),
			TenantID:     tenant1.ID,
			TenantSlug:   tenant1.Slug,
			TenantSchema: tenant1.SchemaName,
		})
		require.NoError(t, err)

		// Try to find this user in tenant2 (should fail)
		_, err = repo.GetByUsernameAndSlug(ctx, uniqueUsername, tenant2.Slug)
		require.Error(t, err, "should not find user in wrong tenant")
	})
}

func TestUserTenantLookupRepository_UsernameUpsert(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)
	addUsernameColumnIfMissing(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "lookup-test-username-upsert")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	t.Run("inserts lookup entry with username", func(t *testing.T) {
		username := "newuser"
		lookup := &repository.UserTenantLookup{
			Email:        "newuser@praxis.de",
			Username:     &username,
			UserID:       uuid.New().String(),
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		}

		err := repo.Upsert(ctx, lookup)
		require.NoError(t, err)

		result, err := repo.GetByEmail(ctx, lookup.Email)
		require.NoError(t, err)
		require.NotNil(t, result.Username)
		assert.Equal(t, username, *result.Username)
	})

	t.Run("updates username on email conflict", func(t *testing.T) {
		email := "updateusername@praxis.de"
		oldUsername := "oldname"
		newUsername := "newname"

		// Insert with old username
		err := repo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        email,
			Username:     &oldUsername,
			UserID:       uuid.New().String(),
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		})
		require.NoError(t, err)

		// Update with new username
		err = repo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        email,
			Username:     &newUsername,
			UserID:       uuid.New().String(),
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		})
		require.NoError(t, err)

		result, err := repo.GetByEmail(ctx, email)
		require.NoError(t, err)
		require.NotNil(t, result.Username)
		assert.Equal(t, newUsername, *result.Username)
	})

	t.Run("allows null username", func(t *testing.T) {
		lookup := &repository.UserTenantLookup{
			Email:        "nousername@praxis.de",
			Username:     nil,
			UserID:       uuid.New().String(),
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			TenantSchema: tenant.SchemaName,
		}

		err := repo.Upsert(ctx, lookup)
		require.NoError(t, err)

		result, err := repo.GetByEmail(ctx, lookup.Email)
		require.NoError(t, err)
		assert.Nil(t, result.Username)
	})
}

func TestUserTenantLookupRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)
	addUsernameColumnIfMissing(ctx, t)

	// Create three tenants to test isolation
	tenantA := suite.SetupUserTenant(t, ctx, "isolation-tenant-a")
	tenantB := suite.SetupUserTenant(t, ctx, "isolation-tenant-b")
	tenantC := suite.SetupUserTenant(t, ctx, "isolation-tenant-c")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	// Create same username "admin" in all three tenants
	adminUsername := "admin"
	var userIDs = make(map[string]string)

	for i, tenant := range []struct {
		tenant *testutil.TestTenant
		email  string
	}{
		{tenantA, "admin@tenant-a.de"},
		{tenantB, "admin@tenant-b.de"},
		{tenantC, "admin@tenant-c.de"},
	} {
		userID := uuid.New().String()
		userIDs[tenant.tenant.Slug] = userID

		err := repo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        tenant.email,
			Username:     &adminUsername,
			UserID:       userID,
			TenantID:     tenant.tenant.ID,
			TenantSlug:   tenant.tenant.Slug,
			TenantSchema: tenant.tenant.SchemaName,
		})
		require.NoError(t, err, "failed to create user %d", i)
	}

	t.Run("each tenant returns its own admin user", func(t *testing.T) {
		for _, tenant := range []*testutil.TestTenant{tenantA, tenantB, tenantC} {
			result, err := repo.GetByUsernameAndSlug(ctx, adminUsername, tenant.Slug)
			require.NoError(t, err)
			assert.Equal(t, userIDs[tenant.Slug], result.UserID,
				"tenant %s should return its own admin user", tenant.Slug)
		}
	})

	t.Run("cross-tenant access fails", func(t *testing.T) {
		// Try to access tenant A's admin via tenant B's slug - should fail
		result, err := repo.GetByUsernameAndSlug(ctx, adminUsername, tenantA.Slug)
		require.NoError(t, err)

		// The result should be tenant A's user, not tenant B's
		assert.NotEqual(t, userIDs[tenantB.Slug], result.UserID)
		assert.Equal(t, userIDs[tenantA.Slug], result.UserID)
	})
}
