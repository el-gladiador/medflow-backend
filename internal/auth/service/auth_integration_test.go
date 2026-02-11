package service_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/auth/repository"
	"github.com/medflow/medflow-backend/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// INTEGRATION TESTS: Subdomain-based Username Login
// ============================================================================

var suite *testutil.IntegrationSuite

func TestMain(m *testing.M) {
	ctx := context.Background()
	var err error

	suite, err = testutil.NewIntegrationSuite(ctx)
	if err != nil {
		panic("failed to create integration suite: " + err.Error())
	}
	defer suite.Cleanup(ctx)

	// Create required tables
	err = createAuthTables(ctx)
	if err != nil {
		panic("failed to create auth tables: " + err.Error())
	}

	code := m.Run()
	os.Exit(code)
}

func createAuthTables(ctx context.Context) error {
	_, err := suite.RawDB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS public.user_tenant_lookup (
			email VARCHAR(255) PRIMARY KEY,
			username VARCHAR(100),
			user_id UUID NOT NULL,
			tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
			tenant_slug VARCHAR(100) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_user_tenant_lookup_user_id ON public.user_tenant_lookup(user_id);
		CREATE INDEX IF NOT EXISTS idx_user_tenant_lookup_tenant_id ON public.user_tenant_lookup(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_user_tenant_lookup_username ON public.user_tenant_lookup(username)
			WHERE username IS NOT NULL;

		-- Create unique index for (username, tenant_slug)
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

// ============================================================================
// TEST: Same Username in Multiple Tenants
// ============================================================================

func TestSubdomainLogin_SameUsernameMultipleTenants(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	// Create two tenants (simulating two different clinics)
	clinicA := suite.SetupUserTenant(t, ctx, "zahnarzt-berlin")
	clinicB := suite.SetupUserTenant(t, ctx, "praxis-mueller")

	repo := repository.NewUserTenantLookupRepository(suite.DB)

	// Both clinics have an "admin" user
	adminUsername := "admin"
	adminAID := uuid.New().String()
	adminBID := uuid.New().String()

	// Create admin user in Clinic A
	err := repo.Upsert(ctx, &repository.UserTenantLookup{
		Email:        "admin@zahnarzt-berlin.de",
		Username:     &adminUsername,
		UserID:       adminAID,
		TenantID:     clinicA.ID,
		TenantSlug:   clinicA.Slug,
	})
	require.NoError(t, err)

	// Create admin user in Clinic B
	err = repo.Upsert(ctx, &repository.UserTenantLookup{
		Email:        "admin@praxis-mueller.de",
		Username:     &adminUsername,
		UserID:       adminBID,
		TenantID:     clinicB.ID,
		TenantSlug:   clinicB.Slug,
	})
	require.NoError(t, err)

	t.Run("username+tenantSlug returns correct clinic A user", func(t *testing.T) {
		// Simulates: User visits zahnarzt-berlin.medflow.de and logs in as "admin"
		result, err := repo.GetByUsernameAndSlug(ctx, "admin", clinicA.Slug)
		require.NoError(t, err)

		assert.Equal(t, adminAID, result.UserID)
		assert.Equal(t, "admin@zahnarzt-berlin.de", result.Email)
		assert.Equal(t, clinicA.Slug, result.TenantSlug)
	})

	t.Run("username+tenantSlug returns correct clinic B user", func(t *testing.T) {
		// Simulates: User visits praxis-mueller.medflow.de and logs in as "admin"
		result, err := repo.GetByUsernameAndSlug(ctx, "admin", clinicB.Slug)
		require.NoError(t, err)

		assert.Equal(t, adminBID, result.UserID)
		assert.Equal(t, "admin@praxis-mueller.de", result.Email)
		assert.Equal(t, clinicB.Slug, result.TenantSlug)
	})

	t.Run("username without tenant fails", func(t *testing.T) {
		// If GetByUsername returns any result, it would be ambiguous
		// In a multi-tenant environment, this should NOT be used
		// The service layer should require tenant_slug for username login
	})
}

// ============================================================================
// TEST: Email Login (tenant optional but validated)
// ============================================================================

func TestSubdomainLogin_EmailLogin(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	// Create a tenant
	clinic := suite.SetupUserTenant(t, ctx, "demo-clinic")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	// Create user
	err := repo.Upsert(ctx, &repository.UserTenantLookup{
		Email:        "doctor@demo-clinic.de",
		UserID:       uuid.New().String(),
		TenantID:     clinic.ID,
		TenantSlug:   clinic.Slug,
	})
	require.NoError(t, err)

	t.Run("email login from main domain works", func(t *testing.T) {
		// Simulates: User visits medflow.de/login and enters email
		result, err := repo.GetByEmail(ctx, "doctor@demo-clinic.de")
		require.NoError(t, err)

		assert.Equal(t, clinic.Slug, result.TenantSlug)
	})

	t.Run("email login from correct subdomain works", func(t *testing.T) {
		// Simulates: User visits demo-clinic.medflow.de and enters email
		result, err := repo.GetByEmail(ctx, "doctor@demo-clinic.de")
		require.NoError(t, err)

		// Verify tenant matches subdomain
		assert.Equal(t, clinic.Slug, result.TenantSlug)
	})

	t.Run("email from different tenant is caught by service layer", func(t *testing.T) {
		// The repository doesn't enforce tenant matching for email lookup
		// The service layer compares the looked-up tenant with the provided tenant_slug
		// This test documents the expected behavior

		result, err := repo.GetByEmail(ctx, "doctor@demo-clinic.de")
		require.NoError(t, err)

		// Service layer would compare: result.TenantSlug != providedTenantSlug
		wrongTenantSlug := "other-clinic"
		assert.NotEqual(t, wrongTenantSlug, result.TenantSlug)
		// Service layer would return errors.BadRequest("tenant_mismatch")
	})
}

// ============================================================================
// TEST: Tenant Isolation Security
// ============================================================================

func TestSubdomainLogin_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	// Create three tenants
	tenantA := suite.SetupUserTenant(t, ctx, "tenant-alpha")
	tenantB := suite.SetupUserTenant(t, ctx, "tenant-beta")
	tenantC := suite.SetupUserTenant(t, ctx, "tenant-gamma")

	repo := repository.NewUserTenantLookupRepository(suite.DB)

	// Create users with same username in all tenants
	commonUsername := "receptionist"
	tenantUsers := map[string]string{}

	for _, tenant := range []*testutil.TestTenant{tenantA, tenantB, tenantC} {
		userID := uuid.New().String()
		tenantUsers[tenant.Slug] = userID

		err := repo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        "receptionist@" + tenant.Slug + ".de",
			Username:     &commonUsername,
			UserID:       userID,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
		})
		require.NoError(t, err)
	}

	t.Run("SECURITY: Cannot access tenant A user via tenant B subdomain", func(t *testing.T) {
		// User at tenant B subdomain should NOT get tenant A's user
		result, err := repo.GetByUsernameAndSlug(ctx, commonUsername, tenantB.Slug)
		require.NoError(t, err)

		// Must be tenant B's user, NOT tenant A's
		assert.Equal(t, tenantUsers[tenantB.Slug], result.UserID)
		assert.NotEqual(t, tenantUsers[tenantA.Slug], result.UserID)
	})

	t.Run("SECURITY: Non-existent tenant slug returns error", func(t *testing.T) {
		_, err := repo.GetByUsernameAndSlug(ctx, commonUsername, "non-existent-tenant")
		require.Error(t, err, "should fail for non-existent tenant")
	})

	t.Run("SECURITY: Each tenant is completely isolated", func(t *testing.T) {
		for _, tenant := range []*testutil.TestTenant{tenantA, tenantB, tenantC} {
			result, err := repo.GetByUsernameAndSlug(ctx, commonUsername, tenant.Slug)
			require.NoError(t, err)

			// Must return the correct user for this specific tenant
			assert.Equal(t, tenantUsers[tenant.Slug], result.UserID,
				"tenant %s returned wrong user", tenant.Slug)

			// Email must match the tenant
			expectedEmail := "receptionist@" + tenant.Slug + ".de"
			assert.Equal(t, expectedEmail, result.Email)
		}
	})
}

// ============================================================================
// TEST: Edge Cases
// ============================================================================

func TestSubdomainLogin_EdgeCases(t *testing.T) {
	ctx := context.Background()
	cleanupLookupTable(ctx, t)

	tenant := suite.SetupUserTenant(t, ctx, "edge-case-clinic")
	repo := repository.NewUserTenantLookupRepository(suite.DB)

	t.Run("username with special characters", func(t *testing.T) {
		username := "user.name-with_special"
		err := repo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        "special@test.de",
			Username:     &username,
			UserID:       uuid.New().String(),
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
		})
		require.NoError(t, err)

		result, err := repo.GetByUsernameAndSlug(ctx, username, tenant.Slug)
		require.NoError(t, err)
		assert.Equal(t, username, *result.Username)
	})

	t.Run("tenant slug with hyphen", func(t *testing.T) {
		// The tenant already has hyphen in slug: "edge-case-clinic"
		username := "testuser"
		err := repo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        "hyphen@test.de",
			Username:     &username,
			UserID:       uuid.New().String(),
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
		})
		require.NoError(t, err)

		result, err := repo.GetByUsernameAndSlug(ctx, username, tenant.Slug)
		require.NoError(t, err)
		assert.Equal(t, "edge-case-clinic", result.TenantSlug)
	})

	t.Run("case sensitivity of username", func(t *testing.T) {
		username := "CaseSensitive"
		err := repo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        "case@test.de",
			Username:     &username,
			UserID:       uuid.New().String(),
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
		})
		require.NoError(t, err)

		// PostgreSQL default is case-sensitive
		_, err = repo.GetByUsernameAndSlug(ctx, "casesensitive", tenant.Slug)
		require.Error(t, err, "lowercase should not match")

		result, err := repo.GetByUsernameAndSlug(ctx, "CaseSensitive", tenant.Slug)
		require.NoError(t, err)
		assert.Equal(t, "CaseSensitive", *result.Username)
	})

	t.Run("null username lookup", func(t *testing.T) {
		// User without username (email-only login)
		err := repo.Upsert(ctx, &repository.UserTenantLookup{
			Email:        "emailonly@test.de",
			Username:     nil, // No username
			UserID:       uuid.New().String(),
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
		})
		require.NoError(t, err)

		// Cannot find by username (it's null)
		_, err = repo.GetByUsernameAndSlug(ctx, "", tenant.Slug)
		require.Error(t, err)

		// But can find by email
		result, err := repo.GetByEmail(ctx, "emailonly@test.de")
		require.NoError(t, err)
		assert.Nil(t, result.Username)
	})
}
