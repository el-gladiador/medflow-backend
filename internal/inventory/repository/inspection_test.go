package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Inspection Repository Tests ---

func TestInspectionRepository_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "insp-create")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Inspection Device")

	repo := repository.NewInspectionRepository(suite.DB)

	insp := &repository.DeviceInspection{
		ItemID:         item.ID,
		InspectionType: "STK",
		InspectionDate: time.Now().UTC().Truncate(time.Second),
		Result:         "passed",
		PerformedBy:    "TUeV Sued",
	}
	err := repo.Create(tenantCtx, insp)
	require.NoError(t, err)
	assert.NotEmpty(t, insp.ID)
	assert.False(t, insp.CreatedAt.IsZero())
	assert.False(t, insp.UpdatedAt.IsZero())
}

func TestInspectionRepository_GetByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "insp-getbyid")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Inspection GetByID Device")

	repo := repository.NewInspectionRepository(suite.DB)

	insp := &repository.DeviceInspection{
		ItemID:         item.ID,
		InspectionType: "MTK",
		InspectionDate: time.Now().UTC().Truncate(time.Second),
		Result:         "passed",
		PerformedBy:    "Internal",
	}
	require.NoError(t, repo.Create(tenantCtx, insp))

	got, err := repo.GetByID(tenantCtx, insp.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, insp.ID, got.ID)
	assert.Equal(t, "MTK", got.InspectionType)
	assert.Equal(t, "passed", got.Result)
	assert.Equal(t, "Internal", got.PerformedBy)
}

func TestInspectionRepository_ListByItem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "insp-list")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Inspection List Device")

	repo := repository.NewInspectionRepository(suite.DB)

	for _, typ := range []string{"STK", "MTK"} {
		require.NoError(t, repo.Create(tenantCtx, &repository.DeviceInspection{
			ItemID:         item.ID,
			InspectionType: typ,
			InspectionDate: time.Now().UTC().Truncate(time.Second),
			Result:         "passed",
			PerformedBy:    "Test",
		}))
	}

	list, err := repo.ListByItem(tenantCtx, item.ID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestInspectionRepository_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "insp-update")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Inspection Update Device")

	repo := repository.NewInspectionRepository(suite.DB)

	insp := &repository.DeviceInspection{
		ItemID:         item.ID,
		InspectionType: "STK",
		InspectionDate: time.Now().UTC().Truncate(time.Second),
		Result:         "passed",
		PerformedBy:    "Test",
	}
	require.NoError(t, repo.Create(tenantCtx, insp))

	insp.Result = "conditional"
	insp.PerformedBy = "Updated Tester"
	err := repo.Update(tenantCtx, insp)
	require.NoError(t, err)

	got, err := repo.GetByID(tenantCtx, insp.ID)
	require.NoError(t, err)
	assert.Equal(t, "conditional", got.Result)
	assert.Equal(t, "Updated Tester", got.PerformedBy)
}

func TestInspectionRepository_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "insp-delete")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Inspection Delete Device")

	repo := repository.NewInspectionRepository(suite.DB)

	insp := &repository.DeviceInspection{
		ItemID:         item.ID,
		InspectionType: "STK",
		InspectionDate: time.Now().UTC().Truncate(time.Second),
		Result:         "passed",
		PerformedBy:    "Test",
	}
	require.NoError(t, repo.Create(tenantCtx, insp))
	require.NoError(t, repo.Delete(tenantCtx, insp.ID))

	_, err := repo.GetByID(tenantCtx, insp.ID)
	assert.Error(t, err)
}

func TestInspectionRepository_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()

	tenant1 := suite.SetupInventoryTenant(t, ctx, "insp-iso-1")
	tenant2 := suite.SetupInventoryTenant(t, ctx, "insp-iso-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	itemRepo := repository.NewItemRepository(suite.DB)
	repo := repository.NewInspectionRepository(suite.DB)

	// Create item + inspection in tenant 1
	item1 := createTestItem(t, ctx1, itemRepo, "Tenant1 Inspection Device")
	insp := &repository.DeviceInspection{
		ItemID:         item1.ID,
		InspectionType: "STK",
		InspectionDate: time.Now().UTC().Truncate(time.Second),
		Result:         "passed",
		PerformedBy:    "Test",
	}
	require.NoError(t, repo.Create(ctx1, insp))

	// Tenant 2 should NOT see tenant 1's inspection
	_, err := repo.GetByID(ctx2, insp.ID)
	assert.Error(t, err, "tenant2 should NOT see tenant1's inspection")

	// Tenant 2 should get empty list for tenant 1's item
	list, err := repo.ListByItem(ctx2, item1.ID)
	require.NoError(t, err)
	assert.Len(t, list, 0, "tenant2 should see 0 inspections for tenant1's item")

	// Tenant 1 should see its own inspection
	list1, err := repo.ListByItem(ctx1, item1.ID)
	require.NoError(t, err)
	assert.Len(t, list1, 1)
	assert.Equal(t, insp.ID, list1[0].ID)
}
