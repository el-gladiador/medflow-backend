package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// TimeEntry represents a daily clock in/out record
type TimeEntry struct {
	ID                string     `db:"id" json:"id"`
	EmployeeID        string     `db:"employee_id" json:"employee_id"`
	EntryDate         time.Time  `db:"entry_date" json:"entry_date"`
	ClockIn           time.Time  `db:"clock_in" json:"clock_in"`
	ClockOut          *time.Time `db:"clock_out" json:"clock_out,omitempty"`
	TotalWorkMinutes  int        `db:"total_work_minutes" json:"total_work_minutes"`
	TotalBreakMinutes int        `db:"total_break_minutes" json:"total_break_minutes"`
	Notes             *string    `db:"notes" json:"notes,omitempty"`
	IsManualEntry     bool       `db:"is_manual_entry" json:"is_manual_entry"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt         *time.Time `db:"deleted_at" json:"-"`
	CreatedBy         *string    `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy         *string    `db:"updated_by" json:"updated_by,omitempty"`

	// Joined fields (populated by specific queries)
	EmployeeName *string `db:"employee_name" json:"employee_name,omitempty"`
}

// TimeBreak represents a break within a time entry
type TimeBreak struct {
	ID          string     `db:"id" json:"id"`
	TimeEntryID string     `db:"time_entry_id" json:"time_entry_id"`
	StartTime   time.Time  `db:"start_time" json:"start_time"`
	EndTime     *time.Time `db:"end_time" json:"end_time,omitempty"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

// TimeCorrection represents an audit trail entry for manager corrections
type TimeCorrection struct {
	ID                 string     `db:"id" json:"id"`
	EmployeeID         string     `db:"employee_id" json:"employee_id"`
	TimeEntryID        *string    `db:"time_entry_id" json:"time_entry_id,omitempty"`
	CorrectionDate     time.Time  `db:"correction_date" json:"correction_date"`
	OriginalClockIn    *time.Time `db:"original_clock_in" json:"original_clock_in,omitempty"`
	OriginalClockOut   *time.Time `db:"original_clock_out" json:"original_clock_out,omitempty"`
	CorrectedClockIn   *time.Time `db:"corrected_clock_in" json:"corrected_clock_in,omitempty"`
	CorrectedClockOut  *time.Time `db:"corrected_clock_out" json:"corrected_clock_out,omitempty"`
	Reason             string     `db:"reason" json:"reason"`
	CorrectedBy        string     `db:"corrected_by" json:"corrected_by"`
	CreatedAt          time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt          *time.Time `db:"deleted_at" json:"-"`

	// Joined fields
	EmployeeName  *string `db:"employee_name" json:"employee_name,omitempty"`
	CorrectorName *string `db:"corrector_name" json:"corrector_name,omitempty"`
}

// EmployeeTimeStatus represents the current time tracking status of an employee
type EmployeeTimeStatus struct {
	EmployeeID    string     `json:"employee_id"`
	EmployeeName  string     `json:"employee_name"`
	Status        string     `json:"status"` // clocked_out, clocked_in, on_break
	TimeEntryID   *string    `json:"time_entry_id,omitempty"`
	ClockIn       *time.Time `json:"clock_in,omitempty"`
	BreakStart    *time.Time `json:"break_start,omitempty"`
	TodayMinutes  int        `json:"today_minutes"`
	WeekMinutes   int        `json:"week_minutes"`
}

// TimePeriodSummary represents time tracking summary for a period
type TimePeriodSummary struct {
	EmployeeID         string       `json:"employee_id"`
	EmployeeName       string       `json:"employee_name"`
	StartDate          time.Time    `json:"start_date"`
	EndDate            time.Time    `json:"end_date"`
	TotalWorkMinutes   int          `json:"total_work_minutes"`
	TotalBreakMinutes  int          `json:"total_break_minutes"`
	TotalDaysWorked    int          `json:"total_days_worked"`
	AverageDailyHours  float64      `json:"average_daily_hours"`
	Entries            []*TimeEntry `json:"entries"`
}

// TimeTrackingRepository handles time tracking persistence
type TimeTrackingRepository struct {
	db *database.DB
}

// NewTimeTrackingRepository creates a new time tracking repository
func NewTimeTrackingRepository(db *database.DB) *TimeTrackingRepository {
	return &TimeTrackingRepository{db: db}
}

// ============================================================================
// TIME ENTRIES
// ============================================================================

// CreateEntry creates a new time entry
// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *TimeTrackingRepository) CreateEntry(ctx context.Context, entry *TimeEntry) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO time_entries (
				id, employee_id, entry_date, clock_in, clock_out,
				total_work_minutes, total_break_minutes, notes, is_manual_entry, created_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			entry.ID, entry.EmployeeID, entry.EntryDate, entry.ClockIn, entry.ClockOut,
			entry.TotalWorkMinutes, entry.TotalBreakMinutes, entry.Notes, entry.IsManualEntry, entry.CreatedBy,
		).Scan(&entry.CreatedAt, &entry.UpdatedAt)
	})
}

// GetEntryByID gets a time entry by ID
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) GetEntryByID(ctx context.Context, id string) (*TimeEntry, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var entry TimeEntry

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT te.id, te.employee_id, te.entry_date, te.clock_in, te.clock_out,
			       te.total_work_minutes, te.total_break_minutes, te.notes, te.is_manual_entry,
			       te.created_at, te.updated_at, te.created_by, te.updated_by,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name
			FROM time_entries te
			LEFT JOIN employees e ON te.employee_id = e.id
			WHERE te.id = $1 AND te.deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &entry, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("time_entry")
	}
	if err != nil {
		return nil, err
	}

	return &entry, nil
}

// GetActiveEntryByEmployeeID gets the active (not clocked out) time entry for an employee
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) GetActiveEntryByEmployeeID(ctx context.Context, employeeID string) (*TimeEntry, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var entry TimeEntry

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, employee_id, entry_date, clock_in, clock_out,
			       total_work_minutes, total_break_minutes, notes, is_manual_entry,
			       created_at, updated_at, created_by, updated_by
			FROM time_entries
			WHERE employee_id = $1 AND clock_out IS NULL AND deleted_at IS NULL
			ORDER BY clock_in DESC
			LIMIT 1
		`
		return r.db.GetContext(ctx, &entry, query, employeeID)
	})

	if err == sql.ErrNoRows {
		return nil, nil // No active entry is not an error
	}
	if err != nil {
		return nil, err
	}

	return &entry, nil
}

// GetEntryByEmployeeAndDate gets a time entry for an employee on a specific date
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) GetEntryByEmployeeAndDate(ctx context.Context, employeeID string, date time.Time) (*TimeEntry, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var entry TimeEntry

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, employee_id, entry_date, clock_in, clock_out,
			       total_work_minutes, total_break_minutes, notes, is_manual_entry,
			       created_at, updated_at, created_by, updated_by
			FROM time_entries
			WHERE employee_id = $1 AND entry_date = $2 AND deleted_at IS NULL
			ORDER BY clock_in DESC
			LIMIT 1
		`
		return r.db.GetContext(ctx, &entry, query, employeeID, date)
	})

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &entry, nil
}

// UpdateEntry updates a time entry
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *TimeTrackingRepository) UpdateEntry(ctx context.Context, entry *TimeEntry) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE time_entries SET
				clock_in = $2, clock_out = $3, total_work_minutes = $4, total_break_minutes = $5,
				notes = $6, is_manual_entry = $7, updated_by = $8
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			entry.ID, entry.ClockIn, entry.ClockOut, entry.TotalWorkMinutes, entry.TotalBreakMinutes,
			entry.Notes, entry.IsManualEntry, entry.UpdatedBy,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("time_entry")
		}

		return nil
	})
}

// SoftDeleteEntry soft deletes a time entry
// TENANT-ISOLATED: Soft deletes only in the tenant's schema
func (r *TimeTrackingRepository) SoftDeleteEntry(ctx context.Context, id string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `UPDATE time_entries SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("time_entry")
		}

		return nil
	})
}

// ListEntriesByDate gets all time entries for a specific date
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) ListEntriesByDate(ctx context.Context, date time.Time) ([]*TimeEntry, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var entries []*TimeEntry

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT te.id, te.employee_id, te.entry_date, te.clock_in, te.clock_out,
			       te.total_work_minutes, te.total_break_minutes, te.notes, te.is_manual_entry,
			       te.created_at, te.updated_at, te.created_by, te.updated_by,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name
			FROM time_entries te
			LEFT JOIN employees e ON te.employee_id = e.id
			WHERE te.entry_date = $1 AND te.deleted_at IS NULL
			ORDER BY te.clock_in
		`
		return r.db.SelectContext(ctx, &entries, query, date)
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// ListEntriesForEmployee gets time entries for an employee within a date range
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) ListEntriesForEmployee(ctx context.Context, employeeID string, startDate, endDate time.Time) ([]*TimeEntry, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var entries []*TimeEntry

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, employee_id, entry_date, clock_in, clock_out,
			       total_work_minutes, total_break_minutes, notes, is_manual_entry,
			       created_at, updated_at, created_by, updated_by
			FROM time_entries
			WHERE employee_id = $1 AND entry_date >= $2 AND entry_date <= $3 AND deleted_at IS NULL
			ORDER BY entry_date, clock_in
		`
		return r.db.SelectContext(ctx, &entries, query, employeeID, startDate, endDate)
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// ============================================================================
// TIME BREAKS
// ============================================================================

// CreateBreak creates a new time break
// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *TimeTrackingRepository) CreateBreak(ctx context.Context, brk *TimeBreak) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if brk.ID == "" {
		brk.ID = uuid.New().String()
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO time_breaks (id, time_entry_id, start_time, end_time)
			VALUES ($1, $2, $3, $4)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			brk.ID, brk.TimeEntryID, brk.StartTime, brk.EndTime,
		).Scan(&brk.CreatedAt, &brk.UpdatedAt)
	})
}

// GetActiveBreak gets the active (not ended) break for a time entry
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) GetActiveBreak(ctx context.Context, timeEntryID string) (*TimeBreak, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var brk TimeBreak

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, time_entry_id, start_time, end_time, created_at, updated_at
			FROM time_breaks
			WHERE time_entry_id = $1 AND end_time IS NULL
			ORDER BY start_time DESC
			LIMIT 1
		`
		return r.db.GetContext(ctx, &brk, query, timeEntryID)
	})

	if err == sql.ErrNoRows {
		return nil, nil // No active break is not an error
	}
	if err != nil {
		return nil, err
	}

	return &brk, nil
}

// UpdateBreak updates a time break
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *TimeTrackingRepository) UpdateBreak(ctx context.Context, brk *TimeBreak) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE time_breaks SET end_time = $2
			WHERE id = $1
		`
		result, err := r.db.ExecContext(ctx, query, brk.ID, brk.EndTime)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("time_break")
		}

		return nil
	})
}

// ListBreaksForEntry gets all breaks for a time entry
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) ListBreaksForEntry(ctx context.Context, timeEntryID string) ([]*TimeBreak, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var breaks []*TimeBreak

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, time_entry_id, start_time, end_time, created_at, updated_at
			FROM time_breaks
			WHERE time_entry_id = $1
			ORDER BY start_time
		`
		return r.db.SelectContext(ctx, &breaks, query, timeEntryID)
	})

	if err != nil {
		return nil, err
	}

	return breaks, nil
}

// CalculateTotalBreakMinutes calculates total break minutes for a time entry
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) CalculateTotalBreakMinutes(ctx context.Context, timeEntryID string) (int, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return 0, err
	}

	var totalMinutes int

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT COALESCE(SUM(
				EXTRACT(EPOCH FROM (COALESCE(end_time, NOW()) - start_time)) / 60
			)::INTEGER, 0)
			FROM time_breaks
			WHERE time_entry_id = $1
		`
		return r.db.GetContext(ctx, &totalMinutes, query, timeEntryID)
	})

	if err != nil {
		return 0, err
	}

	return totalMinutes, nil
}

// ============================================================================
// TIME CORRECTIONS
// ============================================================================

// CreateCorrection creates a new time correction
// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *TimeTrackingRepository) CreateCorrection(ctx context.Context, corr *TimeCorrection) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if corr.ID == "" {
		corr.ID = uuid.New().String()
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO time_corrections (
				id, employee_id, time_entry_id, correction_date,
				original_clock_in, original_clock_out, corrected_clock_in, corrected_clock_out,
				reason, corrected_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			corr.ID, corr.EmployeeID, corr.TimeEntryID, corr.CorrectionDate,
			corr.OriginalClockIn, corr.OriginalClockOut, corr.CorrectedClockIn, corr.CorrectedClockOut,
			corr.Reason, corr.CorrectedBy,
		).Scan(&corr.CreatedAt, &corr.UpdatedAt)
	})
}

// ListCorrectionsForEmployee gets corrections for an employee within a date range
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) ListCorrectionsForEmployee(ctx context.Context, employeeID string, startDate, endDate time.Time) ([]*TimeCorrection, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var corrections []*TimeCorrection

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT tc.id, tc.employee_id, tc.time_entry_id, tc.correction_date,
			       tc.original_clock_in, tc.original_clock_out, tc.corrected_clock_in, tc.corrected_clock_out,
			       tc.reason, tc.corrected_by, tc.created_at, tc.updated_at,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name
			FROM time_corrections tc
			LEFT JOIN employees e ON tc.employee_id = e.id
			WHERE tc.employee_id = $1 AND tc.correction_date >= $2 AND tc.correction_date <= $3 AND tc.deleted_at IS NULL
			ORDER BY tc.correction_date DESC, tc.created_at DESC
		`
		return r.db.SelectContext(ctx, &corrections, query, employeeID, startDate, endDate)
	})

	if err != nil {
		return nil, err
	}

	return corrections, nil
}

// ============================================================================
// STATUS AND SUMMARY QUERIES
// ============================================================================

// GetAllEmployeeStatuses gets the time tracking status for all employees
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) GetAllEmployeeStatuses(ctx context.Context) ([]*EmployeeTimeStatus, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var statuses []*EmployeeTimeStatus

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Get all active employees
		query := `
			SELECT
				e.id as employee_id,
				CONCAT(e.first_name, ' ', e.last_name) as employee_name,
				te.id as time_entry_id,
				te.clock_in,
				tb.start_time as break_start,
				COALESCE(today.total_work_minutes, 0) as today_minutes,
				COALESCE(week.total_work_minutes, 0) as week_minutes
			FROM employees e
			LEFT JOIN time_entries te ON e.id = te.employee_id
				AND te.clock_out IS NULL AND te.deleted_at IS NULL
			LEFT JOIN time_breaks tb ON te.id = tb.time_entry_id AND tb.end_time IS NULL
			LEFT JOIN LATERAL (
				SELECT SUM(total_work_minutes) as total_work_minutes
				FROM time_entries
				WHERE employee_id = e.id
					AND entry_date = CURRENT_DATE
					AND deleted_at IS NULL
			) today ON true
			LEFT JOIN LATERAL (
				SELECT SUM(total_work_minutes) as total_work_minutes
				FROM time_entries
				WHERE employee_id = e.id
					AND entry_date >= date_trunc('week', CURRENT_DATE)
					AND entry_date < date_trunc('week', CURRENT_DATE) + INTERVAL '7 days'
					AND deleted_at IS NULL
			) week ON true
			WHERE e.deleted_at IS NULL AND e.status = 'active'
			ORDER BY e.last_name, e.first_name
		`

		type statusRow struct {
			EmployeeID   string     `db:"employee_id"`
			EmployeeName string     `db:"employee_name"`
			TimeEntryID  *string    `db:"time_entry_id"`
			ClockIn      *time.Time `db:"clock_in"`
			BreakStart   *time.Time `db:"break_start"`
			TodayMinutes int        `db:"today_minutes"`
			WeekMinutes  int        `db:"week_minutes"`
		}

		var rows []statusRow
		if err := r.db.SelectContext(ctx, &rows, query); err != nil {
			return err
		}

		for _, row := range rows {
			status := &EmployeeTimeStatus{
				EmployeeID:   row.EmployeeID,
				EmployeeName: row.EmployeeName,
				TimeEntryID:  row.TimeEntryID,
				ClockIn:      row.ClockIn,
				BreakStart:   row.BreakStart,
				TodayMinutes: row.TodayMinutes,
				WeekMinutes:  row.WeekMinutes,
			}

			// Determine status
			if row.TimeEntryID == nil {
				status.Status = "clocked_out"
			} else if row.BreakStart != nil {
				status.Status = "on_break"
			} else {
				status.Status = "clocked_in"
			}

			statuses = append(statuses, status)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return statuses, nil
}

// GetEmployeeTimeSummary gets a time summary for an employee within a date range
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) GetEmployeeTimeSummary(ctx context.Context, employeeID string, startDate, endDate time.Time) (*TimePeriodSummary, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	summary := &TimePeriodSummary{
		EmployeeID: employeeID,
		StartDate:  startDate,
		EndDate:    endDate,
	}

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Get employee name
		var empName string
		nameQuery := `SELECT CONCAT(first_name, ' ', last_name) FROM employees WHERE id = $1`
		if err := r.db.GetContext(ctx, &empName, nameQuery, employeeID); err != nil {
			if err == sql.ErrNoRows {
				return errors.NotFound("employee")
			}
			return err
		}
		summary.EmployeeName = empName

		// Get time entries
		entries, err := r.ListEntriesForEmployee(ctx, employeeID, startDate, endDate)
		if err != nil {
			return err
		}
		summary.Entries = entries

		// Calculate totals
		for _, entry := range entries {
			summary.TotalWorkMinutes += entry.TotalWorkMinutes
			summary.TotalBreakMinutes += entry.TotalBreakMinutes
		}
		summary.TotalDaysWorked = len(entries)

		if summary.TotalDaysWorked > 0 {
			summary.AverageDailyHours = float64(summary.TotalWorkMinutes) / float64(summary.TotalDaysWorked) / 60.0
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return summary, nil
}

// GetTotalWorkMinutesForWeek gets total work minutes for an employee in the current week
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) GetTotalWorkMinutesForWeek(ctx context.Context, employeeID string) (int, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return 0, err
	}

	var totalMinutes int

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT COALESCE(SUM(total_work_minutes), 0)
			FROM time_entries
			WHERE employee_id = $1
				AND entry_date >= date_trunc('week', CURRENT_DATE)
				AND entry_date < date_trunc('week', CURRENT_DATE) + INTERVAL '7 days'
				AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &totalMinutes, query, employeeID)
	})

	if err != nil {
		return 0, err
	}

	return totalMinutes, nil
}

// CheckEmployeeExists verifies an employee exists
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *TimeTrackingRepository) CheckEmployeeExists(ctx context.Context, employeeID string) (bool, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return false, err
	}

	var exists bool

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `SELECT EXISTS(SELECT 1 FROM employees WHERE id = $1 AND deleted_at IS NULL)`
		return r.db.GetContext(ctx, &exists, query, employeeID)
	})

	if err != nil {
		return false, fmt.Errorf("failed to check employee existence: %w", err)
	}

	return exists, nil
}
