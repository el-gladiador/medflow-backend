package service_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/logger"
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

func newTestService() *service.InventoryService {
	locationRepo := repository.NewLocationRepository(suite.DB)
	itemRepo := repository.NewItemRepository(suite.DB)
	batchRepo := repository.NewBatchRepository(suite.DB)
	alertRepo := repository.NewAlertRepository(suite.DB)
	hazardousRepo := repository.NewHazardousRepository(suite.DB)
	documentRepo := repository.NewDocumentRepository(suite.DB)
	temperatureRepo := repository.NewTemperatureRepository(suite.DB)
	inspectionRepo := repository.NewInspectionRepository(suite.DB)
	trainingRepo := repository.NewTrainingRepository(suite.DB)
	incidentRepo := repository.NewIncidentRepository(suite.DB)
	log := logger.New("test", "test")

	return service.NewInventoryService(
		locationRepo, itemRepo, batchRepo, alertRepo,
		hazardousRepo, documentRepo, temperatureRepo,
		inspectionRepo, trainingRepo, incidentRepo,
		nil, // no event publisher needed for export tests
		log,
	)
}

func strPtr(s string) *string {
	return &s
}

func TestExportInventoryRegister_Empty(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-export-reg-empty")
	tenantCtx := suite.TenantContext(tenant)

	svc := newTestService()

	// Export with no items should still produce valid PDF
	pdfBytes, err := svc.ExportInventoryRegister(tenantCtx)
	require.NoError(t, err)
	require.NotEmpty(t, pdfBytes)

	// Verify PDF header
	assert.True(t, len(pdfBytes) >= 4)
	assert.Equal(t, "%PDF", string(pdfBytes[:4]))
}

func TestExportInventoryRegister_WithItems(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-export-reg-items")
	tenantCtx := suite.TenantContext(tenant)

	svc := newTestService()

	// Create some items
	itemRepo := repository.NewItemRepository(suite.DB)
	items := []*repository.InventoryItem{
		{
			Name:            "Dental Supplies",
			Category:        "Supplies",
			Unit:            "boxes",
			MinStock:        10,
			IsActive:        true,
			CEMarkingNumber: strPtr("CE-001"),
		},
		{
			Name:        "Gloves",
			Category:    "Supplies",
			Unit:        "pairs",
			MinStock:    50,
			IsActive:    true,
			IsHazardous: false,
		},
	}
	for _, item := range items {
		err := itemRepo.Create(tenantCtx, item)
		require.NoError(t, err)
	}

	// Export
	pdfBytes, err := svc.ExportInventoryRegister(tenantCtx)
	require.NoError(t, err)
	require.NotEmpty(t, pdfBytes)
	assert.Equal(t, "%PDF", string(pdfBytes[:4]))

	// PDF with items should be larger than empty
	assert.Greater(t, len(pdfBytes), 100)
}

func TestExportGefahrstoffverzeichnis_Empty(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-export-gef-empty")
	tenantCtx := suite.TenantContext(tenant)

	svc := newTestService()

	// Export with no hazardous items
	pdfBytes, err := svc.ExportGefahrstoffverzeichnis(tenantCtx)
	require.NoError(t, err)
	require.NotEmpty(t, pdfBytes)
	assert.Equal(t, "%PDF", string(pdfBytes[:4]))
}

func TestExportGefahrstoffverzeichnis_WithData(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "test-export-gef-data")
	tenantCtx := suite.TenantContext(tenant)

	svc := newTestService()

	// Create a hazardous item
	itemRepo := repository.NewItemRepository(suite.DB)
	item := &repository.InventoryItem{
		Name:        "Isopropanol",
		Category:    "Supplies",
		Unit:        "liters",
		MinStock:    5,
		IsActive:    true,
		IsHazardous: true,
	}
	err := itemRepo.Create(tenantCtx, item)
	require.NoError(t, err)

	// Add hazardous details
	hazRepo := repository.NewHazardousRepository(suite.DB)
	detail := &repository.HazardousSubstanceDetail{
		ItemID:             item.ID,
		GHSPictogramCodes: strPtr("GHS02,GHS07"),
		HStatements:       strPtr("H225,H319"),
		PStatements:       strPtr("P210,P233"),
		SignalWord:         strPtr("Danger"),
		UsageArea:          strPtr("Surface disinfection"),
	}
	err = hazRepo.Upsert(tenantCtx, detail)
	require.NoError(t, err)

	// Export
	pdfBytes, err := svc.ExportGefahrstoffverzeichnis(tenantCtx)
	require.NoError(t, err)
	require.NotEmpty(t, pdfBytes)
	assert.Equal(t, "%PDF", string(pdfBytes[:4]))
	assert.Greater(t, len(pdfBytes), 100)
}

func TestExportTenantIsolation(t *testing.T) {
	ctx := context.Background()

	tenant1 := suite.SetupInventoryTenant(t, ctx, "test-export-iso-1")
	tenant2 := suite.SetupInventoryTenant(t, ctx, "test-export-iso-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	svc := newTestService()
	itemRepo := repository.NewItemRepository(suite.DB)

	// Create items in tenant 1 only
	for i := 0; i < 3; i++ {
		err := itemRepo.Create(ctx1, &repository.InventoryItem{
			Name:     "Tenant1 Item",
			Category: "Supplies",
			Unit:     "pieces",
			MinStock: 1,
			IsActive: true,
		})
		require.NoError(t, err)
	}

	// Tenant 2 export should produce minimal PDF (no items)
	pdf2, err := svc.ExportInventoryRegister(ctx2)
	require.NoError(t, err)
	assert.Equal(t, "%PDF", string(pdf2[:4]))

	// Tenant 1 export should have items
	pdf1, err := svc.ExportInventoryRegister(ctx1)
	require.NoError(t, err)
	assert.Equal(t, "%PDF", string(pdf1[:4]))

	// Tenant 1's PDF should be larger (contains items)
	assert.Greater(t, len(pdf1), len(pdf2))
}
