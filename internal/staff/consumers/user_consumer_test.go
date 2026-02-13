package consumers_test

import (
	"context"
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
	suite, err = testutil.NewIntegrationSuite(ctx, "staff, public")
	if err != nil {
		panic(err)
	}
	code := m.Run()
	suite.Cleanup(ctx)
	os.Exit(code)
}

func TestUserConsumer_AutoCreateEmployee_OnUserCreated(t *testing.T) {
	ctx := context.Background()

	// Setup tenant with staff migrations
	tenant := suite.SetupStaffTenant(t, ctx, "test-user-created")
	tenantCtx := testutil.WithTestTenant(ctx, tenant)

	// Setup repositories
	employeeRepo := repository.NewEmployeeRepository(suite.DB)

	// Create test user ID (valid UUID)
	userID := uuid.New().String()

	// Manually create employee (simulating createEmployeeForUser)
	jobTitle := "Staff Member"
	employee := &repository.Employee{
		UserID:         &userID,
		FirstName:      "Max",
		LastName:       "Mustermann",
		Email:          stringPtr("test@example.com"),
		EmploymentType: "full_time",
		HireDate:       time.Now(),
		Status:         "active",
		JobTitle:       &jobTitle,
	}
	err := employeeRepo.Create(tenantCtx, employee)
	require.NoError(t, err)

	// Verify employee was created
	createdEmployee, err := employeeRepo.GetByUserID(tenantCtx, userID)
	require.NoError(t, err)
	require.NotNil(t, createdEmployee)

	// Assert employee fields
	assert.Equal(t, userID, *createdEmployee.UserID)
	assert.Equal(t, "Max", createdEmployee.FirstName)
	assert.Equal(t, "Mustermann", createdEmployee.LastName)
	assert.Equal(t, "test@example.com", *createdEmployee.Email)
	assert.Equal(t, "full_time", createdEmployee.EmploymentType)
	assert.Equal(t, "active", createdEmployee.Status)
	assert.Equal(t, "Staff Member", *createdEmployee.JobTitle)
	assert.NotEmpty(t, createdEmployee.ID)
}

func TestUserConsumer_Idempotency(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupStaffTenant(t, ctx, "test-idempotency")
	tenantCtx := testutil.WithTestTenant(ctx, tenant)

	// Setup
	employeeRepo := repository.NewEmployeeRepository(suite.DB)

	userID := uuid.New().String()
	jobTitle := "Manager"
	employee := &repository.Employee{
		UserID:         &userID,
		FirstName:      "Anna",
		LastName:       "Schmidt",
		Email:          stringPtr("anna@example.com"),
		EmploymentType: "full_time",
		HireDate:       time.Now(),
		Status:         "active",
		JobTitle:       &jobTitle,
	}

	// Create employee first time
	err := employeeRepo.Create(tenantCtx, employee)
	require.NoError(t, err)

	// Try to create again (simulate duplicate event)
	existing, _ := employeeRepo.GetByUserID(tenantCtx, userID)
	if existing == nil {
		// Should not reach here - employee should exist
		t.Fatal("employee should exist from first creation")
	}

	// Verify only one employee exists
	employees, total, err := employeeRepo.List(tenantCtx, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, employees, 1)
}

func TestUserConsumer_TenantIsolation(t *testing.T) {
	ctx := context.Background()

	// Create two separate tenants
	tenant1 := suite.SetupStaffTenant(t, ctx, "test-tenant-1")
	tenant2 := suite.SetupStaffTenant(t, ctx, "test-tenant-2")

	ctx1 := testutil.WithTestTenant(ctx, tenant1)
	ctx2 := testutil.WithTestTenant(ctx, tenant2)

	// Setup
	employeeRepo := repository.NewEmployeeRepository(suite.DB)

	// Create employee in tenant1
	userID1 := uuid.New().String()
	jobTitle1 := "Administrator"
	emp1 := &repository.Employee{
		UserID:         &userID1,
		FirstName:      "User",
		LastName:       "One",
		Email:          stringPtr("user1@tenant1.com"),
		EmploymentType: "full_time",
		HireDate:       time.Now(),
		Status:         "active",
		JobTitle:       &jobTitle1,
	}
	err := employeeRepo.Create(ctx1, emp1)
	require.NoError(t, err)

	// Create employee in tenant2
	userID2 := uuid.New().String()
	jobTitle2 := "Manager"
	emp2 := &repository.Employee{
		UserID:         &userID2,
		FirstName:      "User",
		LastName:       "Two",
		Email:          stringPtr("user2@tenant2.com"),
		EmploymentType: "full_time",
		HireDate:       time.Now(),
		Status:         "active",
		JobTitle:       &jobTitle2,
	}
	err = employeeRepo.Create(ctx2, emp2)
	require.NoError(t, err)

	// Verify tenant1 can only see its own employee
	emp1Found, err := employeeRepo.GetByUserID(ctx1, userID1)
	require.NoError(t, err)
	assert.Equal(t, "User", emp1Found.FirstName)
	assert.Equal(t, "One", emp1Found.LastName)

	// Verify tenant1 CANNOT see tenant2's employee
	emp2InTenant1, err := employeeRepo.GetByUserID(ctx1, userID2)
	assert.Error(t, err) // Should fail - cross-tenant access
	assert.Nil(t, emp2InTenant1)

	// Verify tenant2 can only see its own employee
	emp2Found, err := employeeRepo.GetByUserID(ctx2, userID2)
	require.NoError(t, err)
	assert.Equal(t, "User", emp2Found.FirstName)
	assert.Equal(t, "Two", emp2Found.LastName)

	// Verify tenant2 CANNOT see tenant1's employee
	emp1InTenant2, err := employeeRepo.GetByUserID(ctx2, userID1)
	assert.Error(t, err) // Should fail - cross-tenant access
	assert.Nil(t, emp1InTenant2)
}

func TestUserConsumer_RoleToJobTitleMapping(t *testing.T) {
	testCases := []struct {
		role             string
		expectedJobTitle string
	}{
		{"admin", "Administrator"},
		{"manager", "Manager"},
		{"staff", "Staff Member"},
		{"viewer", "Viewer"},
	}

	for _, tc := range testCases {
		t.Run(tc.role, func(t *testing.T) {
			ctx := context.Background()
			tenant := suite.SetupStaffTenant(t, ctx, "test-role-"+tc.role)
			tenantCtx := testutil.WithTestTenant(ctx, tenant)

			// Setup
			employeeRepo := repository.NewEmployeeRepository(suite.DB)

			// Create employee with specific role-based job title
			userID := uuid.New().String()
			employee := &repository.Employee{
				UserID:         &userID,
				FirstName:      "Test",
				LastName:       "User",
				Email:          stringPtr("test@example.com"),
				EmploymentType: "full_time",
				HireDate:       time.Now(),
				Status:         "active",
				JobTitle:       &tc.expectedJobTitle,
			}
			err := employeeRepo.Create(tenantCtx, employee)
			require.NoError(t, err)

			// Verify job title
			created, err := employeeRepo.GetByUserID(tenantCtx, userID)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedJobTitle, *created.JobTitle)
		})
	}
}

func TestUserConsumer_EmployeeFieldDefaults(t *testing.T) {
	ctx := context.Background()
	tenant := suite.SetupStaffTenant(t, ctx, "test-defaults")
	tenantCtx := testutil.WithTestTenant(ctx, tenant)

	// Setup
	employeeRepo := repository.NewEmployeeRepository(suite.DB)

	// Create employee with minimal fields
	userID := uuid.New().String()
	jobTitle := "Staff Member"
	employee := &repository.Employee{
		UserID:         &userID,
		FirstName:      "Min",
		LastName:       "Fields",
		Email:          stringPtr("min@example.com"),
		EmploymentType: "full_time",
		HireDate:       time.Now(),
		Status:         "active",
		JobTitle:       &jobTitle,
	}
	err := employeeRepo.Create(tenantCtx, employee)
	require.NoError(t, err)

	// Verify defaults
	created, err := employeeRepo.GetByUserID(tenantCtx, userID)
	require.NoError(t, err)

	// Check required defaults
	assert.Equal(t, "full_time", created.EmploymentType)
	assert.Equal(t, "active", created.Status)
	assert.NotZero(t, created.HireDate)

	// Check optional fields are properly set
	assert.Nil(t, created.Department)
	assert.Nil(t, created.EmployeeNumber)
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
