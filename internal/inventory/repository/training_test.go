package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Training Repository Tests ---

func TestTrainingRepository_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "train-create")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Training Device")

	repo := repository.NewTrainingRepository(suite.DB)

	tr := &repository.DeviceTraining{
		ItemID:        item.ID,
		TrainingDate:  time.Now().UTC().Truncate(time.Second),
		TrainerName:   "Dr. Schmidt",
		AttendeeNames: "Mueller, Weber, Fischer",
	}
	err := repo.Create(tenantCtx, tr)
	require.NoError(t, err)
	assert.NotEmpty(t, tr.ID)
	assert.False(t, tr.CreatedAt.IsZero())
	assert.False(t, tr.UpdatedAt.IsZero())
}

func TestTrainingRepository_GetByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "train-getbyid")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Training GetByID Device")

	repo := repository.NewTrainingRepository(suite.DB)

	topic := "Geraeteeinweisung gemaess MPBetreibV"
	tr := &repository.DeviceTraining{
		ItemID:        item.ID,
		TrainingDate:  time.Now().UTC().Truncate(time.Second),
		TrainerName:   "Prof. Bauer",
		AttendeeNames: "Klein, Schwarz",
		Topic:         &topic,
	}
	require.NoError(t, repo.Create(tenantCtx, tr))

	got, err := repo.GetByID(tenantCtx, tr.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, tr.ID, got.ID)
	assert.Equal(t, "Prof. Bauer", got.TrainerName)
	assert.Equal(t, "Klein, Schwarz", got.AttendeeNames)
	require.NotNil(t, got.Topic)
	assert.Equal(t, topic, *got.Topic)
}

func TestTrainingRepository_ListByItem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "train-list")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Training List Device")

	repo := repository.NewTrainingRepository(suite.DB)

	// Create multiple trainings
	for i, trainer := range []string{"Trainer A", "Trainer B", "Trainer C"} {
		require.NoError(t, repo.Create(tenantCtx, &repository.DeviceTraining{
			ItemID:        item.ID,
			TrainingDate:  time.Now().UTC().Truncate(time.Second).Add(time.Duration(i) * time.Hour),
			TrainerName:   trainer,
			AttendeeNames: "Staff Member " + trainer,
		}))
	}

	list, err := repo.ListByItem(tenantCtx, item.ID)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func TestTrainingRepository_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "train-update")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Training Update Device")

	repo := repository.NewTrainingRepository(suite.DB)

	tr := &repository.DeviceTraining{
		ItemID:        item.ID,
		TrainingDate:  time.Now().UTC().Truncate(time.Second),
		TrainerName:   "Original Trainer",
		AttendeeNames: "Attendee A",
	}
	require.NoError(t, repo.Create(tenantCtx, tr))

	tr.TrainerName = "Updated Trainer"
	tr.AttendeeNames = "Attendee A, Attendee B"
	err := repo.Update(tenantCtx, tr)
	require.NoError(t, err)

	got, err := repo.GetByID(tenantCtx, tr.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Trainer", got.TrainerName)
	assert.Equal(t, "Attendee A, Attendee B", got.AttendeeNames)
}

func TestTrainingRepository_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "train-delete")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	item := createTestItem(t, tenantCtx, itemRepo, "Training Delete Device")

	repo := repository.NewTrainingRepository(suite.DB)

	tr := &repository.DeviceTraining{
		ItemID:        item.ID,
		TrainingDate:  time.Now().UTC().Truncate(time.Second),
		TrainerName:   "Trainer To Delete",
		AttendeeNames: "Attendee",
	}
	require.NoError(t, repo.Create(tenantCtx, tr))
	require.NoError(t, repo.Delete(tenantCtx, tr.ID))

	_, err := repo.GetByID(tenantCtx, tr.ID)
	assert.Error(t, err)

	// Verify list is empty after soft-delete
	list, err := repo.ListByItem(tenantCtx, item.ID)
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

func TestTrainingRepository_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()

	tenant1 := suite.SetupInventoryTenant(t, ctx, "train-iso-1")
	tenant2 := suite.SetupInventoryTenant(t, ctx, "train-iso-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	itemRepo := repository.NewItemRepository(suite.DB)
	repo := repository.NewTrainingRepository(suite.DB)

	// Create item + training in tenant 1
	item1 := createTestItem(t, ctx1, itemRepo, "Tenant1 Training Device")
	tr := &repository.DeviceTraining{
		ItemID:        item1.ID,
		TrainingDate:  time.Now().UTC().Truncate(time.Second),
		TrainerName:   "Tenant1 Trainer",
		AttendeeNames: "Tenant1 Staff",
	}
	require.NoError(t, repo.Create(ctx1, tr))

	// Tenant 2 should NOT see tenant 1's training
	_, err := repo.GetByID(ctx2, tr.ID)
	assert.Error(t, err, "tenant2 should NOT see tenant1's training")

	// Tenant 2 should get empty list for tenant 1's item
	list, err := repo.ListByItem(ctx2, item1.ID)
	require.NoError(t, err)
	assert.Len(t, list, 0, "tenant2 should see 0 trainings for tenant1's item")

	// Tenant 1 should see its own training
	list1, err := repo.ListByItem(ctx1, item1.ID)
	require.NoError(t, err)
	assert.Len(t, list1, 1)
	assert.Equal(t, tr.ID, list1[0].ID)
	assert.Equal(t, "Tenant1 Trainer", list1[0].TrainerName)
}
