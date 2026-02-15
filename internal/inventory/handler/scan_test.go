package handler_test

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/inventory/handler"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
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
		nil, // no event publisher needed for handler tests
		log,
	)
}

func newTestScanHandler() *handler.ScanHandler {
	svc := newTestService()
	log := logger.New("test", "test")
	return handler.NewScanHandler(svc, log)
}

func strPtr(s string) *string {
	return &s
}

// createTestItem is a helper to create an inventory item for tests.
func createTestItem(t *testing.T, tenantCtx context.Context, name string) *repository.InventoryItem {
	t.Helper()
	itemRepo := repository.NewItemRepository(suite.DB)
	item := &repository.InventoryItem{
		Name:     name,
		Category: "Supplies",
		Unit:     "pieces",
		MinStock: 5,
		IsActive: true,
	}
	err := itemRepo.Create(tenantCtx, item)
	require.NoError(t, err)
	return item
}

// --- LookupByBarcode Tests ---

func TestLookupByBarcode_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "handler-barcode-ok")
	tenantCtx := suite.TenantContext(tenant)

	// Create item with barcode
	itemRepo := repository.NewItemRepository(suite.DB)
	barcode := "5901234123457"
	item := &repository.InventoryItem{
		Name:     "Barcode Handler Item",
		Category: "Supplies",
		Unit:     "pieces",
		MinStock: 5,
		IsActive: true,
		Barcode:  &barcode,
	}
	err := itemRepo.Create(tenantCtx, item)
	require.NoError(t, err)

	// Set up chi router with URL param
	h := newTestScanHandler()
	r := chi.NewRouter()
	r.Use(httputil.TenantMiddleware)
	r.Get("/api/v1/scan/barcode/{barcode}", h.LookupByBarcode)

	// Create request
	req := httptest.NewRequest("GET", "/api/v1/scan/barcode/5901234123457", nil)
	req.Header.Set("X-Tenant-ID", tenant.ID)
	req.Header.Set("X-Tenant-Slug", tenant.Slug)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rr.Code, "unexpected status code. Body: %s", rr.Body.String())

	var resp httputil.Response
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Data)
}

func TestLookupByBarcode_FallbackToArticleNumber(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "handler-barcode-fallback")
	tenantCtx := suite.TenantContext(tenant)

	// Create item with article number (no barcode)
	itemRepo := repository.NewItemRepository(suite.DB)
	articleNum := "ART-FALLBACK-001"
	item := &repository.InventoryItem{
		Name:          "Article Number Fallback Item",
		Category:      "Supplies",
		Unit:          "pieces",
		MinStock:      5,
		IsActive:      true,
		ArticleNumber: &articleNum,
	}
	err := itemRepo.Create(tenantCtx, item)
	require.NoError(t, err)

	// Look up using article number via the barcode endpoint (fallback behavior)
	h := newTestScanHandler()
	r := chi.NewRouter()
	r.Use(httputil.TenantMiddleware)
	r.Get("/api/v1/scan/barcode/{barcode}", h.LookupByBarcode)

	req := httptest.NewRequest("GET", "/api/v1/scan/barcode/ART-FALLBACK-001", nil)
	req.Header.Set("X-Tenant-ID", tenant.ID)
	req.Header.Set("X-Tenant-Slug", tenant.Slug)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "unexpected status code. Body: %s", rr.Body.String())

	var resp httputil.Response
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestLookupByBarcode_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "handler-barcode-nf")

	h := newTestScanHandler()
	r := chi.NewRouter()
	r.Use(httputil.TenantMiddleware)
	r.Get("/api/v1/scan/barcode/{barcode}", h.LookupByBarcode)

	req := httptest.NewRequest("GET", "/api/v1/scan/barcode/NONEXISTENT-BARCODE", nil)
	req.Header.Set("X-Tenant-ID", tenant.ID)
	req.Header.Set("X-Tenant-Slug", tenant.Slug)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "expected 404 for non-existent barcode. Body: %s", rr.Body.String())

	var resp httputil.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "NOT_FOUND", resp.Error.Code)
}

func TestLookupByBarcode_FallbackToPZN(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "handler-pzn-fallback")
	tenantCtx := suite.TenantContext(tenant)

	// Create item with PZN (no barcode, no article number)
	itemRepo := repository.NewItemRepository(suite.DB)
	pzn := "08585997"
	item := &repository.InventoryItem{
		Name:     "PZN Fallback Item",
		Category: "Medication",
		Unit:     "pieces",
		MinStock: 5,
		IsActive: true,
		PZN:      &pzn,
	}
	err := itemRepo.Create(tenantCtx, item)
	require.NoError(t, err)

	h := newTestScanHandler()
	r := chi.NewRouter()
	r.Use(httputil.TenantMiddleware)
	r.Get("/api/v1/scan/barcode/{barcode}", h.LookupByBarcode)

	// Look up by bare PZN
	req := httptest.NewRequest("GET", "/api/v1/scan/barcode/08585997", nil)
	req.Header.Set("X-Tenant-ID", tenant.ID)
	req.Header.Set("X-Tenant-Slug", tenant.Slug)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "should find item by PZN fallback. Body: %s", rr.Body.String())

	var resp httputil.Response
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestLookupByBarcode_PZNWithPrefix(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "handler-pzn-prefix")
	tenantCtx := suite.TenantContext(tenant)

	// Create item with PZN
	itemRepo := repository.NewItemRepository(suite.DB)
	pzn := "12345678"
	item := &repository.InventoryItem{
		Name:     "PZN Prefix Item",
		Category: "Medication",
		Unit:     "pieces",
		MinStock: 5,
		IsActive: true,
		PZN:      &pzn,
	}
	err := itemRepo.Create(tenantCtx, item)
	require.NoError(t, err)

	h := newTestScanHandler()
	r := chi.NewRouter()
	r.Use(httputil.TenantMiddleware)
	r.Get("/api/v1/scan/barcode/{barcode}", h.LookupByBarcode)

	// Look up by PZN with "PZN-" prefix
	req := httptest.NewRequest("GET", "/api/v1/scan/barcode/PZN-12345678", nil)
	req.Header.Set("X-Tenant-ID", tenant.ID)
	req.Header.Set("X-Tenant-Slug", tenant.Slug)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "should find item by PZN with prefix. Body: %s", rr.Body.String())

	var resp httputil.Response
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

// --- LookupByBatchNumber Tests ---

func TestLookupByBatchNumber_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "handler-batch-ok")
	tenantCtx := suite.TenantContext(tenant)

	// Create item and batch
	item := createTestItem(t, tenantCtx, "Batch Handler Item")
	batchRepo := repository.NewBatchRepository(suite.DB)
	batch := &repository.InventoryBatch{
		ItemID:          item.ID,
		BatchNumber:     "HANDLER-BATCH-001",
		InitialQuantity: 100,
		CurrentQuantity: 100,
		ReceivedDate:    time.Now().UTC().Truncate(time.Second),
	}
	err := batchRepo.Create(tenantCtx, batch)
	require.NoError(t, err)

	// Set up handler route
	h := newTestScanHandler()
	r := chi.NewRouter()
	r.Use(httputil.TenantMiddleware)
	r.Get("/api/v1/scan/batch", h.LookupByBatchNumber)

	req := httptest.NewRequest("GET", "/api/v1/scan/batch?batchNumber=HANDLER-BATCH-001", nil)
	req.Header.Set("X-Tenant-ID", tenant.ID)
	req.Header.Set("X-Tenant-Slug", tenant.Slug)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "unexpected status code. Body: %s", rr.Body.String())

	var resp httputil.Response
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Data)
}

func TestLookupByBatchNumber_MissingParam(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "handler-batch-missing")

	h := newTestScanHandler()
	r := chi.NewRouter()
	r.Use(httputil.TenantMiddleware)
	r.Get("/api/v1/scan/batch", h.LookupByBatchNumber)

	// Request without batchNumber query parameter
	req := httptest.NewRequest("GET", "/api/v1/scan/batch", nil)
	req.Header.Set("X-Tenant-ID", tenant.ID)
	req.Header.Set("X-Tenant-Slug", tenant.Slug)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected 400 when batchNumber missing. Body: %s", rr.Body.String())

	var resp httputil.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "BAD_REQUEST", resp.Error.Code)
}

func TestLookupByBatchNumber_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "handler-batch-nf")

	h := newTestScanHandler()
	r := chi.NewRouter()
	r.Use(httputil.TenantMiddleware)
	r.Get("/api/v1/scan/batch", h.LookupByBatchNumber)

	req := httptest.NewRequest("GET", "/api/v1/scan/batch?batchNumber=NONEXISTENT-BATCH", nil)
	req.Header.Set("X-Tenant-ID", tenant.ID)
	req.Header.Set("X-Tenant-Slug", tenant.Slug)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "expected 404 for non-existent batch. Body: %s", rr.Body.String())

	var resp httputil.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "NOT_FOUND", resp.Error.Code)
}

func TestLookupByBatchNumber_EmptyParam(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	tenant := suite.SetupInventoryTenant(t, ctx, "handler-batch-empty")

	h := newTestScanHandler()
	r := chi.NewRouter()
	r.Use(httputil.TenantMiddleware)
	r.Get("/api/v1/scan/batch", h.LookupByBatchNumber)

	// Request with empty batchNumber query parameter
	req := httptest.NewRequest("GET", "/api/v1/scan/batch?batchNumber=", nil)
	req.Header.Set("X-Tenant-ID", tenant.ID)
	req.Header.Set("X-Tenant-Slug", tenant.Slug)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "expected 400 when batchNumber is empty. Body: %s", rr.Body.String())

	var resp httputil.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "BAD_REQUEST", resp.Error.Code)
}

// --- Tenant Isolation (Handler-level) ---

func TestLookupByBarcode_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()

	tenant1 := suite.SetupInventoryTenant(t, ctx, "handler-scan-iso-1")
	tenant2 := suite.SetupInventoryTenant(t, ctx, "handler-scan-iso-2")

	ctx1 := suite.TenantContext(tenant1)

	// Create item with barcode in tenant 1
	itemRepo := repository.NewItemRepository(suite.DB)
	barcode := "1111111111111"
	item := &repository.InventoryItem{
		Name:     "Tenant1 Handler Item",
		Category: "Supplies",
		Unit:     "pieces",
		MinStock: 5,
		IsActive: true,
		Barcode:  &barcode,
	}
	err := itemRepo.Create(ctx1, item)
	require.NoError(t, err)

	h := newTestScanHandler()
	r := chi.NewRouter()
	r.Use(httputil.TenantMiddleware)
	r.Get("/api/v1/scan/barcode/{barcode}", h.LookupByBarcode)

	// Tenant 1 should find the item
	req1 := httptest.NewRequest("GET", "/api/v1/scan/barcode/1111111111111", nil)
	req1.Header.Set("X-Tenant-ID", tenant1.ID)
	req1.Header.Set("X-Tenant-Slug", tenant1.Slug)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code, "tenant1 should find its own item. Body: %s", rr1.Body.String())

	// Tenant 2 should NOT find tenant 1's item (RLS blocks it)
	req2 := httptest.NewRequest("GET", "/api/v1/scan/barcode/1111111111111", nil)
	req2.Header.Set("X-Tenant-ID", tenant2.ID)
	req2.Header.Set("X-Tenant-Slug", tenant2.Slug)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusNotFound, rr2.Code, "tenant2 should NOT see tenant1's item. Body: %s", rr2.Body.String())
}

func TestLookupByBatchNumber_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()

	tenant1 := suite.SetupInventoryTenant(t, ctx, "handler-batch-iso-1")
	tenant2 := suite.SetupInventoryTenant(t, ctx, "handler-batch-iso-2")

	ctx1 := suite.TenantContext(tenant1)

	// Create item + batch in tenant 1
	item := createTestItem(t, ctx1, "Tenant1 Batch ISO Item")
	batchRepo := repository.NewBatchRepository(suite.DB)
	batch := &repository.InventoryBatch{
		ItemID:          item.ID,
		BatchNumber:     "ISO-HANDLER-BATCH-T1",
		InitialQuantity: 50,
		CurrentQuantity: 50,
		ReceivedDate:    time.Now().UTC().Truncate(time.Second),
	}
	err := batchRepo.Create(ctx1, batch)
	require.NoError(t, err)

	h := newTestScanHandler()
	r := chi.NewRouter()
	r.Use(httputil.TenantMiddleware)
	r.Get("/api/v1/scan/batch", h.LookupByBatchNumber)

	// Tenant 1 should find the batch
	req1 := httptest.NewRequest("GET", "/api/v1/scan/batch?batchNumber=ISO-HANDLER-BATCH-T1", nil)
	req1.Header.Set("X-Tenant-ID", tenant1.ID)
	req1.Header.Set("X-Tenant-Slug", tenant1.Slug)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code, "tenant1 should find its own batch. Body: %s", rr1.Body.String())

	// Tenant 2 should NOT find tenant 1's batch
	req2 := httptest.NewRequest("GET", "/api/v1/scan/batch?batchNumber=ISO-HANDLER-BATCH-T1", nil)
	req2.Header.Set("X-Tenant-ID", tenant2.ID)
	req2.Header.Set("X-Tenant-Slug", tenant2.Slug)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusNotFound, rr2.Code, "tenant2 should NOT see tenant1's batch. Body: %s", rr2.Body.String())
}
