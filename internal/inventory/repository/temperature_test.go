package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Temperature Repository Tests ---

// createTestStorageRoom creates a storage room for FK references in temperature tests.
func createTestStorageRoom(t *testing.T, tenantCtx context.Context, locRepo *repository.LocationRepository, name string) *repository.StorageRoom {
	t.Helper()
	room := &repository.StorageRoom{
		Name:     name,
		IsActive: true,
	}
	err := locRepo.CreateRoom(tenantCtx, room)
	require.NoError(t, err)
	return room
}

// createTestCabinet creates a temperature-controlled cabinet for FK references in temperature tests.
func createTestCabinet(t *testing.T, tenantCtx context.Context, locRepo *repository.LocationRepository, roomID, name string) *repository.StorageCabinet {
	t.Helper()
	targetTemp := 4.0
	minTemp := 2.0
	maxTemp := 8.0
	cabinet := &repository.StorageCabinet{
		RoomID:                       roomID,
		Name:                         name,
		TemperatureControlled:        true,
		TargetTemperature:            &targetTemp,
		MinTemperature:               &minTemp,
		MaxTemperature:               &maxTemp,
		TemperatureMonitoringEnabled: true,
		IsActive:                     true,
	}
	err := locRepo.CreateCabinet(tenantCtx, cabinet)
	require.NoError(t, err)
	return cabinet
}

func TestTemperatureRepository_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "temp-create")
	tenantCtx := suite.TenantContext(tenant)

	locRepo := repository.NewLocationRepository(suite.DB)
	room := createTestStorageRoom(t, tenantCtx, locRepo, "Temp Create Room")
	cabinet := createTestCabinet(t, tenantCtx, locRepo, room.ID, "Temp Create Cabinet")

	repo := repository.NewTemperatureRepository(suite.DB)

	reading := &repository.TemperatureReading{
		CabinetID:          cabinet.ID,
		TemperatureCelsius: 4.2,
		RecordedAt:         time.Now().UTC().Truncate(time.Second),
		Source:             "manual",
		IsExcursion:        false,
	}
	err := repo.Create(tenantCtx, reading)
	require.NoError(t, err)
	assert.NotEmpty(t, reading.ID)
	assert.False(t, reading.CreatedAt.IsZero())
}

func TestTemperatureRepository_ListByCabinet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "temp-list")
	tenantCtx := suite.TenantContext(tenant)

	locRepo := repository.NewLocationRepository(suite.DB)
	room := createTestStorageRoom(t, tenantCtx, locRepo, "Temp List Room")
	cabinet := createTestCabinet(t, tenantCtx, locRepo, room.ID, "Temp List Cabinet")

	repo := repository.NewTemperatureRepository(suite.DB)

	baseTime := time.Now().UTC().Truncate(time.Second)

	// Create multiple readings
	for i := 0; i < 5; i++ {
		require.NoError(t, repo.Create(tenantCtx, &repository.TemperatureReading{
			CabinetID:          cabinet.ID,
			TemperatureCelsius: 3.5 + float64(i)*0.5,
			RecordedAt:         baseTime.Add(time.Duration(i) * time.Hour),
			Source:             "manual",
			IsExcursion:        false,
		}))
	}

	// List all readings (page 1, 10 per page)
	readings, total, err := repo.ListByCabinet(tenantCtx, cabinet.ID, nil, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, readings, 5)

	// Test pagination (page 1, 2 per page)
	readings2, total2, err := repo.ListByCabinet(tenantCtx, cabinet.ID, nil, nil, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total2)
	assert.Len(t, readings2, 2)

	// Test date range filter
	fromTime := baseTime.Add(1 * time.Hour)
	toTime := baseTime.Add(3 * time.Hour)
	filtered, filteredTotal, err := repo.ListByCabinet(tenantCtx, cabinet.ID, &fromTime, &toTime, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(3), filteredTotal)
	assert.Len(t, filtered, 3)
}

func TestTemperatureRepository_GetLatestByCabinet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "temp-latest")
	tenantCtx := suite.TenantContext(tenant)

	locRepo := repository.NewLocationRepository(suite.DB)
	room := createTestStorageRoom(t, tenantCtx, locRepo, "Temp Latest Room")
	cabinet := createTestCabinet(t, tenantCtx, locRepo, room.ID, "Temp Latest Cabinet")

	repo := repository.NewTemperatureRepository(suite.DB)

	baseTime := time.Now().UTC().Truncate(time.Second)

	// Create readings at different times
	require.NoError(t, repo.Create(tenantCtx, &repository.TemperatureReading{
		CabinetID:          cabinet.ID,
		TemperatureCelsius: 3.0,
		RecordedAt:         baseTime.Add(-2 * time.Hour),
		Source:             "manual",
		IsExcursion:        false,
	}))

	require.NoError(t, repo.Create(tenantCtx, &repository.TemperatureReading{
		CabinetID:          cabinet.ID,
		TemperatureCelsius: 5.5,
		RecordedAt:         baseTime, // most recent
		Source:             "manual",
		IsExcursion:        false,
	}))

	require.NoError(t, repo.Create(tenantCtx, &repository.TemperatureReading{
		CabinetID:          cabinet.ID,
		TemperatureCelsius: 4.0,
		RecordedAt:         baseTime.Add(-1 * time.Hour),
		Source:             "manual",
		IsExcursion:        false,
	}))

	latest, err := repo.GetLatestByCabinet(tenantCtx, cabinet.ID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, 5.5, latest.TemperatureCelsius)
	assert.Equal(t, baseTime.Unix(), latest.RecordedAt.Unix())
}

func TestTemperatureRepository_ExcursionDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "temp-excursion")
	tenantCtx := suite.TenantContext(tenant)

	locRepo := repository.NewLocationRepository(suite.DB)
	room := createTestStorageRoom(t, tenantCtx, locRepo, "Temp Excursion Room")
	cabinet := createTestCabinet(t, tenantCtx, locRepo, room.ID, "Temp Excursion Cabinet")

	repo := repository.NewTemperatureRepository(suite.DB)

	baseTime := time.Now().UTC().Truncate(time.Second)

	// Create normal readings
	require.NoError(t, repo.Create(tenantCtx, &repository.TemperatureReading{
		CabinetID:          cabinet.ID,
		TemperatureCelsius: 4.0,
		RecordedAt:         baseTime.Add(-3 * time.Hour),
		Source:             "manual",
		IsExcursion:        false,
	}))

	require.NoError(t, repo.Create(tenantCtx, &repository.TemperatureReading{
		CabinetID:          cabinet.ID,
		TemperatureCelsius: 3.5,
		RecordedAt:         baseTime.Add(-1 * time.Hour),
		Source:             "manual",
		IsExcursion:        false,
	}))

	// Create excursion readings (temperature out of range)
	require.NoError(t, repo.Create(tenantCtx, &repository.TemperatureReading{
		CabinetID:          cabinet.ID,
		TemperatureCelsius: 12.0,
		RecordedAt:         baseTime.Add(-2 * time.Hour),
		Source:             "manual",
		IsExcursion:        true,
		Notes:              strPtr("Temperature excursion detected - above max threshold"),
	}))

	require.NoError(t, repo.Create(tenantCtx, &repository.TemperatureReading{
		CabinetID:          cabinet.ID,
		TemperatureCelsius: -1.0,
		RecordedAt:         baseTime,
		Source:             "sensor",
		IsExcursion:        true,
		Notes:              strPtr("Temperature excursion detected - below min threshold"),
	}))

	// Get excursions in the time range
	from := baseTime.Add(-4 * time.Hour)
	to := baseTime.Add(1 * time.Hour)
	excursions, err := repo.GetExcursions(tenantCtx, cabinet.ID, from, to)
	require.NoError(t, err)
	assert.Len(t, excursions, 2, "should find exactly 2 excursion readings")

	// Verify all returned readings are excursions
	for _, exc := range excursions {
		assert.True(t, exc.IsExcursion, "returned reading should be an excursion")
	}

	// Verify narrower range returns fewer excursions
	narrowFrom := baseTime.Add(-90 * time.Minute)
	narrowTo := baseTime.Add(-30 * time.Minute)
	narrowExcursions, err := repo.GetExcursions(tenantCtx, cabinet.ID, narrowFrom, narrowTo)
	require.NoError(t, err)
	assert.Len(t, narrowExcursions, 0, "should find 0 excursions in narrow range that misses them")
}

func TestTemperatureRepository_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()

	tenant1 := suite.SetupInventoryTenant(t, ctx, "temp-iso-1")
	tenant2 := suite.SetupInventoryTenant(t, ctx, "temp-iso-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	locRepo := repository.NewLocationRepository(suite.DB)
	repo := repository.NewTemperatureRepository(suite.DB)

	// Create room + cabinet + reading in tenant 1
	room1 := createTestStorageRoom(t, ctx1, locRepo, "Tenant1 Room")
	cabinet1 := createTestCabinet(t, ctx1, locRepo, room1.ID, "Tenant1 Cabinet")

	require.NoError(t, repo.Create(ctx1, &repository.TemperatureReading{
		CabinetID:          cabinet1.ID,
		TemperatureCelsius: 4.0,
		RecordedAt:         time.Now().UTC().Truncate(time.Second),
		Source:             "manual",
		IsExcursion:        false,
	}))

	// Tenant 2 should NOT see tenant 1's readings
	readings, total, err := repo.ListByCabinet(ctx2, cabinet1.ID, nil, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total, "tenant2 should see 0 readings for tenant1's cabinet")
	assert.Len(t, readings, 0)

	// Tenant 2's GetLatestByCabinet for tenant 1's cabinet should fail
	_, err = repo.GetLatestByCabinet(ctx2, cabinet1.ID)
	assert.Error(t, err, "tenant2 should NOT see tenant1's latest reading")

	// Tenant 1 should see its own reading
	readings1, total1, err := repo.ListByCabinet(ctx1, cabinet1.ID, nil, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total1)
	assert.Len(t, readings1, 1)
	assert.Equal(t, cabinet1.ID, readings1[0].CabinetID)
}
