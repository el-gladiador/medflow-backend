package repository_test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/staff/repository"
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
		log.Fatalf("failed to create integration suite: %v", err)
	}
	defer suite.Cleanup(ctx)
	defer testutil.TerminateContainer(ctx)

	os.Exit(m.Run())
}

func TestEmployeeRepository_Create(t *testing.T) {
	ctx := context.Background()

	// Setup a tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-create-employee")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create tenant context
	tenantCtx := suite.TenantContext(tenant)

	// Create a test employee
	now := time.Now().UTC().Truncate(time.Second)
	emp := &repository.Employee{
		FirstName:      "Max",
		LastName:       "Mustermann",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
	}

	// Test creation
	err := repo.Create(tenantCtx, emp)
	require.NoError(t, err)

	// Verify ID was assigned
	assert.NotEmpty(t, emp.ID)
	assert.Equal(t, "Max", emp.FirstName)
	assert.Equal(t, "Mustermann", emp.LastName)
	assert.Equal(t, "full_time", emp.EmploymentType)
	assert.Equal(t, "active", emp.Status)
}

func TestEmployeeRepository_GetByID(t *testing.T) {
	ctx := context.Background()

	// Setup a tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-get-employee")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create tenant context
	tenantCtx := suite.TenantContext(tenant)

	// Create a test employee first
	now := time.Now().UTC().Truncate(time.Second)
	emp := &repository.Employee{
		FirstName:      "Anna",
		LastName:       "Schmidt",
		EmploymentType: "part_time",
		HireDate:       now,
		Status:         "active",
		Email:          strPtr("anna.schmidt@example.com"),
	}
	err := repo.Create(tenantCtx, emp)
	require.NoError(t, err)

	// Test retrieval
	retrieved, err := repo.GetByID(tenantCtx, emp.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, emp.ID, retrieved.ID)
	assert.Equal(t, "Anna", retrieved.FirstName)
	assert.Equal(t, "Schmidt", retrieved.LastName)
	assert.Equal(t, "part_time", retrieved.EmploymentType)
}

func TestEmployeeRepository_List(t *testing.T) {
	ctx := context.Background()

	// Setup a tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-list-employees")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create tenant context
	tenantCtx := suite.TenantContext(tenant)

	now := time.Now().UTC().Truncate(time.Second)

	// Create multiple test employees
	employees := []*repository.Employee{
		{FirstName: "Hans", LastName: "Mueller", EmploymentType: "full_time", HireDate: now, Status: "active"},
		{FirstName: "Petra", LastName: "Weber", EmploymentType: "part_time", HireDate: now, Status: "active"},
		{FirstName: "Klaus", LastName: "Bauer", EmploymentType: "contractor", HireDate: now, Status: "on_leave"},
	}

	for _, emp := range employees {
		err := repo.Create(tenantCtx, emp)
		require.NoError(t, err)
	}

	// Test listing
	results, total, err := repo.List(tenantCtx, 1, 10)
	require.NoError(t, err)

	assert.Equal(t, int64(3), total)
	assert.Len(t, results, 3)
}

func TestEmployeeRepository_Update(t *testing.T) {
	ctx := context.Background()

	// Setup a tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-update-employee")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create tenant context
	tenantCtx := suite.TenantContext(tenant)

	// Create a test employee first
	now := time.Now().UTC().Truncate(time.Second)
	emp := &repository.Employee{
		FirstName:      "Original",
		LastName:       "Name",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
	}
	err := repo.Create(tenantCtx, emp)
	require.NoError(t, err)

	// Update the employee
	emp.FirstName = "Updated"
	emp.LastName = "Person"
	emp.Status = "on_leave"
	err = repo.Update(tenantCtx, emp)
	require.NoError(t, err)

	// Verify update
	updated, err := repo.GetByID(tenantCtx, emp.ID)
	require.NoError(t, err)

	assert.Equal(t, "Updated", updated.FirstName)
	assert.Equal(t, "Person", updated.LastName)
	assert.Equal(t, "on_leave", updated.Status)
}

func TestEmployeeRepository_SoftDelete(t *testing.T) {
	ctx := context.Background()

	// Setup a tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-delete-employee")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create tenant context
	tenantCtx := suite.TenantContext(tenant)

	// Create a test employee first
	now := time.Now().UTC().Truncate(time.Second)
	emp := &repository.Employee{
		FirstName:      "ToDelete",
		LastName:       "Employee",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
	}
	err := repo.Create(tenantCtx, emp)
	require.NoError(t, err)

	// Soft delete the employee
	err = repo.SoftDelete(tenantCtx, emp.ID)
	require.NoError(t, err)

	// Verify employee is not found (soft deleted)
	deleted, err := repo.GetByID(tenantCtx, emp.ID)
	assert.Error(t, err)
	assert.Nil(t, deleted)
}

func TestEmployeeRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()

	// Setup two separate tenants
	tenant1 := suite.SetupStaffTenant(t, ctx, "test-tenant-isolation-1")
	tenant2 := suite.SetupStaffTenant(t, ctx, "test-tenant-isolation-2")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create contexts for each tenant
	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	now := time.Now().UTC().Truncate(time.Second)

	// Create employee in tenant 1
	emp1 := &repository.Employee{
		FirstName:      "Tenant1",
		LastName:       "Employee",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
	}
	err := repo.Create(ctx1, emp1)
	require.NoError(t, err)

	// Create employee in tenant 2
	emp2 := &repository.Employee{
		FirstName:      "Tenant2",
		LastName:       "Employee",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
	}
	err = repo.Create(ctx2, emp2)
	require.NoError(t, err)

	// Verify tenant 1 only sees its employee
	results1, total1, err := repo.List(ctx1, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total1)
	assert.Len(t, results1, 1)
	assert.Equal(t, "Tenant1", results1[0].FirstName)

	// Verify tenant 2 only sees its employee
	results2, total2, err := repo.List(ctx2, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total2)
	assert.Len(t, results2, 1)
	assert.Equal(t, "Tenant2", results2[0].FirstName)

	// Verify tenant 1 cannot access tenant 2's employee
	notFound, err := repo.GetByID(ctx1, emp2.ID)
	assert.Error(t, err)
	assert.Nil(t, notFound)

	// Verify tenant 2 cannot access tenant 1's employee
	notFound, err = repo.GetByID(ctx2, emp1.ID)
	assert.Error(t, err)
	assert.Nil(t, notFound)
}

func TestEmployeeRepository_Address(t *testing.T) {
	ctx := context.Background()

	// Setup a tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-employee-address")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create tenant context
	tenantCtx := suite.TenantContext(tenant)

	now := time.Now().UTC().Truncate(time.Second)

	// Create an employee first
	emp := &repository.Employee{
		FirstName:      "Test",
		LastName:       "Address",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
	}
	err := repo.Create(tenantCtx, emp)
	require.NoError(t, err)

	// Create an address for the employee
	addr := &repository.EmployeeAddress{
		EmployeeID:  emp.ID,
		AddressType: "home",
		Street:      "Hauptstrasse",
		HouseNumber: strPtr("123"),
		PostalCode:  "12345",
		City:        "Berlin",
		Country:     "Germany",
		IsPrimary:   true,
	}
	err = repo.SaveAddress(tenantCtx, addr)
	require.NoError(t, err)

	// Retrieve the address
	retrieved, err := repo.GetAddress(tenantCtx, emp.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, "Hauptstrasse", retrieved.Street)
	assert.Equal(t, "12345", retrieved.PostalCode)
	assert.Equal(t, "Berlin", retrieved.City)
}

func strPtr(s string) *string {
	return &s
}

// ============================================================================
// CREDENTIAL MANAGEMENT TESTS
// ============================================================================

func TestEmployeeRepository_UpdateUserID(t *testing.T) {
	ctx := context.Background()

	// Setup a tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-update-user-id")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create tenant context
	tenantCtx := suite.TenantContext(tenant)

	now := time.Now().UTC().Truncate(time.Second)

	// Create an employee without user_id
	emp := &repository.Employee{
		FirstName:      "TestUser",
		LastName:       "LinkEmployee",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
		Email:          strPtr("test@example.com"),
	}
	err := repo.Create(tenantCtx, emp)
	require.NoError(t, err)
	assert.Nil(t, emp.UserID) // Initially no user_id

	// Update user_id (must be valid UUID)
	userID := uuid.New().String()
	err = repo.UpdateUserID(tenantCtx, emp.ID, userID)
	require.NoError(t, err)

	// Verify user_id was set
	updated, err := repo.GetByID(tenantCtx, emp.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.UserID)
	assert.Equal(t, userID, *updated.UserID)
}

func TestEmployeeRepository_UpdateUserID_AlreadyHasUserID(t *testing.T) {
	ctx := context.Background()

	// Setup a tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-update-user-id-conflict")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create tenant context
	tenantCtx := suite.TenantContext(tenant)

	now := time.Now().UTC().Truncate(time.Second)
	existingUserID := uuid.New().String()

	// Create an employee WITH user_id already set
	emp := &repository.Employee{
		FirstName:      "HasUser",
		LastName:       "Already",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
		Email:          strPtr("hasuser@example.com"),
		UserID:         &existingUserID,
	}
	err := repo.Create(tenantCtx, emp)
	require.NoError(t, err)

	// Try to update user_id - should fail with conflict error
	newUserID := uuid.New().String()
	err = repo.UpdateUserID(tenantCtx, emp.ID, newUserID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already has user credentials")

	// Verify original user_id is unchanged
	retrieved, err := repo.GetByID(tenantCtx, emp.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.UserID)
	assert.Equal(t, existingUserID, *retrieved.UserID)
}

func TestEmployeeRepository_UpdateUserID_NotFound(t *testing.T) {
	ctx := context.Background()

	// Setup a tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-update-user-id-notfound")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create tenant context
	tenantCtx := suite.TenantContext(tenant)

	// Try to update user_id for non-existent employee (use valid UUID format)
	nonExistentID := uuid.New().String()
	userID := uuid.New().String()
	err := repo.UpdateUserID(tenantCtx, nonExistentID, userID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEmployeeRepository_ClearUserID(t *testing.T) {
	ctx := context.Background()

	// Setup a tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-clear-user-id")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create tenant context
	tenantCtx := suite.TenantContext(tenant)

	now := time.Now().UTC().Truncate(time.Second)
	userID := uuid.New().String()

	// Create an employee with user_id
	emp := &repository.Employee{
		FirstName:      "Clear",
		LastName:       "UserID",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
		Email:          strPtr("clear@example.com"),
		UserID:         &userID,
	}
	err := repo.Create(tenantCtx, emp)
	require.NoError(t, err)
	require.NotNil(t, emp.UserID)

	// Clear user_id
	err = repo.ClearUserID(tenantCtx, emp.ID)
	require.NoError(t, err)

	// Verify user_id was cleared
	updated, err := repo.GetByID(tenantCtx, emp.ID)
	require.NoError(t, err)
	assert.Nil(t, updated.UserID)
}

func TestEmployeeRepository_ClearUserID_NotFound(t *testing.T) {
	ctx := context.Background()

	// Setup a tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-clear-user-id-notfound")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create tenant context
	tenantCtx := suite.TenantContext(tenant)

	// Try to clear user_id for non-existent employee (use valid UUID format)
	nonExistentID := uuid.New().String()
	err := repo.ClearUserID(tenantCtx, nonExistentID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEmployeeRepository_UpdateUserID_TenantIsolation(t *testing.T) {
	ctx := context.Background()

	// Setup two separate tenants
	tenant1 := suite.SetupStaffTenant(t, ctx, "test-userid-isolation-1")
	tenant2 := suite.SetupStaffTenant(t, ctx, "test-userid-isolation-2")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create contexts for each tenant
	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	now := time.Now().UTC().Truncate(time.Second)

	// Create employee in tenant 1
	emp1 := &repository.Employee{
		FirstName:      "Tenant1",
		LastName:       "Credentials",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
		Email:          strPtr("tenant1@example.com"),
	}
	err := repo.Create(ctx1, emp1)
	require.NoError(t, err)

	// Create employee in tenant 2
	emp2 := &repository.Employee{
		FirstName:      "Tenant2",
		LastName:       "Credentials",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
		Email:          strPtr("tenant2@example.com"),
	}
	err = repo.Create(ctx2, emp2)
	require.NoError(t, err)

	maliciousUserID := uuid.New().String()
	legitimateUserID := uuid.New().String()

	// CRITICAL: Try to update tenant1's employee user_id from tenant2 context
	// This MUST fail - cross-tenant access should not be allowed
	err = repo.UpdateUserID(ctx2, emp1.ID, maliciousUserID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify tenant1's employee is unchanged
	retrieved, err := repo.GetByID(ctx1, emp1.ID)
	require.NoError(t, err)
	assert.Nil(t, retrieved.UserID) // Should still be nil

	// Now properly update tenant1's employee from tenant1 context
	err = repo.UpdateUserID(ctx1, emp1.ID, legitimateUserID)
	require.NoError(t, err)

	// CRITICAL: Try to clear tenant1's employee user_id from tenant2 context
	// This MUST fail
	err = repo.ClearUserID(ctx2, emp1.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify tenant1's employee user_id is still set
	retrieved, err = repo.GetByID(ctx1, emp1.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.UserID)
	assert.Equal(t, legitimateUserID, *retrieved.UserID)
}

func TestEmployeeRepository_GetByUserID(t *testing.T) {
	ctx := context.Background()

	// Setup a tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-get-by-user-id")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create tenant context
	tenantCtx := suite.TenantContext(tenant)

	now := time.Now().UTC().Truncate(time.Second)
	userID := uuid.New().String()

	// Create an employee with user_id
	emp := &repository.Employee{
		FirstName:      "UserLookup",
		LastName:       "Test",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
		Email:          strPtr("userlookup@example.com"),
		UserID:         &userID,
	}
	err := repo.Create(tenantCtx, emp)
	require.NoError(t, err)

	// Lookup by user_id
	retrieved, err := repo.GetByUserID(tenantCtx, userID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, emp.ID, retrieved.ID)
	assert.Equal(t, "UserLookup", retrieved.FirstName)
}

func TestEmployeeRepository_GetByUserID_TenantIsolation(t *testing.T) {
	ctx := context.Background()

	// Setup two separate tenants
	tenant1 := suite.SetupStaffTenant(t, ctx, "test-getbyuserid-isolation-1")
	tenant2 := suite.SetupStaffTenant(t, ctx, "test-getbyuserid-isolation-2")

	// Create repository with the suite's DB
	repo := repository.NewEmployeeRepository(suite.DB)

	// Create contexts for each tenant
	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	now := time.Now().UTC().Truncate(time.Second)
	userID := uuid.New().String()

	// Create employee with user_id in tenant 1
	emp1 := &repository.Employee{
		FirstName:      "Tenant1",
		LastName:       "UserLookup",
		EmploymentType: "full_time",
		HireDate:       now,
		Status:         "active",
		Email:          strPtr("tenant1lookup@example.com"),
		UserID:         &userID,
	}
	err := repo.Create(ctx1, emp1)
	require.NoError(t, err)

	// CRITICAL: Tenant 2 should NOT be able to find tenant 1's employee by user_id
	notFound, err := repo.GetByUserID(ctx2, userID)
	require.Error(t, err)
	assert.Nil(t, notFound)

	// Tenant 1 should find their own employee
	found, err := repo.GetByUserID(ctx1, userID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, emp1.ID, found.ID)
}
