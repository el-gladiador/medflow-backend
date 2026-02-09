package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: create an employee in the given tenant context and return its ID
func createTestEmployee(t *testing.T, ctx context.Context, firstName, lastName string) string {
	t.Helper()
	repo := repository.NewEmployeeRepository(suite.DB)
	emp := &repository.Employee{
		FirstName:      firstName,
		LastName:       lastName,
		EmploymentType: "full_time",
		HireDate:       time.Now().UTC().Truncate(time.Second),
		Status:         "active",
	}
	err := repo.Create(ctx, emp)
	require.NoError(t, err)
	return emp.ID
}

// helper: clock in an employee and return the time entry
func clockInEmployee(t *testing.T, ctx context.Context, repo *repository.TimeTrackingRepository, employeeID string) *repository.TimeEntry {
	t.Helper()
	now := time.Now().UTC()
	entry := &repository.TimeEntry{
		EmployeeID:    employeeID,
		EntryDate:     time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
		ClockIn:       now,
		IsManualEntry: false,
	}
	err := repo.CreateEntry(ctx, entry)
	require.NoError(t, err)
	require.NotEmpty(t, entry.ID)
	return entry
}

// helper: start a break for a time entry and return the break
func startBreak(t *testing.T, ctx context.Context, repo *repository.TimeTrackingRepository, timeEntryID string) *repository.TimeBreak {
	t.Helper()
	brk := &repository.TimeBreak{
		TimeEntryID: timeEntryID,
		StartTime:   time.Now().UTC(),
	}
	err := repo.CreateBreak(ctx, brk)
	require.NoError(t, err)
	require.NotEmpty(t, brk.ID)
	return brk
}

// ============================================================================
// END BREAK FLOW TESTS
// ============================================================================

func TestTimeTracking_EndBreak_FullFlow(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-end-break-flow")
	tenantCtx := suite.TenantContext(tenant)

	repo := repository.NewTimeTrackingRepository(suite.DB)
	employeeID := createTestEmployee(t, tenantCtx, "Anna", "Schmidt")

	// Step 1: Clock in
	entry := clockInEmployee(t, tenantCtx, repo, employeeID)

	// Step 2: Start break
	brk := startBreak(t, tenantCtx, repo, entry.ID)

	// Verify active break exists
	activeBreak, err := repo.GetActiveBreak(tenantCtx, entry.ID)
	require.NoError(t, err)
	require.NotNil(t, activeBreak)
	assert.Equal(t, brk.ID, activeBreak.ID)
	assert.Nil(t, activeBreak.EndTime)

	// Step 3: End break (the flow that was broken on frontend)
	now := time.Now().UTC()
	activeBreak.EndTime = &now
	err = repo.UpdateBreak(tenantCtx, activeBreak)
	require.NoError(t, err)

	// Step 4: Verify break is ended
	noActiveBreak, err := repo.GetActiveBreak(tenantCtx, entry.ID)
	require.NoError(t, err)
	assert.Nil(t, noActiveBreak, "should have no active break after ending it")

	// Step 5: Verify break shows in list with end time
	breaks, err := repo.ListBreaksForEntry(tenantCtx, entry.ID)
	require.NoError(t, err)
	require.Len(t, breaks, 1)
	assert.Equal(t, brk.ID, breaks[0].ID)
	assert.NotNil(t, breaks[0].EndTime)

	// Step 6: Verify entry is still active (can resume work)
	activeEntry, err := repo.GetActiveEntryByEmployeeID(tenantCtx, employeeID)
	require.NoError(t, err)
	require.NotNil(t, activeEntry, "entry should still be active after ending break")
	assert.Nil(t, activeEntry.ClockOut)
}

func TestTimeTracking_EndBreak_NotOnBreak(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-end-break-not-on-break")
	tenantCtx := suite.TenantContext(tenant)

	repo := repository.NewTimeTrackingRepository(suite.DB)
	employeeID := createTestEmployee(t, tenantCtx, "Max", "Mueller")

	// Clock in but don't start a break
	entry := clockInEmployee(t, tenantCtx, repo, employeeID)

	// GetActiveBreak should return nil (not an error)
	activeBreak, err := repo.GetActiveBreak(tenantCtx, entry.ID)
	require.NoError(t, err)
	assert.Nil(t, activeBreak, "should have no active break")
}

func TestTimeTracking_EndBreak_NotClockedIn(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-end-break-not-clocked-in")
	tenantCtx := suite.TenantContext(tenant)

	repo := repository.NewTimeTrackingRepository(suite.DB)
	employeeID := createTestEmployee(t, tenantCtx, "Klaus", "Weber")

	// No clock in - GetActiveEntry should return nil
	activeEntry, err := repo.GetActiveEntryByEmployeeID(tenantCtx, employeeID)
	require.NoError(t, err)
	assert.Nil(t, activeEntry, "should have no active entry")
}

func TestTimeTracking_MultipleBreaks(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-multiple-breaks")
	tenantCtx := suite.TenantContext(tenant)

	repo := repository.NewTimeTrackingRepository(suite.DB)
	employeeID := createTestEmployee(t, tenantCtx, "Petra", "Bauer")

	entry := clockInEmployee(t, tenantCtx, repo, employeeID)

	// First break: start and end
	brk1 := startBreak(t, tenantCtx, repo, entry.ID)
	now1 := time.Now().UTC()
	brk1.EndTime = &now1
	err := repo.UpdateBreak(tenantCtx, brk1)
	require.NoError(t, err)

	// Second break: start and end
	brk2 := startBreak(t, tenantCtx, repo, entry.ID)
	now2 := time.Now().UTC()
	brk2.EndTime = &now2
	err = repo.UpdateBreak(tenantCtx, brk2)
	require.NoError(t, err)

	// Verify both breaks are recorded
	breaks, err := repo.ListBreaksForEntry(tenantCtx, entry.ID)
	require.NoError(t, err)
	assert.Len(t, breaks, 2)

	// No active break remains
	activeBreak, err := repo.GetActiveBreak(tenantCtx, entry.ID)
	require.NoError(t, err)
	assert.Nil(t, activeBreak)

	// Verify total break minutes are calculated
	totalBreakMin, err := repo.CalculateTotalBreakMinutes(tenantCtx, entry.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, totalBreakMin, 0)
}

func TestTimeTracking_ClockIn_ClockOut_WithBreak(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-full-day-flow")
	tenantCtx := suite.TenantContext(tenant)

	repo := repository.NewTimeTrackingRepository(suite.DB)
	employeeID := createTestEmployee(t, tenantCtx, "Hans", "Fischer")

	// Clock in
	entry := clockInEmployee(t, tenantCtx, repo, employeeID)

	// Start break
	brk := startBreak(t, tenantCtx, repo, entry.ID)

	// End break
	breakEnd := time.Now().UTC()
	brk.EndTime = &breakEnd
	err := repo.UpdateBreak(tenantCtx, brk)
	require.NoError(t, err)

	// Clock out
	clockOut := time.Now().UTC()
	entry.ClockOut = &clockOut

	totalBreakMin, err := repo.CalculateTotalBreakMinutes(tenantCtx, entry.ID)
	require.NoError(t, err)
	entry.TotalBreakMinutes = totalBreakMin

	totalMinutes := int(entry.ClockOut.Sub(entry.ClockIn).Minutes())
	entry.TotalWorkMinutes = totalMinutes - totalBreakMin
	if entry.TotalWorkMinutes < 0 {
		entry.TotalWorkMinutes = 0
	}

	err = repo.UpdateEntry(tenantCtx, entry)
	require.NoError(t, err)

	// Verify entry is no longer active
	activeEntry, err := repo.GetActiveEntryByEmployeeID(tenantCtx, employeeID)
	require.NoError(t, err)
	assert.Nil(t, activeEntry, "should have no active entry after clock out")

	// Verify the completed entry
	completed, err := repo.GetEntryByID(tenantCtx, entry.ID)
	require.NoError(t, err)
	require.NotNil(t, completed)
	assert.NotNil(t, completed.ClockOut)
	assert.GreaterOrEqual(t, completed.TotalBreakMinutes, 0)
	assert.GreaterOrEqual(t, completed.TotalWorkMinutes, 0)
}

// ============================================================================
// TENANT ISOLATION TESTS
// ============================================================================

func TestTimeTracking_TenantIsolation(t *testing.T) {
	ctx := context.Background()

	// Create two separate tenants
	tenant1 := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-tt-isolation-1")
	tenant2 := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-tt-isolation-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	repo := repository.NewTimeTrackingRepository(suite.DB)

	// Create employees in each tenant
	emp1ID := createTestEmployee(t, ctx1, "Tenant1", "Employee")
	emp2ID := createTestEmployee(t, ctx2, "Tenant2", "Employee")

	// Clock in employee in tenant 1, start and end break
	entry1 := clockInEmployee(t, ctx1, repo, emp1ID)
	brk1 := startBreak(t, ctx1, repo, entry1.ID)
	now := time.Now().UTC()
	brk1.EndTime = &now
	err := repo.UpdateBreak(ctx1, brk1)
	require.NoError(t, err)

	// Clock in employee in tenant 2
	clockInEmployee(t, ctx2, repo, emp2ID)

	// CRITICAL: Tenant 2 cannot see tenant 1's time entry
	crossEntry, err := repo.GetEntryByID(ctx2, entry1.ID)
	assert.Error(t, err, "cross-tenant access to time entry must fail")
	assert.Nil(t, crossEntry)

	// CRITICAL: Tenant 2 cannot see tenant 1's active entry
	crossActive, err := repo.GetActiveEntryByEmployeeID(ctx2, emp1ID)
	require.NoError(t, err) // returns nil, not error, for no rows
	assert.Nil(t, crossActive, "cross-tenant should not see other tenant's active entry")

	// CRITICAL: Tenant 2 cannot see tenant 1's breaks
	crossBreaks, err := repo.ListBreaksForEntry(ctx2, entry1.ID)
	require.NoError(t, err)
	assert.Empty(t, crossBreaks, "cross-tenant should not see other tenant's breaks")
}

// ============================================================================
// MANUAL ENTRY & CORRECTION TESTS
// ============================================================================

func TestTimeTracking_ManualClockIn(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-manual-clock-in")
	tenantCtx := suite.TenantContext(tenant)

	repo := repository.NewTimeTrackingRepository(suite.DB)
	employeeID := createTestEmployee(t, tenantCtx, "Lisa", "Hartmann")

	// Create manual clock-in entry
	now := time.Now().UTC()
	entry := &repository.TimeEntry{
		EmployeeID:    employeeID,
		EntryDate:     time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
		ClockIn:       time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, now.Location()),
		IsManualEntry: true,
	}
	err := repo.CreateEntry(tenantCtx, entry)
	require.NoError(t, err)
	require.NotEmpty(t, entry.ID)

	// Verify entry was created with is_manual_entry = true
	fetched, err := repo.GetEntryByID(tenantCtx, entry.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.True(t, fetched.IsManualEntry, "manual entry should have is_manual_entry = true")
	assert.Equal(t, employeeID, fetched.EmployeeID)
	assert.Nil(t, fetched.ClockOut, "clock out should be nil for clock-in only")
}

func TestTimeTracking_ManualClockOut(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-manual-clock-out")
	tenantCtx := suite.TenantContext(tenant)

	repo := repository.NewTimeTrackingRepository(suite.DB)
	employeeID := createTestEmployee(t, tenantCtx, "Thomas", "Richter")

	// Create manual entry with clock-in
	now := time.Now().UTC()
	entry := &repository.TimeEntry{
		EmployeeID:    employeeID,
		EntryDate:     time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
		ClockIn:       time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, now.Location()),
		IsManualEntry: true,
	}
	err := repo.CreateEntry(tenantCtx, entry)
	require.NoError(t, err)

	// Manual clock-out
	clockOutTime := time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, now.Location())
	entry.ClockOut = &clockOutTime
	entry.TotalWorkMinutes = int(clockOutTime.Sub(entry.ClockIn).Minutes())
	err = repo.UpdateEntry(tenantCtx, entry)
	require.NoError(t, err)

	// Verify entry is completed
	fetched, err := repo.GetEntryByID(tenantCtx, entry.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.NotNil(t, fetched.ClockOut)
	assert.True(t, fetched.IsManualEntry)
	assert.Equal(t, 540, fetched.TotalWorkMinutes) // 9 hours = 540 minutes

	// Should not be active anymore
	active, err := repo.GetActiveEntryByEmployeeID(tenantCtx, employeeID)
	require.NoError(t, err)
	assert.Nil(t, active, "should have no active entry after manual clock out")
}

func TestTimeTracking_UpdateEntry(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-update-entry")
	tenantCtx := suite.TenantContext(tenant)

	repo := repository.NewTimeTrackingRepository(suite.DB)
	employeeID := createTestEmployee(t, tenantCtx, "Maria", "Schneider")

	// Create and clock in
	entry := clockInEmployee(t, tenantCtx, repo, employeeID)
	originalClockIn := entry.ClockIn

	// Update clock-in time (e.g., manager correcting it)
	newClockIn := originalClockIn.Add(-30 * time.Minute) // 30 min earlier
	entry.ClockIn = newClockIn
	entry.Notes = stringPtr("Corrected clock-in time")
	err := repo.UpdateEntry(tenantCtx, entry)
	require.NoError(t, err)

	// Verify updated
	fetched, err := repo.GetEntryByID(tenantCtx, entry.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.WithinDuration(t, newClockIn, fetched.ClockIn, time.Second)
	assert.NotNil(t, fetched.Notes)
	assert.Equal(t, "Corrected clock-in time", *fetched.Notes)
}

func TestTimeTracking_SoftDeleteEntry(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-soft-delete-entry")
	tenantCtx := suite.TenantContext(tenant)

	repo := repository.NewTimeTrackingRepository(suite.DB)
	employeeID := createTestEmployee(t, tenantCtx, "Stefan", "Wagner")

	// Create entry
	entry := clockInEmployee(t, tenantCtx, repo, employeeID)

	// Soft delete
	err := repo.SoftDeleteEntry(tenantCtx, entry.ID)
	require.NoError(t, err)

	// Entry should no longer be retrievable
	deleted, err := repo.GetEntryByID(tenantCtx, entry.ID)
	assert.Error(t, err, "soft-deleted entry should not be found")
	assert.Nil(t, deleted)

	// Should not appear as active
	active, err := repo.GetActiveEntryByEmployeeID(tenantCtx, employeeID)
	require.NoError(t, err)
	assert.Nil(t, active, "soft-deleted entry should not be active")

	// Double delete should fail gracefully (not found)
	err = repo.SoftDeleteEntry(tenantCtx, entry.ID)
	assert.Error(t, err, "deleting already-deleted entry should error")
}

func TestTimeTracking_CreateCorrection(t *testing.T) {
	ctx := context.Background()

	tenant := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-create-correction")
	tenantCtx := suite.TenantContext(tenant)

	repo := repository.NewTimeTrackingRepository(suite.DB)
	employeeID := createTestEmployee(t, tenantCtx, "Claudia", "Meyer")

	// Create and complete an entry
	entry := clockInEmployee(t, tenantCtx, repo, employeeID)
	clockOutTime := time.Now().UTC()
	entry.ClockOut = &clockOutTime
	entry.TotalWorkMinutes = 480
	err := repo.UpdateEntry(tenantCtx, entry)
	require.NoError(t, err)

	// Create a correction record
	correctedClockIn := entry.ClockIn.Add(-15 * time.Minute)
	correctorID := "manager-uuid-123"
	corr := &repository.TimeCorrection{
		EmployeeID:       employeeID,
		TimeEntryID:      &entry.ID,
		CorrectionDate:   entry.EntryDate,
		OriginalClockIn:  &entry.ClockIn,
		CorrectedClockIn: &correctedClockIn,
		Reason:           "Employee forgot to clock in on time",
		CorrectedBy:      correctorID,
	}
	err = repo.CreateCorrection(tenantCtx, corr)
	require.NoError(t, err)
	require.NotEmpty(t, corr.ID)
	assert.NotZero(t, corr.CreatedAt)

	// List corrections for employee
	now := time.Now().UTC()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endDate := startDate.AddDate(0, 1, -1)
	corrections, err := repo.ListCorrectionsForEmployee(tenantCtx, employeeID, startDate, endDate)
	require.NoError(t, err)
	require.Len(t, corrections, 1)
	assert.Equal(t, corr.ID, corrections[0].ID)
	assert.Equal(t, "Employee forgot to clock in on time", corrections[0].Reason)
	assert.Equal(t, correctorID, corrections[0].CorrectedBy)
}

func TestTimeTracking_TenantIsolation_ManualEntry(t *testing.T) {
	ctx := context.Background()

	// Create two separate tenants
	tenant1 := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-tt-manual-iso-1")
	tenant2 := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-tt-manual-iso-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	repo := repository.NewTimeTrackingRepository(suite.DB)

	// Create employee and manual entry in tenant 1
	emp1ID := createTestEmployee(t, ctx1, "Tenant1", "Manual")
	now := time.Now().UTC()
	entry := &repository.TimeEntry{
		EmployeeID:    emp1ID,
		EntryDate:     time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
		ClockIn:       time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, now.Location()),
		IsManualEntry: true,
	}
	err := repo.CreateEntry(ctx1, entry)
	require.NoError(t, err)

	// CRITICAL: Tenant 2 cannot see tenant 1's manual entry
	crossEntry, err := repo.GetEntryByID(ctx2, entry.ID)
	assert.Error(t, err, "cross-tenant access to manual entry must fail")
	assert.Nil(t, crossEntry)

	// CRITICAL: Tenant 2 cannot update tenant 1's entry
	clockOutTime := time.Now().UTC()
	entry.ClockOut = &clockOutTime
	err = repo.UpdateEntry(ctx2, entry)
	assert.Error(t, err, "cross-tenant update of manual entry must fail")

	// CRITICAL: Tenant 2 cannot soft-delete tenant 1's entry
	err = repo.SoftDeleteEntry(ctx2, entry.ID)
	assert.Error(t, err, "cross-tenant soft-delete must fail")

	// Verify entry is still intact in tenant 1
	intact, err := repo.GetEntryByID(ctx1, entry.ID)
	require.NoError(t, err)
	require.NotNil(t, intact)
	assert.Nil(t, intact.ClockOut, "entry should not have been modified by cross-tenant attempt")
}

// helper: create a string pointer
func stringPtr(s string) *string {
	return &s
}

func TestTimeTracking_TenantIsolation_UpdateBreak(t *testing.T) {
	ctx := context.Background()

	tenant1 := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-tt-update-iso-1")
	tenant2 := suite.SetupStaffWithTimeTrackingTenant(t, ctx, "test-tt-update-iso-2")

	ctx1 := suite.TenantContext(tenant1)
	ctx2 := suite.TenantContext(tenant2)

	repo := repository.NewTimeTrackingRepository(suite.DB)

	// Create and clock in employee in tenant 1, start break
	emp1ID := createTestEmployee(t, ctx1, "Tenant1", "Break")
	entry1 := clockInEmployee(t, ctx1, repo, emp1ID)
	brk1 := startBreak(t, ctx1, repo, entry1.ID)

	// CRITICAL: Tenant 2 must not be able to end tenant 1's break
	now := time.Now().UTC()
	brk1.EndTime = &now
	err := repo.UpdateBreak(ctx2, brk1)
	require.Error(t, err, "cross-tenant break update must fail")

	// Verify break is still active in tenant 1
	activeBreak, err := repo.GetActiveBreak(ctx1, entry1.ID)
	require.NoError(t, err)
	require.NotNil(t, activeBreak, "break should still be active (cross-tenant update failed)")
	assert.Nil(t, activeBreak.EndTime)
}
