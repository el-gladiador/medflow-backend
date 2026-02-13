package repository_test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var suite *testutil.IntegrationSuite

func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error
	suite, err = testutil.NewIntegrationSuite(ctx, "inventory, public")
	if err != nil {
		log.Fatalf("failed to create integration suite: %v", err)
	}
	defer suite.Cleanup(ctx)
	defer testutil.TerminateContainer(ctx)

	os.Exit(m.Run())
}

// Helper to create an item for tests that need a parent item
func createTestItem(t *testing.T, tenantCtx context.Context, repo *repository.ItemRepository, name string) *repository.InventoryItem {
	t.Helper()
	item := &repository.InventoryItem{
		Name:        name,
		Category:    "Supplies",
		Unit:        "pieces",
		MinStock:    5,
		IsHazardous: true,
		IsActive:    true,
	}
	err := repo.Create(tenantCtx, item)
	require.NoError(t, err)
	return item
}

func strPtr(s string) *string {
	return &s
}

// --- Hazardous Repository Tests ---

func TestHazardousRepository_Upsert_Create(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-hazardous-create")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	hazRepo := repository.NewHazardousRepository(suite.DB)

	// Create parent item
	item := createTestItem(t, tenantCtx, itemRepo, "Isopropanol")

	// Create hazardous details
	detail := &repository.HazardousSubstanceDetail{
		ItemID:              item.ID,
		GHSPictogramCodes:  strPtr("GHS02,GHS07"),
		HStatements:        strPtr("H225,H319"),
		PStatements:        strPtr("P210,P233"),
		SignalWord:          strPtr("Danger"),
		UsageArea:           strPtr("Surface disinfection"),
		StorageInstructions: strPtr("Keep away from heat"),
		EmergencyProcedures: strPtr("Ventilate area"),
	}

	err := hazRepo.Upsert(tenantCtx, detail)
	require.NoError(t, err)
	assert.NotEmpty(t, detail.ID)
	assert.False(t, detail.CreatedAt.IsZero())
	assert.False(t, detail.UpdatedAt.IsZero())
}

func TestHazardousRepository_GetByItemID(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-hazardous-get")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	hazRepo := repository.NewHazardousRepository(suite.DB)

	item := createTestItem(t, tenantCtx, itemRepo, "Ethanol")

	detail := &repository.HazardousSubstanceDetail{
		ItemID:             item.ID,
		GHSPictogramCodes: strPtr("GHS02"),
		HStatements:       strPtr("H225"),
		SignalWord:         strPtr("Danger"),
	}
	err := hazRepo.Upsert(tenantCtx, detail)
	require.NoError(t, err)

	// Retrieve
	retrieved, err := hazRepo.GetByItemID(tenantCtx, item.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, detail.ID, retrieved.ID)
	assert.Equal(t, item.ID, retrieved.ItemID)
	assert.Equal(t, "GHS02", *retrieved.GHSPictogramCodes)
	assert.Equal(t, "H225", *retrieved.HStatements)
	assert.Equal(t, "Danger", *retrieved.SignalWord)
}

func TestHazardousRepository_Upsert_Update(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-hazardous-upsert")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	hazRepo := repository.NewHazardousRepository(suite.DB)

	item := createTestItem(t, tenantCtx, itemRepo, "Acetone")

	// Initial create
	detail := &repository.HazardousSubstanceDetail{
		ItemID:    item.ID,
		SignalWord: strPtr("Warning"),
	}
	err := hazRepo.Upsert(tenantCtx, detail)
	require.NoError(t, err)
	originalID := detail.ID

	// Upsert again with different data (same item_id)
	detail2 := &repository.HazardousSubstanceDetail{
		ItemID:             item.ID,
		SignalWord:         strPtr("Danger"),
		GHSPictogramCodes: strPtr("GHS02,GHS07"),
	}
	err = hazRepo.Upsert(tenantCtx, detail2)
	require.NoError(t, err)

	// Should have reused the same row (upsert)
	assert.Equal(t, originalID, detail2.ID)

	// Verify updated values
	retrieved, err := hazRepo.GetByItemID(tenantCtx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, "Danger", *retrieved.SignalWord)
	assert.Equal(t, "GHS02,GHS07", *retrieved.GHSPictogramCodes)
}

func TestHazardousRepository_Delete(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-hazardous-delete")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	hazRepo := repository.NewHazardousRepository(suite.DB)

	item := createTestItem(t, tenantCtx, itemRepo, "Formaldehyde")

	detail := &repository.HazardousSubstanceDetail{
		ItemID:    item.ID,
		SignalWord: strPtr("Danger"),
	}
	err := hazRepo.Upsert(tenantCtx, detail)
	require.NoError(t, err)

	// Delete
	err = hazRepo.Delete(tenantCtx, item.ID)
	require.NoError(t, err)

	// Verify not found
	_, err = hazRepo.GetByItemID(tenantCtx, item.ID)
	assert.Error(t, err)
}

func TestHazardousRepository_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-hazardous-delete-nf")
	tenantCtx := suite.TenantContext(tenant)

	hazRepo := repository.NewHazardousRepository(suite.DB)

	// Delete nonexistent
	err := hazRepo.Delete(tenantCtx, "nonexistent-id")
	assert.Error(t, err)
}

func TestHazardousRepository_ListAll(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-hazardous-list")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	hazRepo := repository.NewHazardousRepository(suite.DB)

	// Create two items with hazardous details
	item1 := createTestItem(t, tenantCtx, itemRepo, "Chemical A")
	item2 := createTestItem(t, tenantCtx, itemRepo, "Chemical B")

	err := hazRepo.Upsert(tenantCtx, &repository.HazardousSubstanceDetail{
		ItemID:    item1.ID,
		SignalWord: strPtr("Danger"),
	})
	require.NoError(t, err)

	err = hazRepo.Upsert(tenantCtx, &repository.HazardousSubstanceDetail{
		ItemID:    item2.ID,
		SignalWord: strPtr("Warning"),
	})
	require.NoError(t, err)

	// List all
	details, err := hazRepo.ListAll(tenantCtx)
	require.NoError(t, err)
	assert.Len(t, details, 2)
}

func TestHazardousRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()

	tenant1 := suite.SetupInventoryTenant(t, ctx, "test-hazardous-iso-1")
	tenant2 := suite.SetupInventoryTenant(t, ctx, "test-hazardous-iso-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	itemRepo := repository.NewItemRepository(suite.DB)
	hazRepo := repository.NewHazardousRepository(suite.DB)

	// Create item + hazardous details in tenant 1
	item1 := createTestItem(t, ctx1, itemRepo, "Tenant1 Chemical")
	err := hazRepo.Upsert(ctx1, &repository.HazardousSubstanceDetail{
		ItemID:    item1.ID,
		SignalWord: strPtr("Danger"),
	})
	require.NoError(t, err)

	// Create item + hazardous details in tenant 2
	item2 := createTestItem(t, ctx2, itemRepo, "Tenant2 Chemical")
	err = hazRepo.Upsert(ctx2, &repository.HazardousSubstanceDetail{
		ItemID:    item2.ID,
		SignalWord: strPtr("Warning"),
	})
	require.NoError(t, err)

	// Tenant 1 should NOT see tenant 2's hazardous details
	_, err = hazRepo.GetByItemID(ctx1, item2.ID)
	assert.Error(t, err, "tenant1 should NOT see tenant2's hazardous details")

	// Tenant 2 should NOT see tenant 1's hazardous details
	_, err = hazRepo.GetByItemID(ctx2, item1.ID)
	assert.Error(t, err, "tenant2 should NOT see tenant1's hazardous details")

	// Each tenant should see only their own in ListAll
	list1, err := hazRepo.ListAll(ctx1)
	require.NoError(t, err)
	assert.Len(t, list1, 1)
	assert.Equal(t, item1.ID, list1[0].ItemID)

	list2, err := hazRepo.ListAll(ctx2)
	require.NoError(t, err)
	assert.Len(t, list2, 1)
	assert.Equal(t, item2.ID, list2[0].ItemID)
}

// --- Document Repository Tests ---

func TestDocumentRepository_Create(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-doc-create")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	docRepo := repository.NewDocumentRepository(suite.DB)

	item := createTestItem(t, tenantCtx, itemRepo, "Item with Docs")

	doc := &repository.ItemDocument{
		ItemID:       item.ID,
		DocumentType: "sdb",
		FileName:     "safety-data-sheet.pdf",
		FilePath:     "uploads/inventory/test/sdb.pdf",
		FileSizeBytes: intPtr(1024),
		MimeType:     strPtr("application/pdf"),
	}

	err := docRepo.Create(tenantCtx, doc)
	require.NoError(t, err)
	assert.NotEmpty(t, doc.ID)
	assert.False(t, doc.CreatedAt.IsZero())
	assert.False(t, doc.UploadedAt.IsZero())
}

func TestDocumentRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-doc-get")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	docRepo := repository.NewDocumentRepository(suite.DB)

	item := createTestItem(t, tenantCtx, itemRepo, "Item Docs Get")

	doc := &repository.ItemDocument{
		ItemID:       item.ID,
		DocumentType: "certificate",
		FileName:     "ce-cert.pdf",
		FilePath:     "uploads/inventory/test/cert.pdf",
		FileSizeBytes: intPtr(2048),
		MimeType:     strPtr("application/pdf"),
	}
	err := docRepo.Create(tenantCtx, doc)
	require.NoError(t, err)

	// Retrieve
	retrieved, err := docRepo.GetByID(tenantCtx, doc.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, doc.ID, retrieved.ID)
	assert.Equal(t, item.ID, retrieved.ItemID)
	assert.Equal(t, "certificate", retrieved.DocumentType)
	assert.Equal(t, "ce-cert.pdf", retrieved.FileName)
	assert.Equal(t, "uploads/inventory/test/cert.pdf", retrieved.FilePath)
}

func TestDocumentRepository_ListByItem(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-doc-list")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	docRepo := repository.NewDocumentRepository(suite.DB)

	item := createTestItem(t, tenantCtx, itemRepo, "Item Multiple Docs")

	// Create multiple documents
	docs := []*repository.ItemDocument{
		{ItemID: item.ID, DocumentType: "sdb", FileName: "sdb.pdf", FilePath: "uploads/sdb.pdf"},
		{ItemID: item.ID, DocumentType: "manual", FileName: "manual.pdf", FilePath: "uploads/manual.pdf"},
		{ItemID: item.ID, DocumentType: "certificate", FileName: "cert.pdf", FilePath: "uploads/cert.pdf"},
	}

	for _, d := range docs {
		err := docRepo.Create(tenantCtx, d)
		require.NoError(t, err)
	}

	// List
	results, err := docRepo.ListByItem(tenantCtx, item.ID)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestDocumentRepository_Delete(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-doc-delete")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	docRepo := repository.NewDocumentRepository(suite.DB)

	item := createTestItem(t, tenantCtx, itemRepo, "Item Delete Doc")

	doc := &repository.ItemDocument{
		ItemID:       item.ID,
		DocumentType: "sdb",
		FileName:     "to-delete.pdf",
		FilePath:     "uploads/delete.pdf",
	}
	err := docRepo.Create(tenantCtx, doc)
	require.NoError(t, err)

	// Delete
	err = docRepo.Delete(tenantCtx, doc.ID)
	require.NoError(t, err)

	// Verify not found
	_, err = docRepo.GetByID(tenantCtx, doc.ID)
	assert.Error(t, err)

	// List should be empty
	results, err := docRepo.ListByItem(tenantCtx, item.ID)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestDocumentRepository_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-doc-delete-nf")
	tenantCtx := suite.TenantContext(tenant)

	docRepo := repository.NewDocumentRepository(suite.DB)

	err := docRepo.Delete(tenantCtx, "nonexistent-id")
	assert.Error(t, err)
}

func TestDocumentRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()

	tenant1 := suite.SetupInventoryTenant(t, ctx, "test-doc-iso-1")
	tenant2 := suite.SetupInventoryTenant(t, ctx, "test-doc-iso-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	itemRepo := repository.NewItemRepository(suite.DB)
	docRepo := repository.NewDocumentRepository(suite.DB)

	// Create item + doc in tenant 1
	item1 := createTestItem(t, ctx1, itemRepo, "Tenant1 Item")
	doc1 := &repository.ItemDocument{
		ItemID:       item1.ID,
		DocumentType: "sdb",
		FileName:     "t1-sdb.pdf",
		FilePath:     "uploads/t1/sdb.pdf",
	}
	err := docRepo.Create(ctx1, doc1)
	require.NoError(t, err)

	// Create item + doc in tenant 2
	item2 := createTestItem(t, ctx2, itemRepo, "Tenant2 Item")
	doc2 := &repository.ItemDocument{
		ItemID:       item2.ID,
		DocumentType: "sdb",
		FileName:     "t2-sdb.pdf",
		FilePath:     "uploads/t2/sdb.pdf",
	}
	err = docRepo.Create(ctx2, doc2)
	require.NoError(t, err)

	// Tenant 1 should NOT see tenant 2's document
	_, err = docRepo.GetByID(ctx1, doc2.ID)
	assert.Error(t, err, "tenant1 should NOT see tenant2's document")

	// Tenant 2 should NOT see tenant 1's document
	_, err = docRepo.GetByID(ctx2, doc1.ID)
	assert.Error(t, err, "tenant2 should NOT see tenant1's document")

	// ListByItem should only show own tenant's docs
	list1, err := docRepo.ListByItem(ctx1, item1.ID)
	require.NoError(t, err)
	assert.Len(t, list1, 1)
	assert.Equal(t, doc1.ID, list1[0].ID)

	list2, err := docRepo.ListByItem(ctx2, item2.ID)
	require.NoError(t, err)
	assert.Len(t, list2, 1)
	assert.Equal(t, doc2.ID, list2[0].ID)
}

// --- Item Compliance Fields Tests ---

func TestItemRepository_ComplianceFields(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-item-compliance")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)

	// Create item with compliance fields
	item := &repository.InventoryItem{
		Name:                "Dental Implant",
		Category:            "Supplies",
		Unit:                "pieces",
		MinStock:            10,
		IsActive:            true,
		IsHazardous:         false,
		CEMarkingNumber:     strPtr("CE-12345"),
		NotifiedBodyID:      strPtr("NB-0123"),
		UdiDI:               strPtr("(01)04056789012345"),
		UdiPI:               strPtr("(17)261231(10)LOT123"),
		SerialNumber:        strPtr("SN-2026-001"),
		ManufacturerAddress: strPtr("MedTech GmbH, Berlin, Germany"),
	}

	err := itemRepo.Create(tenantCtx, item)
	require.NoError(t, err)
	assert.NotEmpty(t, item.ID)

	// Retrieve and verify compliance fields are persisted
	retrieved, err := itemRepo.GetByID(tenantCtx, item.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, "CE-12345", *retrieved.CEMarkingNumber)
	assert.Equal(t, "NB-0123", *retrieved.NotifiedBodyID)
	assert.Equal(t, "(01)04056789012345", *retrieved.UdiDI)
	assert.Equal(t, "(17)261231(10)LOT123", *retrieved.UdiPI)
	assert.Equal(t, "SN-2026-001", *retrieved.SerialNumber)
	assert.Equal(t, "MedTech GmbH, Berlin, Germany", *retrieved.ManufacturerAddress)
}

// --- Batch OpenedAt Field Test ---

func TestBatchRepository_OpenedAt(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-batch-opened")
	tenantCtx := suite.TenantContext(tenant)

	itemRepo := repository.NewItemRepository(suite.DB)
	batchRepo := repository.NewBatchRepository(suite.DB)

	// Create parent item
	item := &repository.InventoryItem{
		Name:     "Eye Drops",
		Category: "Medicine",
		Unit:     "bottles",
		MinStock: 5,
		IsActive: true,
	}
	err := itemRepo.Create(tenantCtx, item)
	require.NoError(t, err)

	// Create batch without openedAt
	batch := &repository.InventoryBatch{
		ItemID:          item.ID,
		BatchNumber:     "BATCH-001",
		InitialQuantity: 100,
		CurrentQuantity: 100,
		ReceivedDate:    time.Now().UTC().Truncate(time.Second),
	}
	err = batchRepo.Create(tenantCtx, batch)
	require.NoError(t, err)
	assert.Nil(t, batch.OpenedAt)

	// Retrieve and verify openedAt is nil
	retrieved, err := batchRepo.GetByID(tenantCtx, batch.ID)
	require.NoError(t, err)
	assert.Nil(t, retrieved.OpenedAt)

	// Update with openedAt
	now := time.Now().UTC().Truncate(time.Second)
	batch.OpenedAt = &now
	err = batchRepo.Update(tenantCtx, batch)
	require.NoError(t, err)

	// Verify openedAt was persisted
	updated, err := batchRepo.GetByID(tenantCtx, batch.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.OpenedAt)
	assert.Equal(t, now.Unix(), updated.OpenedAt.Unix())
}

// Helper
func intPtr(i int) *int {
	return &i
}
