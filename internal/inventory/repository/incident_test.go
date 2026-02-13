package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Incident Repository Tests ---

func TestIncidentRepository_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "inc-create")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Incident Device")

	repo := repository.NewIncidentRepository(suite.DB)

	inc := &repository.DeviceIncident{
		ItemID:       item.ID,
		IncidentDate: time.Now().UTC().Truncate(time.Second),
		IncidentType: "malfunction",
		Description:  "Device displayed error code E42 during use",
	}
	err := repo.Create(tenantCtx, inc)
	require.NoError(t, err)
	assert.NotEmpty(t, inc.ID)
	assert.False(t, inc.CreatedAt.IsZero())
	assert.False(t, inc.UpdatedAt.IsZero())
}

func TestIncidentRepository_GetByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "inc-getbyid")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Incident GetByID Device")

	repo := repository.NewIncidentRepository(suite.DB)

	consequences := "Device taken out of service"
	correctiveAction := "Sent to manufacturer for repair"
	inc := &repository.DeviceIncident{
		ItemID:           item.ID,
		IncidentDate:     time.Now().UTC().Truncate(time.Second),
		IncidentType:     "malfunction",
		Description:      "Unexpected shutdown during operation",
		Consequences:     &consequences,
		CorrectiveAction: &correctiveAction,
	}
	require.NoError(t, repo.Create(tenantCtx, inc))

	got, err := repo.GetByID(tenantCtx, inc.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, inc.ID, got.ID)
	assert.Equal(t, "malfunction", got.IncidentType)
	assert.Equal(t, "Unexpected shutdown during operation", got.Description)
	require.NotNil(t, got.Consequences)
	assert.Equal(t, consequences, *got.Consequences)
	require.NotNil(t, got.CorrectiveAction)
	assert.Equal(t, correctiveAction, *got.CorrectiveAction)
}

func TestIncidentRepository_ListByItem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "inc-list")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Incident List Device")

	repo := repository.NewIncidentRepository(suite.DB)

	// Create multiple incidents
	for i, desc := range []string{"First malfunction", "Second malfunction"} {
		require.NoError(t, repo.Create(tenantCtx, &repository.DeviceIncident{
			ItemID:       item.ID,
			IncidentDate: time.Now().UTC().Truncate(time.Second).Add(time.Duration(i) * time.Hour),
			IncidentType: "malfunction",
			Description:  desc,
		}))
	}

	list, err := repo.ListByItem(tenantCtx, item.ID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestIncidentRepository_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "inc-update")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Incident Update Device")

	repo := repository.NewIncidentRepository(suite.DB)

	inc := &repository.DeviceIncident{
		ItemID:       item.ID,
		IncidentDate: time.Now().UTC().Truncate(time.Second),
		IncidentType: "malfunction",
		Description:  "Original description",
	}
	require.NoError(t, repo.Create(tenantCtx, inc))

	inc.Description = "Updated description with more detail"
	reportedTo := "BfArM"
	inc.ReportedTo = &reportedTo
	err := repo.Update(tenantCtx, inc)
	require.NoError(t, err)

	got, err := repo.GetByID(tenantCtx, inc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated description with more detail", got.Description)
	require.NotNil(t, got.ReportedTo)
	assert.Equal(t, "BfArM", *got.ReportedTo)
}

func TestIncidentRepository_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "inc-delete")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Incident Delete Device")

	repo := repository.NewIncidentRepository(suite.DB)

	inc := &repository.DeviceIncident{
		ItemID:       item.ID,
		IncidentDate: time.Now().UTC().Truncate(time.Second),
		IncidentType: "malfunction",
		Description:  "Incident to delete",
	}
	require.NoError(t, repo.Create(tenantCtx, inc))
	require.NoError(t, repo.Delete(tenantCtx, inc.ID))

	_, err := repo.GetByID(tenantCtx, inc.ID)
	assert.Error(t, err)

	// Verify list is empty after soft-delete
	list, err := repo.ListByItem(tenantCtx, item.ID)
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

func TestIncidentRepository_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()

	tenant1 := suite.SetupInventoryTenant(t, ctx, "inc-iso-1")
	tenant2 := suite.SetupInventoryTenant(t, ctx, "inc-iso-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	itemRepo := repository.NewItemRepository(suite.DB)
	repo := repository.NewIncidentRepository(suite.DB)

	// Create item + incident in tenant 1
	item1 := createTestItem(t, ctx1, itemRepo, "Tenant1 Incident Device")
	inc := &repository.DeviceIncident{
		ItemID:       item1.ID,
		IncidentDate: time.Now().UTC().Truncate(time.Second),
		IncidentType: "malfunction",
		Description:  "Tenant 1 incident",
	}
	require.NoError(t, repo.Create(ctx1, inc))

	// Tenant 2 should NOT see tenant 1's incident
	_, err := repo.GetByID(ctx2, inc.ID)
	assert.Error(t, err, "tenant2 should NOT see tenant1's incident")

	// Tenant 2 should get empty list for tenant 1's item
	list, err := repo.ListByItem(ctx2, item1.ID)
	require.NoError(t, err)
	assert.Len(t, list, 0, "tenant2 should see 0 incidents for tenant1's item")

	// Tenant 1 should see its own incident
	list1, err := repo.ListByItem(ctx1, item1.ID)
	require.NoError(t, err)
	assert.Len(t, list1, 1)
	assert.Equal(t, inc.ID, list1[0].ID)
	assert.Equal(t, "Tenant 1 incident", list1[0].Description)
}
