package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// SHIFT TEMPLATE TESTS
// ============================================================================

func TestShiftRepository_CreateTemplate(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-create-template")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	tmpl := &repository.ShiftTemplate{
		Name:                 "Morning Shift",
		StartTime:            "08:00:00",
		EndTime:              "16:00:00",
		BreakDurationMinutes: 30,
		ShiftType:            "regular",
		Color:                "#22c55e",
		IsActive:             true,
	}

	err := repo.CreateTemplate(tenantCtx, tmpl)
	require.NoError(t, err)
	assert.NotEmpty(t, tmpl.ID)
}

func TestShiftRepository_GetTemplateByID(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-get-template")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	tmpl := &repository.ShiftTemplate{
		Name:                 "Evening Shift",
		StartTime:            "14:00:00",
		EndTime:              "22:00:00",
		BreakDurationMinutes: 45,
		ShiftType:            "regular",
		Color:                "#3b82f6",
		IsActive:             true,
	}
	err := repo.CreateTemplate(tenantCtx, tmpl)
	require.NoError(t, err)

	retrieved, err := repo.GetTemplateByID(tenantCtx, tmpl.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, "Evening Shift", retrieved.Name)
	assert.Equal(t, 45, retrieved.BreakDurationMinutes)
}

func TestShiftRepository_ListTemplates(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-list-templates")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	templates := []*repository.ShiftTemplate{
		{Name: "Morning", StartTime: "06:00:00", EndTime: "14:00:00", ShiftType: "regular", IsActive: true},
		{Name: "Afternoon", StartTime: "14:00:00", EndTime: "22:00:00", ShiftType: "regular", IsActive: true},
		{Name: "Night", StartTime: "22:00:00", EndTime: "06:00:00", ShiftType: "night", IsActive: false},
	}
	for _, tmpl := range templates {
		err := repo.CreateTemplate(tenantCtx, tmpl)
		require.NoError(t, err)
	}

	// List all
	all, err := repo.ListTemplates(tenantCtx, false)
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// List active only
	active, err := repo.ListTemplates(tenantCtx, true)
	require.NoError(t, err)
	assert.Len(t, active, 2)
}

func TestShiftRepository_UpdateTemplate(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-update-template")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	tmpl := &repository.ShiftTemplate{
		Name:      "Original",
		StartTime: "08:00:00",
		EndTime:   "16:00:00",
		ShiftType: "regular",
		IsActive:  true,
	}
	err := repo.CreateTemplate(tenantCtx, tmpl)
	require.NoError(t, err)

	tmpl.Name = "Updated"
	tmpl.ShiftType = "on_call"
	err = repo.UpdateTemplate(tenantCtx, tmpl)
	require.NoError(t, err)

	updated, err := repo.GetTemplateByID(tenantCtx, tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)
	assert.Equal(t, "on_call", updated.ShiftType)
}

func TestShiftRepository_DeleteTemplate(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-delete-template")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	tmpl := &repository.ShiftTemplate{
		Name:      "ToDelete",
		StartTime: "08:00:00",
		EndTime:   "16:00:00",
		ShiftType: "regular",
		IsActive:  true,
	}
	err := repo.CreateTemplate(tenantCtx, tmpl)
	require.NoError(t, err)

	err = repo.DeleteTemplate(tenantCtx, tmpl.ID)
	require.NoError(t, err)

	// Should not appear in active list
	active, err := repo.ListTemplates(tenantCtx, true)
	require.NoError(t, err)
	assert.Len(t, active, 0)
}

// ============================================================================
// SHIFT ASSIGNMENT TESTS
// ============================================================================

// createShiftTestEmployee is a helper to create an employee for shift tests
func createShiftTestEmployee(t *testing.T, tenantCtx context.Context, firstName string) *repository.Employee {
	t.Helper()
	empRepo := repository.NewEmployeeRepository(suite.DB)
	emp := &repository.Employee{
		FirstName:      firstName,
		LastName:       "Test",
		EmploymentType: "full_time",
		HireDate:       time.Now().UTC().Truncate(time.Second),
		Status:         "active",
	}
	err := empRepo.Create(tenantCtx, emp)
	require.NoError(t, err)
	return emp
}

func TestShiftRepository_CreateAssignment(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-create-assignment")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	emp := createShiftTestEmployee(t, tenantCtx, "ShiftCreate")

	shift := &repository.ShiftAssignment{
		EmployeeID:           emp.ID,
		ShiftDate:            time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		StartTime:            "08:00:00",
		EndTime:              "16:00:00",
		BreakDurationMinutes: 30,
		ShiftType:            "regular",
		Status:               "scheduled",
	}

	err := repo.CreateAssignment(tenantCtx, shift)
	require.NoError(t, err)
	assert.NotEmpty(t, shift.ID)
	assert.Equal(t, "regular", shift.ShiftType)
	assert.Equal(t, "scheduled", shift.Status)
}

func TestShiftRepository_GetAssignmentByID(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-get-assignment")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	emp := createShiftTestEmployee(t, tenantCtx, "ShiftGet")

	shift := &repository.ShiftAssignment{
		EmployeeID:           emp.ID,
		ShiftDate:            time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		StartTime:            "09:00:00",
		EndTime:              "17:00:00",
		BreakDurationMinutes: 30,
		ShiftType:            "on_call",
	}
	err := repo.CreateAssignment(tenantCtx, shift)
	require.NoError(t, err)

	retrieved, err := repo.GetAssignmentByID(tenantCtx, shift.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, emp.ID, retrieved.EmployeeID)
	assert.Equal(t, "on_call", retrieved.ShiftType)
	assert.Equal(t, 30, retrieved.BreakDurationMinutes)
	// Joined employee name should be populated
	assert.NotNil(t, retrieved.EmployeeName)
	assert.Contains(t, *retrieved.EmployeeName, "ShiftGet")
}

func TestShiftRepository_ListAssignments(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-list-assignments")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	emp := createShiftTestEmployee(t, tenantCtx, "ShiftList")

	baseDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)

	// Create 3 shifts on different dates
	for i := 0; i < 3; i++ {
		shift := &repository.ShiftAssignment{
			EmployeeID: emp.ID,
			ShiftDate:  baseDate.AddDate(0, 0, i),
			StartTime:  "08:00:00",
			EndTime:    "16:00:00",
			ShiftType:  "regular",
		}
		err := repo.CreateAssignment(tenantCtx, shift)
		require.NoError(t, err)
	}

	// List all
	params := repository.ShiftListParams{Page: 1, PerPage: 50}
	shifts, total, err := repo.ListAssignments(tenantCtx, params)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, shifts, 3)

	// List with employee filter
	empID := emp.ID
	params.EmployeeID = &empID
	shifts, total, err = repo.ListAssignments(tenantCtx, params)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)

	// List with date range filter
	startDate := baseDate
	endDate := baseDate.AddDate(0, 0, 1)
	params = repository.ShiftListParams{
		Page:      1,
		PerPage:   50,
		StartDate: &startDate,
		EndDate:   &endDate,
	}
	shifts, total, err = repo.ListAssignments(tenantCtx, params)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, shifts, 2)
}

func TestShiftRepository_UpdateAssignment(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-update-assignment")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	emp := createShiftTestEmployee(t, tenantCtx, "ShiftUpdate")

	shift := &repository.ShiftAssignment{
		EmployeeID: emp.ID,
		ShiftDate:  time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
		StartTime:  "08:00:00",
		EndTime:    "16:00:00",
		ShiftType:  "regular",
		Status:     "scheduled",
	}
	err := repo.CreateAssignment(tenantCtx, shift)
	require.NoError(t, err)

	// Update shift
	shift.StartTime = "09:00:00"
	shift.EndTime = "17:00:00"
	shift.ShiftType = "emergency"
	shift.Status = "confirmed"
	err = repo.UpdateAssignment(tenantCtx, shift)
	require.NoError(t, err)

	// Verify
	updated, err := repo.GetAssignmentByID(tenantCtx, shift.ID)
	require.NoError(t, err)
	assert.Equal(t, "emergency", updated.ShiftType)
	assert.Equal(t, "confirmed", updated.Status)
}

func TestShiftRepository_DeleteAssignment(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-delete-assignment")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	emp := createShiftTestEmployee(t, tenantCtx, "ShiftDelete")

	shift := &repository.ShiftAssignment{
		EmployeeID: emp.ID,
		ShiftDate:  time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC),
		StartTime:  "08:00:00",
		EndTime:    "16:00:00",
		ShiftType:  "regular",
	}
	err := repo.CreateAssignment(tenantCtx, shift)
	require.NoError(t, err)

	// Delete
	err = repo.DeleteAssignment(tenantCtx, shift.ID)
	require.NoError(t, err)

	// Verify it's gone (soft deleted)
	_, err = repo.GetAssignmentByID(tenantCtx, shift.ID)
	assert.Error(t, err)
}

func TestShiftRepository_BulkCreateAssignments(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-bulk-create")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	emp1 := createShiftTestEmployee(t, tenantCtx, "BulkEmp1")
	emp2 := createShiftTestEmployee(t, tenantCtx, "BulkEmp2")

	shifts := []*repository.ShiftAssignment{
		{
			EmployeeID: emp1.ID,
			ShiftDate:  time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			StartTime:  "08:00:00",
			EndTime:    "16:00:00",
			ShiftType:  "regular",
		},
		{
			EmployeeID: emp2.ID,
			ShiftDate:  time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			StartTime:  "10:00:00",
			EndTime:    "18:00:00",
			ShiftType:  "on_call",
		},
	}

	err := repo.BulkCreateAssignments(tenantCtx, shifts)
	require.NoError(t, err)

	// Verify both got IDs
	for _, s := range shifts {
		assert.NotEmpty(t, s.ID)
	}

	// Verify they're retrievable
	retrieved, err := repo.GetAssignmentByID(tenantCtx, shifts[0].ID)
	require.NoError(t, err)
	assert.Equal(t, emp1.ID, retrieved.EmployeeID)
}

func TestShiftRepository_CheckForConflicts(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-conflicts")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	emp := createShiftTestEmployee(t, tenantCtx, "ConflictEmp")

	// Create an existing shift 08:00-16:00
	existing := &repository.ShiftAssignment{
		EmployeeID: emp.ID,
		ShiftDate:  time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		StartTime:  "08:00:00",
		EndTime:    "16:00:00",
		ShiftType:  "regular",
	}
	err := repo.CreateAssignment(tenantCtx, existing)
	require.NoError(t, err)

	// Test overlapping shift (10:00-18:00 same day) - should conflict
	hasConflict, reason, err := repo.CheckForConflicts(
		tenantCtx, emp.ID,
		time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		"10:00:00", "18:00:00",
		nil,
	)
	require.NoError(t, err)
	assert.True(t, hasConflict)
	assert.NotEmpty(t, reason)

	// Test non-overlapping shift (17:00-22:00 same day) - no conflict
	hasConflict, _, err = repo.CheckForConflicts(
		tenantCtx, emp.ID,
		time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		"17:00:00", "22:00:00",
		nil,
	)
	require.NoError(t, err)
	assert.False(t, hasConflict)

	// Test excluding self (same shift ID) - no conflict
	hasConflict, _, err = repo.CheckForConflicts(
		tenantCtx, emp.ID,
		time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		"08:00:00", "16:00:00",
		&existing.ID,
	)
	require.NoError(t, err)
	assert.False(t, hasConflict)

	// Test different day - no conflict
	hasConflict, _, err = repo.CheckForConflicts(
		tenantCtx, emp.ID,
		time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC),
		"08:00:00", "16:00:00",
		nil,
	)
	require.NoError(t, err)
	assert.False(t, hasConflict)
}

func TestShiftRepository_GetEmployeeShiftsForDateRange(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffTenant(t, ctx, "test-emp-date-range")
	repo := repository.NewShiftRepository(suite.DB)
	tenantCtx := suite.TenantContext(tenant)

	emp := createShiftTestEmployee(t, tenantCtx, "DateRange")

	// Create shifts across multiple dates
	dates := []time.Time{
		time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
	}
	for _, d := range dates {
		shift := &repository.ShiftAssignment{
			EmployeeID: emp.ID,
			ShiftDate:  d,
			StartTime:  "08:00:00",
			EndTime:    "16:00:00",
			ShiftType:  "regular",
		}
		err := repo.CreateAssignment(tenantCtx, shift)
		require.NoError(t, err)
	}

	// Query a date range that covers 3 of 4 shifts
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)

	shifts, err := repo.GetEmployeeShiftsForDateRange(tenantCtx, emp.ID, start, end)
	require.NoError(t, err)
	assert.Len(t, shifts, 3)
}

// ============================================================================
// TENANT ISOLATION TESTS
// ============================================================================

func TestShiftRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()

	tenant1 := suite.SetupStaffTenant(t, ctx, "shift-iso-1")
	tenant2 := suite.SetupStaffTenant(t, ctx, "shift-iso-2")

	repo := repository.NewShiftRepository(suite.DB)

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	// Create employee and shift in tenant 1
	emp1 := createShiftTestEmployee(t, ctx1, "IsoEmp1")
	shift1 := &repository.ShiftAssignment{
		EmployeeID: emp1.ID,
		ShiftDate:  time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		StartTime:  "08:00:00",
		EndTime:    "16:00:00",
		ShiftType:  "regular",
	}
	err := repo.CreateAssignment(ctx1, shift1)
	require.NoError(t, err)

	// Tenant 2 should NOT see tenant 1's shift
	_, err = repo.GetAssignmentByID(ctx2, shift1.ID)
	assert.Error(t, err, "tenant 2 should not access tenant 1's shift")

	// Tenant 2's list should be empty
	params := repository.ShiftListParams{Page: 1, PerPage: 50}
	shifts, total, err := repo.ListAssignments(ctx2, params)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Len(t, shifts, 0)

	// Tenant 1 should see its own shift
	shifts, total, err = repo.ListAssignments(ctx1, params)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, shifts, 1)
}

func TestShiftRepository_TemplateTenantIsolation(t *testing.T) {
	ctx := context.Background()

	tenant1 := suite.SetupStaffTenant(t, ctx, "tmpl-iso-1")
	tenant2 := suite.SetupStaffTenant(t, ctx, "tmpl-iso-2")

	repo := repository.NewShiftRepository(suite.DB)

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	// Create template in tenant 1
	tmpl := &repository.ShiftTemplate{
		Name:      "Tenant1Template",
		StartTime: "08:00:00",
		EndTime:   "16:00:00",
		ShiftType: "regular",
		IsActive:  true,
	}
	err := repo.CreateTemplate(ctx1, tmpl)
	require.NoError(t, err)

	// Tenant 2 should NOT see it
	_, err = repo.GetTemplateByID(ctx2, tmpl.ID)
	assert.Error(t, err, "tenant 2 should not access tenant 1's template")

	templates, err := repo.ListTemplates(ctx2, false)
	require.NoError(t, err)
	assert.Len(t, templates, 0)
}
