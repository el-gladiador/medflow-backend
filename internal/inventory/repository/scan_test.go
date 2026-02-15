package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- GetByBatchNumber Tests ---

func TestGetByBatchNumber_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "scan-batch-found")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	batchRepo := repository.NewBatchRepository(suite.DB)

	// Create parent item
	item := createTestItem(t, tenantCtx, itemRepo, "Scan Batch Item")

	// Create batch with a known batch number
	batch := &repository.InventoryBatch{
		ItemID:          item.ID,
		BatchNumber:     "SCAN-BATCH-001",
		InitialQuantity: 50,
		CurrentQuantity: 50,
		ReceivedDate:    time.Now().UTC().Truncate(time.Second),
	}
	err := batchRepo.Create(tenantCtx, batch)
	require.NoError(t, err)

	// Look up by batch number
	found, err := batchRepo.GetByBatchNumber(tenantCtx, "SCAN-BATCH-001")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, batch.ID, found.ID)
	assert.Equal(t, "SCAN-BATCH-001", found.BatchNumber)
	assert.Equal(t, item.ID, found.ItemID)
	assert.Equal(t, 50, found.CurrentQuantity)
	assert.Equal(t, 50, found.Quantity) // Computed field
}

func TestGetByBatchNumber_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "scan-batch-notfound")
	tenantCtx := suite.TenantContext(tenant)

	batchRepo := repository.NewBatchRepository(suite.DB)

	// Look up a non-existent batch number
	found, err := batchRepo.GetByBatchNumber(tenantCtx, "NONEXISTENT-BATCH-999")
	assert.Nil(t, found)
	require.Error(t, err)

	// Verify it's a NotFound error
	var appErr *errors.AppError
	require.True(t, errors.As(err, &appErr), "expected AppError, got %T", err)
	assert.Equal(t, "NOT_FOUND", appErr.Code)
}

func TestGetByBatchNumber_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()

	tenant1 := suite.SetupInventoryTenant(t, ctx, "scan-batch-iso-1")
	tenant2 := suite.SetupInventoryTenant(t, ctx, "scan-batch-iso-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	itemRepo := repository.NewItemRepository(suite.DB)
	batchRepo := repository.NewBatchRepository(suite.DB)

	// Create item + batch in tenant 1
	item1 := createTestItem(t, ctx1, itemRepo, "Tenant1 Scan Item")
	batch1 := &repository.InventoryBatch{
		ItemID:          item1.ID,
		BatchNumber:     "ISO-BATCH-T1",
		InitialQuantity: 25,
		CurrentQuantity: 25,
		ReceivedDate:    time.Now().UTC().Truncate(time.Second),
	}
	err := batchRepo.Create(ctx1, batch1)
	require.NoError(t, err)

	// Tenant 1 should find its own batch
	found1, err := batchRepo.GetByBatchNumber(ctx1, "ISO-BATCH-T1")
	require.NoError(t, err)
	require.NotNil(t, found1)
	assert.Equal(t, batch1.ID, found1.ID)

	// Tenant 2 should NOT see tenant 1's batch (RLS blocks it)
	found2, err := batchRepo.GetByBatchNumber(ctx2, "ISO-BATCH-T1")
	assert.Nil(t, found2)
	assert.Error(t, err, "tenant2 should NOT see tenant1's batch")
}

// --- GetByArticleNumber Tests ---

func TestGetByArticleNumber_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "scan-article-found")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)

	// Create item with an article number
	articleNum := "ART-2026-001"
	item := &repository.InventoryItem{
		Name:          "Dental Composite",
		Category:      "Supplies",
		Unit:          "pieces",
		MinStock:      10,
		IsActive:      true,
		ArticleNumber: &articleNum,
	}
	err := itemRepo.Create(tenantCtx, item)
	require.NoError(t, err)

	// Look up by article number
	found, err := itemRepo.GetByArticleNumber(tenantCtx, "ART-2026-001")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, item.ID, found.ID)
	assert.Equal(t, "Dental Composite", found.Name)
	assert.Equal(t, "ART-2026-001", *found.ArticleNumber)
}

func TestGetByArticleNumber_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "scan-article-notfound")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)

	// Look up a non-existent article number
	found, err := itemRepo.GetByArticleNumber(tenantCtx, "NONEXISTENT-ART-999")
	assert.Nil(t, found)
	require.Error(t, err)

	// Verify it's a NotFound error
	var appErr *errors.AppError
	require.True(t, errors.As(err, &appErr), "expected AppError, got %T", err)
	assert.Equal(t, "NOT_FOUND", appErr.Code)
}

// --- GetByBarcode Tests ---

func TestGetByBarcode_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "scan-barcode-found")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)

	// Create item with a barcode
	barcode := "4012345678901"
	item := &repository.InventoryItem{
		Name:     "Barcode Test Item",
		Category: "Supplies",
		Unit:     "pieces",
		MinStock: 5,
		IsActive: true,
		Barcode:  &barcode,
	}
	err := itemRepo.Create(tenantCtx, item)
	require.NoError(t, err)

	// Look up by barcode
	found, err := itemRepo.GetByBarcode(tenantCtx, "4012345678901")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, item.ID, found.ID)
	assert.Equal(t, "Barcode Test Item", found.Name)
	assert.Equal(t, "4012345678901", *found.Barcode)
}

func TestGetByBarcode_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "scan-barcode-notfound")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)

	// Look up a non-existent barcode
	found, err := itemRepo.GetByBarcode(tenantCtx, "0000000000000")
	assert.Nil(t, found)
	require.Error(t, err)

	// Verify it's a NotFound error
	var appErr *errors.AppError
	require.True(t, errors.As(err, &appErr), "expected AppError, got %T", err)
	assert.Equal(t, "NOT_FOUND", appErr.Code)
}

// --- GetByPZN Tests ---

func TestGetByPZN_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "scan-pzn-found")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)

	// Create item with a PZN
	pzn := "08585997"
	item := &repository.InventoryItem{
		Name:     "Lidocain 2%",
		Category: "Medication",
		Unit:     "pieces",
		MinStock: 10,
		IsActive: true,
		PZN:      &pzn,
	}
	err := itemRepo.Create(tenantCtx, item)
	require.NoError(t, err)

	// Look up by PZN
	found, err := itemRepo.GetByPZN(tenantCtx, "08585997")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, item.ID, found.ID)
	assert.Equal(t, "Lidocain 2%", found.Name)
	assert.Equal(t, "08585997", *found.PZN)
}

func TestGetByPZN_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "scan-pzn-notfound")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)

	// Look up a non-existent PZN
	found, err := itemRepo.GetByPZN(tenantCtx, "99999999")
	assert.Nil(t, found)
	require.Error(t, err)

	// Verify it's a NotFound error
	var appErr *errors.AppError
	require.True(t, errors.As(err, &appErr), "expected AppError, got %T", err)
	assert.Equal(t, "NOT_FOUND", appErr.Code)
}

func TestGetByPZN_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()

	tenant1 := suite.SetupInventoryTenant(t, ctx, "scan-pzn-iso-1")
	tenant2 := suite.SetupInventoryTenant(t, ctx, "scan-pzn-iso-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	itemRepo := repository.NewItemRepository(suite.DB)

	// Create item with PZN in tenant 1
	pzn := "08585997"
	item1 := &repository.InventoryItem{
		Name:     "Tenant1 PZN Item",
		Category: "Medication",
		Unit:     "pieces",
		MinStock: 5,
		IsActive: true,
		PZN:      &pzn,
	}
	err := itemRepo.Create(ctx1, item1)
	require.NoError(t, err)

	// Tenant 1 should find its own item
	found1, err := itemRepo.GetByPZN(ctx1, "08585997")
	require.NoError(t, err)
	require.NotNil(t, found1)
	assert.Equal(t, item1.ID, found1.ID)

	// Tenant 2 should NOT see tenant 1's item (RLS blocks it)
	found2, err := itemRepo.GetByPZN(ctx2, "08585997")
	assert.Nil(t, found2)
	assert.Error(t, err, "tenant2 should NOT see tenant1's PZN item")
}

func TestGetByBarcode_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()

	tenant1 := suite.SetupInventoryTenant(t, ctx, "scan-barcode-iso-1")
	tenant2 := suite.SetupInventoryTenant(t, ctx, "scan-barcode-iso-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	itemRepo := repository.NewItemRepository(suite.DB)

	// Create item with barcode in tenant 1
	barcode := "9999999999999"
	item1 := &repository.InventoryItem{
		Name:     "Tenant1 Barcode Item",
		Category: "Supplies",
		Unit:     "pieces",
		MinStock: 5,
		IsActive: true,
		Barcode:  &barcode,
	}
	err := itemRepo.Create(ctx1, item1)
	require.NoError(t, err)

	// Tenant 1 should find its own item
	found1, err := itemRepo.GetByBarcode(ctx1, "9999999999999")
	require.NoError(t, err)
	require.NotNil(t, found1)
	assert.Equal(t, item1.ID, found1.ID)

	// Tenant 2 should NOT see tenant 1's item (RLS blocks it)
	found2, err := itemRepo.GetByBarcode(ctx2, "9999999999999")
	assert.Nil(t, found2)
	assert.Error(t, err, "tenant2 should NOT see tenant1's barcode item")
}
