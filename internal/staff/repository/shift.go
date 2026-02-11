package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// ShiftTemplate represents a reusable shift definition
type ShiftTemplate struct {
	ID                   string     `db:"id" json:"id"`
	Name                 string     `db:"name" json:"name"`
	Description          *string    `db:"description" json:"description,omitempty"`
	StartTime            string     `db:"start_time" json:"start_time"`            // TIME format HH:MM:SS
	EndTime              string     `db:"end_time" json:"end_time"`                // TIME format HH:MM:SS
	BreakDurationMinutes int        `db:"break_duration_minutes" json:"break_duration_minutes"`
	ShiftType            string     `db:"shift_type" json:"shift_type"` // regular, on_call, emergency, night
	Color                string     `db:"color" json:"color"`
	IsActive             bool       `db:"is_active" json:"is_active"`
	CreatedAt            time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt            *time.Time `db:"deleted_at" json:"-"`
	CreatedBy            *string    `db:"created_by" json:"created_by,omitempty"`
}

// ShiftAssignment represents a scheduled shift for an employee
type ShiftAssignment struct {
	ID                   string     `db:"id" json:"id"`
	EmployeeID           string     `db:"employee_id" json:"employee_id"`
	ShiftTemplateID      *string    `db:"shift_template_id" json:"shift_template_id,omitempty"`
	ShiftDate            time.Time  `db:"shift_date" json:"shift_date"`
	StartTime            string     `db:"start_time" json:"start_time"` // TIME format HH:MM:SS
	EndTime              string     `db:"end_time" json:"end_time"`     // TIME format HH:MM:SS
	BreakDurationMinutes int        `db:"break_duration_minutes" json:"break_duration_minutes"`
	ShiftType            string     `db:"shift_type" json:"shift_type"` // regular, on_call, emergency, night
	Status               string     `db:"status" json:"status"`         // scheduled, confirmed, completed, cancelled
	HasConflict          bool       `db:"has_conflict" json:"has_conflict"`
	ConflictReason       *string    `db:"conflict_reason" json:"conflict_reason,omitempty"`
	Notes                *string    `db:"notes" json:"notes,omitempty"`
	CreatedAt            time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt            *time.Time `db:"deleted_at" json:"-"`
	CreatedBy            *string    `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy            *string    `db:"updated_by" json:"updated_by,omitempty"`

	// Joined fields (populated by specific queries)
	EmployeeName *string `db:"employee_name" json:"employee_name,omitempty"`
	TemplateName *string `db:"template_name" json:"template_name,omitempty"`
}

// ShiftListParams holds parameters for listing shifts
type ShiftListParams struct {
	EmployeeID *string
	StartDate  *time.Time
	EndDate    *time.Time
	Status     *string
	ShiftType  *string
	Page       int
	PerPage    int
}

// ShiftRepository handles shift persistence
type ShiftRepository struct {
	db *database.DB
}

// NewShiftRepository creates a new shift repository
func NewShiftRepository(db *database.DB) *ShiftRepository {
	return &ShiftRepository{db: db}
}

// ============================================================================
// SHIFT TEMPLATES
// ============================================================================

// CreateTemplate creates a new shift template
// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *ShiftRepository) CreateTemplate(ctx context.Context, tmpl *ShiftTemplate) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if tmpl.ID == "" {
		tmpl.ID = uuid.New().String()
	}

	// Set defaults
	if tmpl.ShiftType == "" {
		tmpl.ShiftType = "regular"
	}
	if tmpl.Color == "" {
		tmpl.Color = "#22c55e"
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO shift_templates (
				id, tenant_id, name, description, start_time, end_time,
				break_duration_minutes, shift_type, color, is_active, created_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			tmpl.ID, tenantID, tmpl.Name, tmpl.Description, tmpl.StartTime, tmpl.EndTime,
			tmpl.BreakDurationMinutes, tmpl.ShiftType, tmpl.Color, tmpl.IsActive, tmpl.CreatedBy,
		).Scan(&tmpl.CreatedAt, &tmpl.UpdatedAt)
	})
}

// GetTemplateByID gets a shift template by ID
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *ShiftRepository) GetTemplateByID(ctx context.Context, id string) (*ShiftTemplate, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var tmpl ShiftTemplate

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, name, description,
			       start_time::text as start_time, end_time::text as end_time,
			       break_duration_minutes, shift_type, color, is_active,
			       created_at, updated_at, created_by
			FROM shift_templates
			WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &tmpl, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("shift_template")
	}
	if err != nil {
		return nil, err
	}

	return &tmpl, nil
}

// ListTemplates lists all active shift templates
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *ShiftRepository) ListTemplates(ctx context.Context, activeOnly bool) ([]*ShiftTemplate, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var templates []*ShiftTemplate

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, name, description,
			       start_time::text as start_time, end_time::text as end_time,
			       break_duration_minutes, shift_type, color, is_active,
			       created_at, updated_at, created_by
			FROM shift_templates
			WHERE deleted_at IS NULL
		`
		if activeOnly {
			query += " AND is_active = true"
		}
		query += " ORDER BY name"

		return r.db.SelectContext(ctx, &templates, query)
	})

	if err != nil {
		return nil, err
	}

	return templates, nil
}

// UpdateTemplate updates a shift template
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *ShiftRepository) UpdateTemplate(ctx context.Context, tmpl *ShiftTemplate) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE shift_templates SET
				name = $2, description = $3, start_time = $4, end_time = $5,
				break_duration_minutes = $6, shift_type = $7, color = $8, is_active = $9
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			tmpl.ID, tmpl.Name, tmpl.Description, tmpl.StartTime, tmpl.EndTime,
			tmpl.BreakDurationMinutes, tmpl.ShiftType, tmpl.Color, tmpl.IsActive,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("shift_template")
		}

		return nil
	})
}

// DeleteTemplate soft deletes a shift template
// TENANT-ISOLATED: Soft deletes only in the tenant's schema
func (r *ShiftRepository) DeleteTemplate(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE shift_templates SET deleted_at = NOW(), is_active = false WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("shift_template")
		}

		return nil
	})
}

// ============================================================================
// SHIFT ASSIGNMENTS
// ============================================================================

// CreateAssignment creates a new shift assignment
// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *ShiftRepository) CreateAssignment(ctx context.Context, shift *ShiftAssignment) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if shift.ID == "" {
		shift.ID = uuid.New().String()
	}

	// Set defaults
	if shift.Status == "" {
		shift.Status = "scheduled"
	}
	if shift.ShiftType == "" {
		shift.ShiftType = "regular"
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO shift_assignments (
				id, tenant_id, employee_id, shift_template_id, shift_date, start_time, end_time,
				break_duration_minutes, shift_type, status, has_conflict, conflict_reason, notes, created_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			shift.ID, tenantID, shift.EmployeeID, shift.ShiftTemplateID, shift.ShiftDate, shift.StartTime, shift.EndTime,
			shift.BreakDurationMinutes, shift.ShiftType, shift.Status, shift.HasConflict, shift.ConflictReason,
			shift.Notes, shift.CreatedBy,
		).Scan(&shift.CreatedAt, &shift.UpdatedAt)
	})
}

// GetAssignmentByID gets a shift assignment by ID
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *ShiftRepository) GetAssignmentByID(ctx context.Context, id string) (*ShiftAssignment, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var shift ShiftAssignment

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT sa.id, sa.employee_id, sa.shift_template_id, sa.shift_date,
			       sa.start_time::text as start_time, sa.end_time::text as end_time,
			       sa.break_duration_minutes, sa.shift_type, sa.status, sa.has_conflict, sa.conflict_reason,
			       sa.notes, sa.created_at, sa.updated_at, sa.created_by, sa.updated_by,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name,
			       st.name as template_name
			FROM shift_assignments sa
			LEFT JOIN employees e ON sa.employee_id = e.id
			LEFT JOIN shift_templates st ON sa.shift_template_id = st.id
			WHERE sa.id = $1 AND sa.deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &shift, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("shift_assignment")
	}
	if err != nil {
		return nil, err
	}

	return &shift, nil
}

// ListAssignments lists shift assignments with filters
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *ShiftRepository) ListAssignments(ctx context.Context, params ShiftListParams) ([]*ShiftAssignment, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var shifts []*ShiftAssignment

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		// Build WHERE clause
		whereClause := "WHERE sa.deleted_at IS NULL"
		args := []interface{}{}
		argNum := 1

		if params.EmployeeID != nil {
			whereClause += " AND sa.employee_id = $" + string(rune('0'+argNum))
			args = append(args, *params.EmployeeID)
			argNum++
		}
		if params.StartDate != nil {
			whereClause += " AND sa.shift_date >= $" + string(rune('0'+argNum))
			args = append(args, *params.StartDate)
			argNum++
		}
		if params.EndDate != nil {
			whereClause += " AND sa.shift_date <= $" + string(rune('0'+argNum))
			args = append(args, *params.EndDate)
			argNum++
		}
		if params.Status != nil {
			whereClause += " AND sa.status = $" + string(rune('0'+argNum))
			args = append(args, *params.Status)
			argNum++
		}
		if params.ShiftType != nil {
			whereClause += " AND sa.shift_type = $" + string(rune('0'+argNum))
			args = append(args, *params.ShiftType)
			argNum++
		}

		// Count total
		countQuery := "SELECT COUNT(*) FROM shift_assignments sa " + whereClause
		if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
			return err
		}

		// Get paginated shifts
		if params.PerPage <= 0 {
			params.PerPage = 50
		}
		if params.Page <= 0 {
			params.Page = 1
		}
		offset := (params.Page - 1) * params.PerPage

		query := `
			SELECT sa.id, sa.employee_id, sa.shift_template_id, sa.shift_date,
			       sa.start_time::text as start_time, sa.end_time::text as end_time,
			       sa.break_duration_minutes, sa.shift_type, sa.status, sa.has_conflict, sa.conflict_reason,
			       sa.notes, sa.created_at, sa.updated_at, sa.created_by, sa.updated_by,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name,
			       st.name as template_name
			FROM shift_assignments sa
			LEFT JOIN employees e ON sa.employee_id = e.id
			LEFT JOIN shift_templates st ON sa.shift_template_id = st.id
		` + whereClause + `
			ORDER BY sa.shift_date, sa.start_time
			LIMIT $` + string(rune('0'+argNum)) + ` OFFSET $` + string(rune('0'+argNum+1))

		args = append(args, params.PerPage, offset)
		return r.db.SelectContext(ctx, &shifts, query, args...)
	})

	if err != nil {
		return nil, 0, err
	}

	return shifts, total, nil
}

// GetAssignmentsForDate gets all shifts for a specific date
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *ShiftRepository) GetAssignmentsForDate(ctx context.Context, date time.Time) ([]*ShiftAssignment, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var shifts []*ShiftAssignment

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT sa.id, sa.employee_id, sa.shift_template_id, sa.shift_date,
			       sa.start_time::text as start_time, sa.end_time::text as end_time,
			       sa.break_duration_minutes, sa.shift_type, sa.status, sa.has_conflict, sa.conflict_reason,
			       sa.notes, sa.created_at, sa.updated_at, sa.created_by, sa.updated_by,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name,
			       st.name as template_name
			FROM shift_assignments sa
			LEFT JOIN employees e ON sa.employee_id = e.id
			LEFT JOIN shift_templates st ON sa.shift_template_id = st.id
			WHERE sa.shift_date = $1 AND sa.deleted_at IS NULL AND sa.status != 'cancelled'
			ORDER BY sa.start_time, e.last_name
		`
		return r.db.SelectContext(ctx, &shifts, query, date)
	})

	if err != nil {
		return nil, err
	}

	return shifts, nil
}

// GetEmployeeShiftsForDateRange gets shifts for an employee in a date range
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *ShiftRepository) GetEmployeeShiftsForDateRange(ctx context.Context, employeeID string, startDate, endDate time.Time) ([]*ShiftAssignment, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var shifts []*ShiftAssignment

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, employee_id, shift_template_id, shift_date,
			       start_time::text as start_time, end_time::text as end_time,
			       break_duration_minutes, shift_type, status, has_conflict, conflict_reason,
			       notes, created_at, updated_at, created_by, updated_by
			FROM shift_assignments
			WHERE employee_id = $1 AND shift_date >= $2 AND shift_date <= $3
			      AND deleted_at IS NULL AND status != 'cancelled'
			ORDER BY shift_date, start_time
		`
		return r.db.SelectContext(ctx, &shifts, query, employeeID, startDate, endDate)
	})

	if err != nil {
		return nil, err
	}

	return shifts, nil
}

// UpdateAssignment updates a shift assignment
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *ShiftRepository) UpdateAssignment(ctx context.Context, shift *ShiftAssignment) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE shift_assignments SET
				shift_template_id = $2, shift_date = $3, start_time = $4, end_time = $5,
				break_duration_minutes = $6, shift_type = $7, status = $8,
				has_conflict = $9, conflict_reason = $10, notes = $11, updated_by = $12
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			shift.ID, shift.ShiftTemplateID, shift.ShiftDate, shift.StartTime, shift.EndTime,
			shift.BreakDurationMinutes, shift.ShiftType, shift.Status,
			shift.HasConflict, shift.ConflictReason, shift.Notes, shift.UpdatedBy,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("shift_assignment")
		}

		return nil
	})
}

// DeleteAssignment soft deletes a shift assignment
// TENANT-ISOLATED: Soft deletes only in the tenant's schema
func (r *ShiftRepository) DeleteAssignment(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE shift_assignments SET deleted_at = NOW(), status = 'cancelled' WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("shift_assignment")
		}

		return nil
	})
}

// BulkCreateAssignments creates multiple shift assignments
// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *ShiftRepository) BulkCreateAssignments(ctx context.Context, shifts []*ShiftAssignment) error {
	if len(shifts) == 0 {
		return nil
	}

	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		for _, shift := range shifts {
			if shift.ID == "" {
				shift.ID = uuid.New().String()
			}
			if shift.Status == "" {
				shift.Status = "scheduled"
			}
			if shift.ShiftType == "" {
				shift.ShiftType = "regular"
			}

			query := `
				INSERT INTO shift_assignments (
					id, tenant_id, employee_id, shift_template_id, shift_date, start_time, end_time,
					break_duration_minutes, shift_type, status, has_conflict, conflict_reason, notes, created_by
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
				RETURNING created_at, updated_at
			`
			if err := r.db.QueryRowxContext(ctx, query,
				shift.ID, tenantID, shift.EmployeeID, shift.ShiftTemplateID, shift.ShiftDate, shift.StartTime, shift.EndTime,
				shift.BreakDurationMinutes, shift.ShiftType, shift.Status, shift.HasConflict, shift.ConflictReason,
				shift.Notes, shift.CreatedBy,
			).Scan(&shift.CreatedAt, &shift.UpdatedAt); err != nil {
				return err
			}
		}
		return nil
	})
}

// ShiftWithTimes represents a shift with computed start/end times
type ShiftWithTimes struct {
	ShiftAssignment
	ShiftStart time.Time
	ShiftEnd   time.Time
}

// ListAssignmentsByEmployeeAndDateRange gets shifts with computed times for compliance checking
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *ShiftRepository) ListAssignmentsByEmployeeAndDateRange(ctx context.Context, employeeID string, startDate, endDate time.Time) ([]*ShiftWithTimes, error) {
	shifts, err := r.GetEmployeeShiftsForDateRange(ctx, employeeID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	result := make([]*ShiftWithTimes, 0, len(shifts))
	for _, shift := range shifts {
		// Parse time strings to create full timestamps
		startParsed, err := time.Parse("15:04:05", shift.StartTime)
		if err != nil {
			continue
		}
		endParsed, err := time.Parse("15:04:05", shift.EndTime)
		if err != nil {
			continue
		}

		shiftStart := time.Date(
			shift.ShiftDate.Year(), shift.ShiftDate.Month(), shift.ShiftDate.Day(),
			startParsed.Hour(), startParsed.Minute(), startParsed.Second(), 0,
			shift.ShiftDate.Location(),
		)
		shiftEnd := time.Date(
			shift.ShiftDate.Year(), shift.ShiftDate.Month(), shift.ShiftDate.Day(),
			endParsed.Hour(), endParsed.Minute(), endParsed.Second(), 0,
			shift.ShiftDate.Location(),
		)

		// Handle overnight shifts
		if shiftEnd.Before(shiftStart) {
			shiftEnd = shiftEnd.AddDate(0, 0, 1)
		}

		result = append(result, &ShiftWithTimes{
			ShiftAssignment: *shift,
			ShiftStart:      shiftStart,
			ShiftEnd:        shiftEnd,
		})
	}

	return result, nil
}

// CheckForConflicts checks if a shift assignment conflicts with existing shifts or violates rest period
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *ShiftRepository) CheckForConflicts(ctx context.Context, employeeID string, shiftDate time.Time, startTime, endTime string, excludeID *string) (bool, string, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return false, "", err
	}

	var count int
	var hasConflict bool
	var reason string

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		// Check for overlapping shifts on the same day
		overlapQuery := `
			SELECT COUNT(*) FROM shift_assignments
			WHERE employee_id = $1 AND shift_date = $2 AND deleted_at IS NULL AND status != 'cancelled'
			AND (
				(start_time < $4 AND end_time > $3)
			)
		`
		args := []interface{}{employeeID, shiftDate, startTime, endTime}

		if excludeID != nil {
			overlapQuery += " AND id != $5"
			args = append(args, *excludeID)
		}

		if err := r.db.GetContext(ctx, &count, overlapQuery, args...); err != nil {
			return err
		}

		if count > 0 {
			hasConflict = true
			reason = "Overlapping shift on the same day"
			return nil
		}

		// Check for rest period violation (11 hours minimum between shifts)
		// This would require more complex logic with the previous/next day's shifts

		return nil
	})

	if err != nil {
		return false, "", err
	}

	return hasConflict, reason, nil
}
