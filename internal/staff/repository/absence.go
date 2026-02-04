package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// Absence represents an employee absence (vacation, sick, etc.)
type Absence struct {
	ID               string     `db:"id" json:"id"`
	EmployeeID       string     `db:"employee_id" json:"employee_id"`
	StartDate        time.Time  `db:"start_date" json:"start_date"`
	EndDate          time.Time  `db:"end_date" json:"end_date"`
	AbsenceType      string     `db:"absence_type" json:"absence_type"` // vacation, sick, sick_child, training, special_leave, unpaid_leave, parental_leave, comp_time, other
	Status           string     `db:"status" json:"status"`             // pending, approved, rejected, cancelled
	RequestedAt      time.Time  `db:"requested_at" json:"requested_at"`
	ReviewedBy       *string    `db:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time `db:"reviewed_at" json:"reviewed_at,omitempty"`
	RejectionReason  *string    `db:"rejection_reason" json:"rejection_reason,omitempty"`
	VacationDaysUsed *float64   `db:"vacation_days_used" json:"vacation_days_used,omitempty"`
	EmployeeNote     *string    `db:"employee_note" json:"employee_note,omitempty"`
	ManagerNote      *string    `db:"manager_note" json:"manager_note,omitempty"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt        *time.Time `db:"deleted_at" json:"-"`
	CreatedBy        *string    `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy        *string    `db:"updated_by" json:"updated_by,omitempty"`

	// Joined fields
	EmployeeName *string `db:"employee_name" json:"employee_name,omitempty"`
}

// VacationBalance represents an employee's vacation balance for a year
type VacationBalance struct {
	ID                   string    `db:"id" json:"id"`
	EmployeeID           string    `db:"employee_id" json:"employee_id"`
	Year                 int       `db:"year" json:"year"`
	AnnualEntitlement    float64   `db:"annual_entitlement" json:"annual_entitlement"`
	CarryoverFromPrevious float64  `db:"carryover_from_previous" json:"carryover_from_previous"`
	AdditionalGranted    float64   `db:"additional_granted" json:"additional_granted"`
	Taken                float64   `db:"taken" json:"taken"`
	Planned              float64   `db:"planned" json:"planned"`
	Pending              float64   `db:"pending" json:"pending"`
	CreatedAt            time.Time `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time `db:"updated_at" json:"updated_at"`
}

// Available returns the available vacation days
func (v *VacationBalance) Available() float64 {
	return v.AnnualEntitlement + v.CarryoverFromPrevious + v.AdditionalGranted - v.Taken - v.Planned
}

// ArbzgComplianceLog represents a compliance violation log entry
type ArbzgComplianceLog struct {
	ID             string          `db:"id" json:"id"`
	EmployeeID     string          `db:"employee_id" json:"employee_id"`
	TimeEntryID    *string         `db:"time_entry_id" json:"time_entry_id,omitempty"`
	ViolationDate  time.Time       `db:"violation_date" json:"violation_date"`
	ViolationType  string          `db:"violation_type" json:"violation_type"`
	Severity       string          `db:"severity" json:"severity"` // warning, violation, critical
	Description    string          `db:"description" json:"description"`
	Details        json.RawMessage `db:"details" json:"details,omitempty"`
	AcknowledgedBy *string         `db:"acknowledged_by" json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time      `db:"acknowledged_at" json:"acknowledged_at,omitempty"`
	ResolutionNote *string         `db:"resolution_note" json:"resolution_note,omitempty"`
	CreatedAt      time.Time       `db:"created_at" json:"created_at"`

	// Joined fields
	EmployeeName *string `db:"employee_name" json:"employee_name,omitempty"`
}

// AbsenceListParams holds parameters for listing absences
type AbsenceListParams struct {
	EmployeeID  *string
	StartDate   *time.Time
	EndDate     *time.Time
	Status      *string
	AbsenceType *string
	Page        int
	PerPage     int
}

// AbsenceRepository handles absence persistence
type AbsenceRepository struct {
	db *database.DB
}

// NewAbsenceRepository creates a new absence repository
func NewAbsenceRepository(db *database.DB) *AbsenceRepository {
	return &AbsenceRepository{db: db}
}

// ============================================================================
// ABSENCES
// ============================================================================

// Create creates a new absence request
// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *AbsenceRepository) Create(ctx context.Context, absence *Absence) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if absence.ID == "" {
		absence.ID = uuid.New().String()
	}
	if absence.Status == "" {
		absence.Status = "pending"
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO absences (
				id, employee_id, start_date, end_date, absence_type, status,
				requested_at, vacation_days_used, employee_note, created_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			absence.ID, absence.EmployeeID, absence.StartDate, absence.EndDate,
			absence.AbsenceType, absence.Status, absence.RequestedAt,
			absence.VacationDaysUsed, absence.EmployeeNote, absence.CreatedBy,
		).Scan(&absence.CreatedAt, &absence.UpdatedAt)
	})
}

// GetByID gets an absence by ID
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *AbsenceRepository) GetByID(ctx context.Context, id string) (*Absence, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var absence Absence

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT a.id, a.employee_id, a.start_date, a.end_date, a.absence_type, a.status,
			       a.requested_at, a.reviewed_by, a.reviewed_at, a.rejection_reason,
			       a.vacation_days_used, a.employee_note, a.manager_note,
			       a.created_at, a.updated_at, a.created_by, a.updated_by,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name
			FROM absences a
			LEFT JOIN employees e ON a.employee_id = e.id
			WHERE a.id = $1 AND a.deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &absence, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("absence")
	}
	if err != nil {
		return nil, err
	}

	return &absence, nil
}

// List lists absences with filters
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *AbsenceRepository) List(ctx context.Context, params AbsenceListParams) ([]*Absence, int64, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var absences []*Absence

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Build WHERE clause
		whereClause := "WHERE a.deleted_at IS NULL"
		args := []interface{}{}
		argNum := 1

		if params.EmployeeID != nil {
			whereClause += " AND a.employee_id = $" + string(rune('0'+argNum))
			args = append(args, *params.EmployeeID)
			argNum++
		}
		if params.StartDate != nil {
			whereClause += " AND a.end_date >= $" + string(rune('0'+argNum))
			args = append(args, *params.StartDate)
			argNum++
		}
		if params.EndDate != nil {
			whereClause += " AND a.start_date <= $" + string(rune('0'+argNum))
			args = append(args, *params.EndDate)
			argNum++
		}
		if params.Status != nil {
			whereClause += " AND a.status = $" + string(rune('0'+argNum))
			args = append(args, *params.Status)
			argNum++
		}
		if params.AbsenceType != nil {
			whereClause += " AND a.absence_type = $" + string(rune('0'+argNum))
			args = append(args, *params.AbsenceType)
			argNum++
		}

		// Count total
		countQuery := "SELECT COUNT(*) FROM absences a " + whereClause
		if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
			return err
		}

		// Get paginated absences
		if params.PerPage <= 0 {
			params.PerPage = 20
		}
		if params.Page <= 0 {
			params.Page = 1
		}
		offset := (params.Page - 1) * params.PerPage

		query := `
			SELECT a.id, a.employee_id, a.start_date, a.end_date, a.absence_type, a.status,
			       a.requested_at, a.reviewed_by, a.reviewed_at, a.rejection_reason,
			       a.vacation_days_used, a.employee_note, a.manager_note,
			       a.created_at, a.updated_at, a.created_by, a.updated_by,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name
			FROM absences a
			LEFT JOIN employees e ON a.employee_id = e.id
		` + whereClause + `
			ORDER BY a.start_date DESC
			LIMIT $` + string(rune('0'+argNum)) + ` OFFSET $` + string(rune('0'+argNum+1))

		args = append(args, params.PerPage, offset)
		return r.db.SelectContext(ctx, &absences, query, args...)
	})

	if err != nil {
		return nil, 0, err
	}

	return absences, total, nil
}

// ListPending lists all pending absence requests
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *AbsenceRepository) ListPending(ctx context.Context) ([]*Absence, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var absences []*Absence

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT a.id, a.employee_id, a.start_date, a.end_date, a.absence_type, a.status,
			       a.requested_at, a.reviewed_by, a.reviewed_at, a.rejection_reason,
			       a.vacation_days_used, a.employee_note, a.manager_note,
			       a.created_at, a.updated_at, a.created_by, a.updated_by,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name
			FROM absences a
			LEFT JOIN employees e ON a.employee_id = e.id
			WHERE a.status = 'pending' AND a.deleted_at IS NULL
			ORDER BY a.requested_at
		`
		return r.db.SelectContext(ctx, &absences, query)
	})

	if err != nil {
		return nil, err
	}

	return absences, nil
}

// Update updates an absence
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *AbsenceRepository) Update(ctx context.Context, absence *Absence) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE absences SET
				start_date = $2, end_date = $3, absence_type = $4, status = $5,
				reviewed_by = $6, reviewed_at = $7, rejection_reason = $8,
				vacation_days_used = $9, employee_note = $10, manager_note = $11, updated_by = $12
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			absence.ID, absence.StartDate, absence.EndDate, absence.AbsenceType, absence.Status,
			absence.ReviewedBy, absence.ReviewedAt, absence.RejectionReason,
			absence.VacationDaysUsed, absence.EmployeeNote, absence.ManagerNote, absence.UpdatedBy,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("absence")
		}

		return nil
	})
}

// Approve approves an absence request
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *AbsenceRepository) Approve(ctx context.Context, id string, reviewedBy string, managerNote *string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE absences SET
				status = 'approved', reviewed_by = $2, reviewed_at = NOW(), manager_note = $3
			WHERE id = $1 AND status = 'pending' AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query, id, reviewedBy, managerNote)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("absence")
		}

		return nil
	})
}

// Reject rejects an absence request
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *AbsenceRepository) Reject(ctx context.Context, id string, reviewedBy string, reason string, managerNote *string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE absences SET
				status = 'rejected', reviewed_by = $2, reviewed_at = NOW(),
				rejection_reason = $3, manager_note = $4
			WHERE id = $1 AND status = 'pending' AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query, id, reviewedBy, reason, managerNote)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("absence")
		}

		return nil
	})
}

// Delete soft deletes an absence
// TENANT-ISOLATED: Soft deletes only in the tenant's schema
func (r *AbsenceRepository) Delete(ctx context.Context, id string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `UPDATE absences SET deleted_at = NOW(), status = 'cancelled' WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("absence")
		}

		return nil
	})
}

// GetAbsencesForDateRange gets absences that overlap with a date range
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *AbsenceRepository) GetAbsencesForDateRange(ctx context.Context, employeeID string, startDate, endDate time.Time) ([]*Absence, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var absences []*Absence

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, employee_id, start_date, end_date, absence_type, status,
			       requested_at, reviewed_by, reviewed_at, rejection_reason,
			       vacation_days_used, employee_note, manager_note,
			       created_at, updated_at, created_by, updated_by
			FROM absences
			WHERE employee_id = $1 AND start_date <= $3 AND end_date >= $2
			      AND deleted_at IS NULL AND status IN ('pending', 'approved')
			ORDER BY start_date
		`
		return r.db.SelectContext(ctx, &absences, query, employeeID, startDate, endDate)
	})

	if err != nil {
		return nil, err
	}

	return absences, nil
}

// ============================================================================
// VACATION BALANCE
// ============================================================================

// GetVacationBalance gets vacation balance for an employee for a year
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *AbsenceRepository) GetVacationBalance(ctx context.Context, employeeID string, year int) (*VacationBalance, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var balance VacationBalance

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, employee_id, year, annual_entitlement, carryover_from_previous,
			       additional_granted, taken, planned, pending, created_at, updated_at
			FROM vacation_balances
			WHERE employee_id = $1 AND year = $2
		`
		return r.db.GetContext(ctx, &balance, query, employeeID, year)
	})

	if err == sql.ErrNoRows {
		return nil, nil // No balance for this year is not an error
	}
	if err != nil {
		return nil, err
	}

	return &balance, nil
}

// CreateOrUpdateVacationBalance creates or updates vacation balance
// TENANT-ISOLATED: Inserts/updates only in the tenant's schema
func (r *AbsenceRepository) CreateOrUpdateVacationBalance(ctx context.Context, balance *VacationBalance) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if balance.ID == "" {
		balance.ID = uuid.New().String()
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO vacation_balances (
				id, employee_id, year, annual_entitlement, carryover_from_previous,
				additional_granted, taken, planned, pending
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (employee_id, year)
			DO UPDATE SET
				annual_entitlement = $4, carryover_from_previous = $5,
				additional_granted = $6, taken = $7, planned = $8, pending = $9,
				updated_at = NOW()
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			balance.ID, balance.EmployeeID, balance.Year, balance.AnnualEntitlement,
			balance.CarryoverFromPrevious, balance.AdditionalGranted,
			balance.Taken, balance.Planned, balance.Pending,
		).Scan(&balance.CreatedAt, &balance.UpdatedAt)
	})
}

// UpdateVacationUsage updates the vacation usage fields
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *AbsenceRepository) UpdateVacationUsage(ctx context.Context, employeeID string, year int, taken, planned, pending float64) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE vacation_balances SET
				taken = $3, planned = $4, pending = $5, updated_at = NOW()
			WHERE employee_id = $1 AND year = $2
		`
		result, err := r.db.ExecContext(ctx, query, employeeID, year, taken, planned, pending)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("vacation_balance")
		}

		return nil
	})
}

// ============================================================================
// ARBZG COMPLIANCE LOG
// ============================================================================

// CreateComplianceLog creates a new compliance log entry
// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *AbsenceRepository) CreateComplianceLog(ctx context.Context, log *ArbzgComplianceLog) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if log.ID == "" {
		log.ID = uuid.New().String()
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO arbzg_compliance_log (
				id, employee_id, time_entry_id, violation_date, violation_type,
				severity, description, details
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at
		`
		return r.db.QueryRowxContext(ctx, query,
			log.ID, log.EmployeeID, log.TimeEntryID, log.ViolationDate,
			log.ViolationType, log.Severity, log.Description, log.Details,
		).Scan(&log.CreatedAt)
	})
}

// ListComplianceLogs lists compliance violations with filters
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *AbsenceRepository) ListComplianceLogs(ctx context.Context, employeeID *string, startDate, endDate *time.Time, unacknowledgedOnly bool, page, perPage int) ([]*ArbzgComplianceLog, int64, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var logs []*ArbzgComplianceLog

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Build WHERE clause
		whereClause := "WHERE 1=1"
		args := []interface{}{}
		argNum := 1

		if employeeID != nil {
			whereClause += " AND c.employee_id = $" + string(rune('0'+argNum))
			args = append(args, *employeeID)
			argNum++
		}
		if startDate != nil {
			whereClause += " AND c.violation_date >= $" + string(rune('0'+argNum))
			args = append(args, *startDate)
			argNum++
		}
		if endDate != nil {
			whereClause += " AND c.violation_date <= $" + string(rune('0'+argNum))
			args = append(args, *endDate)
			argNum++
		}
		if unacknowledgedOnly {
			whereClause += " AND c.acknowledged_at IS NULL"
		}

		// Count total
		countQuery := "SELECT COUNT(*) FROM arbzg_compliance_log c " + whereClause
		if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
			return err
		}

		// Get paginated logs
		if perPage <= 0 {
			perPage = 20
		}
		if page <= 0 {
			page = 1
		}
		offset := (page - 1) * perPage

		query := `
			SELECT c.id, c.employee_id, c.time_entry_id, c.violation_date, c.violation_type,
			       c.severity, c.description, c.details, c.acknowledged_by, c.acknowledged_at,
			       c.resolution_note, c.created_at,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name
			FROM arbzg_compliance_log c
			LEFT JOIN employees e ON c.employee_id = e.id
		` + whereClause + `
			ORDER BY c.violation_date DESC, c.created_at DESC
			LIMIT $` + string(rune('0'+argNum)) + ` OFFSET $` + string(rune('0'+argNum+1))

		args = append(args, perPage, offset)
		return r.db.SelectContext(ctx, &logs, query, args...)
	})

	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// AcknowledgeViolation acknowledges a compliance violation
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *AbsenceRepository) AcknowledgeViolation(ctx context.Context, id string, acknowledgedBy string, resolutionNote *string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE arbzg_compliance_log SET
				acknowledged_by = $2, acknowledged_at = NOW(), resolution_note = $3
			WHERE id = $1 AND acknowledged_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query, id, acknowledgedBy, resolutionNote)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("compliance_log")
		}

		return nil
	})
}
