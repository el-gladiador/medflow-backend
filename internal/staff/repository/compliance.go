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

// ============================================================================
// COMPLIANCE VIOLATION
// ============================================================================

// ComplianceViolation represents a record of a labor law violation
type ComplianceViolation struct {
	ID                string     `db:"id" json:"id"`
	EmployeeID        string     `db:"employee_id" json:"employee_id"`
	ViolationType     string     `db:"violation_type" json:"violation_type"`
	ViolationDate     time.Time  `db:"violation_date" json:"violation_date"`
	TimeEntryID       *string    `db:"time_entry_id" json:"time_entry_id,omitempty"`
	ShiftAssignmentID *string    `db:"shift_assignment_id" json:"shift_assignment_id,omitempty"`
	ExpectedValue     *string    `db:"expected_value" json:"expected_value,omitempty"`
	ActualValue       *string    `db:"actual_value" json:"actual_value,omitempty"`
	Description       *string    `db:"description" json:"description,omitempty"`
	Status            string     `db:"status" json:"status"`
	AcknowledgedBy    *string    `db:"acknowledged_by" json:"acknowledged_by,omitempty"`
	AcknowledgedAt    *time.Time `db:"acknowledged_at" json:"acknowledged_at,omitempty"`
	ResolutionNotes   *string    `db:"resolution_notes" json:"resolution_notes,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`

	// Joined fields
	EmployeeName *string `db:"employee_name" json:"employee_name,omitempty"`
}

// Violation types
const (
	ViolationMissingBreak          = "missing_break"
	ViolationInsufficientBreak     = "insufficient_break"
	ViolationBreakTooShort6h       = "break_too_short_6h"
	ViolationBreakTooShort9h       = "break_too_short_9h"
	ViolationMaxDailyHoursExceeded = "max_daily_hours_exceeded"
	ViolationMaxWeeklyHoursExceeded = "max_weekly_hours_exceeded"
	ViolationRestPeriodViolated    = "rest_period_violated"
	ViolationNoBreakAfter6h        = "no_break_after_6h"
)

// ============================================================================
// COMPLIANCE SETTINGS
// ============================================================================

// ComplianceSettings represents tenant-configurable compliance settings
type ComplianceSettings struct {
	ID                             string    `db:"id" json:"id"`
	MinBreak6hMinutes              int       `db:"min_break_6h_minutes" json:"min_break_6h_minutes"`
	MinBreak9hMinutes              int       `db:"min_break_9h_minutes" json:"min_break_9h_minutes"`
	MinBreakSegmentMinutes         int       `db:"min_break_segment_minutes" json:"min_break_segment_minutes"`
	MaxDailyHours                  int       `db:"max_daily_hours" json:"max_daily_hours"`
	TargetDailyHours               int       `db:"target_daily_hours" json:"target_daily_hours"`
	MaxWeeklyHours                 int       `db:"max_weekly_hours" json:"max_weekly_hours"`
	MinRestBetweenShiftsHours      int       `db:"min_rest_between_shifts_hours" json:"min_rest_between_shifts_hours"`
	AlertNoBreakAfterMinutes       int       `db:"alert_no_break_after_minutes" json:"alert_no_break_after_minutes"`
	AlertBreakTooLongMinutes       int       `db:"alert_break_too_long_minutes" json:"alert_break_too_long_minutes"`
	AlertApproachingMaxHoursMinutes int      `db:"alert_approaching_max_hours_minutes" json:"alert_approaching_max_hours_minutes"`
	NotifyEmployeeViolations       bool      `db:"notify_employee_violations" json:"notify_employee_violations"`
	NotifyManagerViolations        bool      `db:"notify_manager_violations" json:"notify_manager_violations"`
	CreatedAt                      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt                      time.Time `db:"updated_at" json:"updated_at"`
}

// ============================================================================
// COMPLIANCE ALERT
// ============================================================================

// ComplianceAlert represents a real-time compliance alert
type ComplianceAlert struct {
	ID          string     `db:"id" json:"id"`
	EmployeeID  string     `db:"employee_id" json:"employee_id"`
	AlertType   string     `db:"alert_type" json:"alert_type"`
	Severity    string     `db:"severity" json:"severity"`
	Message     string     `db:"message" json:"message"`
	ActionLabel *string    `db:"action_label" json:"action_label,omitempty"`
	IsActive    bool       `db:"is_active" json:"is_active"`
	DismissedBy *string    `db:"dismissed_by" json:"dismissed_by,omitempty"`
	DismissedAt *time.Time `db:"dismissed_at" json:"dismissed_at,omitempty"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`

	// Joined fields
	EmployeeName *string `db:"employee_name" json:"employee_name,omitempty"`
}

// Alert types
const (
	AlertNoBreakWarning      = "no_break_warning"
	AlertBreakTooLong        = "break_too_long"
	AlertMaxHoursApproaching = "max_hours_approaching"
	AlertMaxHoursExceeded    = "max_hours_exceeded"
)

// Alert severities
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// ============================================================================
// TIME CORRECTION REQUEST
// ============================================================================

// CorrectionRequest represents an employee's request for time correction
type CorrectionRequest struct {
	ID                string     `db:"id" json:"id"`
	EmployeeID        string     `db:"employee_id" json:"employee_id"`
	TimeEntryID       *string    `db:"time_entry_id" json:"time_entry_id,omitempty"`
	RequestedDate     time.Time  `db:"requested_date" json:"requested_date"`
	RequestedClockIn  *time.Time `db:"requested_clock_in" json:"requested_clock_in,omitempty"`
	RequestedClockOut *time.Time `db:"requested_clock_out" json:"requested_clock_out,omitempty"`
	RequestType       string     `db:"request_type" json:"request_type"`
	Reason            string     `db:"reason" json:"reason"`
	Status            string     `db:"status" json:"status"`
	ReviewedBy        *string    `db:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt        *time.Time `db:"reviewed_at" json:"reviewed_at,omitempty"`
	RejectionReason   *string    `db:"rejection_reason" json:"rejection_reason,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt         *time.Time `db:"deleted_at" json:"-"`

	// Joined fields
	EmployeeName *string `db:"employee_name" json:"employee_name,omitempty"`
	ReviewerName *string `db:"reviewer_name" json:"reviewer_name,omitempty"`
}

// Correction request types
const (
	CorrectionTypeClockIn   = "clock_in_correction"
	CorrectionTypeClockOut  = "clock_out_correction"
	CorrectionTypeMissedEntry = "missed_entry"
	CorrectionTypeDeleteEntry = "delete_entry"
)

// Correction request statuses
const (
	CorrectionStatusPending  = "pending"
	CorrectionStatusApproved = "approved"
	CorrectionStatusRejected = "rejected"
)

// ============================================================================
// REPOSITORY
// ============================================================================

// ComplianceRepository handles compliance data persistence
type ComplianceRepository struct {
	db *database.DB
}

// NewComplianceRepository creates a new compliance repository
func NewComplianceRepository(db *database.DB) *ComplianceRepository {
	return &ComplianceRepository{db: db}
}

// ============================================================================
// SETTINGS METHODS
// ============================================================================

// GetSettings gets the tenant's compliance settings
func (r *ComplianceRepository) GetSettings(ctx context.Context) (*ComplianceSettings, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var settings ComplianceSettings

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, min_break_6h_minutes, min_break_9h_minutes, min_break_segment_minutes,
			       max_daily_hours, target_daily_hours, max_weekly_hours,
			       min_rest_between_shifts_hours, alert_no_break_after_minutes,
			       alert_break_too_long_minutes, alert_approaching_max_hours_minutes,
			       notify_employee_violations, notify_manager_violations,
			       created_at, updated_at
			FROM compliance_settings
			LIMIT 1
		`
		return r.db.GetContext(ctx, &settings, query)
	})

	if err == sql.ErrNoRows {
		// Return default settings if none exist
		return &ComplianceSettings{
			MinBreak6hMinutes:              30,
			MinBreak9hMinutes:              45,
			MinBreakSegmentMinutes:         15,
			MaxDailyHours:                  10,
			TargetDailyHours:               8,
			MaxWeeklyHours:                 48,
			MinRestBetweenShiftsHours:      11,
			AlertNoBreakAfterMinutes:       360,
			AlertBreakTooLongMinutes:       60,
			AlertApproachingMaxHoursMinutes: 30,
			NotifyEmployeeViolations:       true,
			NotifyManagerViolations:        true,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return &settings, nil
}

// UpdateSettings updates the tenant's compliance settings
func (r *ComplianceRepository) UpdateSettings(ctx context.Context, settings *ComplianceSettings) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE compliance_settings SET
				min_break_6h_minutes = $1,
				min_break_9h_minutes = $2,
				min_break_segment_minutes = $3,
				max_daily_hours = $4,
				target_daily_hours = $5,
				max_weekly_hours = $6,
				min_rest_between_shifts_hours = $7,
				alert_no_break_after_minutes = $8,
				alert_break_too_long_minutes = $9,
				alert_approaching_max_hours_minutes = $10,
				notify_employee_violations = $11,
				notify_manager_violations = $12
		`
		_, err := r.db.ExecContext(ctx, query,
			settings.MinBreak6hMinutes,
			settings.MinBreak9hMinutes,
			settings.MinBreakSegmentMinutes,
			settings.MaxDailyHours,
			settings.TargetDailyHours,
			settings.MaxWeeklyHours,
			settings.MinRestBetweenShiftsHours,
			settings.AlertNoBreakAfterMinutes,
			settings.AlertBreakTooLongMinutes,
			settings.AlertApproachingMaxHoursMinutes,
			settings.NotifyEmployeeViolations,
			settings.NotifyManagerViolations,
		)
		return err
	})
}

// ============================================================================
// VIOLATION METHODS
// ============================================================================

// CreateViolation creates a new compliance violation record
func (r *ComplianceRepository) CreateViolation(ctx context.Context, v *ComplianceViolation) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	if v.Status == "" {
		v.Status = "open"
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO compliance_violations (
				id, employee_id, violation_type, violation_date,
				time_entry_id, shift_assignment_id, expected_value, actual_value,
				description, status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			v.ID, v.EmployeeID, v.ViolationType, v.ViolationDate,
			v.TimeEntryID, v.ShiftAssignmentID, v.ExpectedValue, v.ActualValue,
			v.Description, v.Status,
		).Scan(&v.CreatedAt, &v.UpdatedAt)
	})
}

// ListViolations lists violations with filters
func (r *ComplianceRepository) ListViolations(ctx context.Context, employeeID *string, startDate, endDate *time.Time, status *string) ([]*ComplianceViolation, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var violations []*ComplianceViolation

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT v.id, v.employee_id, v.violation_type, v.violation_date,
			       v.time_entry_id, v.shift_assignment_id, v.expected_value, v.actual_value,
			       v.description, v.status, v.acknowledged_by, v.acknowledged_at,
			       v.resolution_notes, v.created_at, v.updated_at,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name
			FROM compliance_violations v
			LEFT JOIN employees e ON v.employee_id = e.id
			WHERE 1=1
		`
		args := []interface{}{}
		argNum := 1

		if employeeID != nil {
			query += ` AND v.employee_id = $` + string(rune('0'+argNum))
			args = append(args, *employeeID)
			argNum++
		}
		if startDate != nil {
			query += ` AND v.violation_date >= $` + string(rune('0'+argNum))
			args = append(args, *startDate)
			argNum++
		}
		if endDate != nil {
			query += ` AND v.violation_date <= $` + string(rune('0'+argNum))
			args = append(args, *endDate)
			argNum++
		}
		if status != nil {
			query += ` AND v.status = $` + string(rune('0'+argNum))
			args = append(args, *status)
		}

		query += ` ORDER BY v.violation_date DESC, v.created_at DESC`

		return r.db.SelectContext(ctx, &violations, query, args...)
	})

	if err != nil {
		return nil, err
	}

	return violations, nil
}

// AcknowledgeViolation marks a violation as acknowledged
func (r *ComplianceRepository) AcknowledgeViolation(ctx context.Context, id string, acknowledgedBy string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE compliance_violations
			SET status = 'acknowledged', acknowledged_by = $2, acknowledged_at = NOW()
			WHERE id = $1
		`
		result, err := r.db.ExecContext(ctx, query, id, acknowledgedBy)
		if err != nil {
			return err
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return errors.NotFound("violation")
		}
		return nil
	})
}

// ============================================================================
// ALERT METHODS
// ============================================================================

// CreateAlert creates a new compliance alert
func (r *ComplianceRepository) CreateAlert(ctx context.Context, a *ComplianceAlert) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if a.ID == "" {
		a.ID = uuid.New().String()
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO compliance_alerts (
				id, employee_id, alert_type, severity, message, action_label, is_active
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			a.ID, a.EmployeeID, a.AlertType, a.Severity, a.Message, a.ActionLabel, true,
		).Scan(&a.CreatedAt, &a.UpdatedAt)
	})
}

// ListActiveAlerts lists all active alerts
func (r *ComplianceRepository) ListActiveAlerts(ctx context.Context) ([]*ComplianceAlert, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var alerts []*ComplianceAlert

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT a.id, a.employee_id, a.alert_type, a.severity, a.message,
			       a.action_label, a.is_active, a.dismissed_by, a.dismissed_at,
			       a.created_at, a.updated_at,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name
			FROM compliance_alerts a
			LEFT JOIN employees e ON a.employee_id = e.id
			WHERE a.is_active = true
			ORDER BY
				CASE a.severity
					WHEN 'critical' THEN 1
					WHEN 'warning' THEN 2
					ELSE 3
				END,
				a.created_at DESC
		`
		return r.db.SelectContext(ctx, &alerts, query)
	})

	if err != nil {
		return nil, err
	}

	return alerts, nil
}

// DismissAlert dismisses an alert
func (r *ComplianceRepository) DismissAlert(ctx context.Context, id string, dismissedBy string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE compliance_alerts
			SET is_active = false, dismissed_by = $2, dismissed_at = NOW()
			WHERE id = $1
		`
		_, err := r.db.ExecContext(ctx, query, id, dismissedBy)
		return err
	})
}

// DeactivateAlertsForEmployee deactivates all alerts for an employee
func (r *ComplianceRepository) DeactivateAlertsForEmployee(ctx context.Context, employeeID string, alertType string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE compliance_alerts
			SET is_active = false
			WHERE employee_id = $1 AND alert_type = $2 AND is_active = true
		`
		_, err := r.db.ExecContext(ctx, query, employeeID, alertType)
		return err
	})
}

// ============================================================================
// CORRECTION REQUEST METHODS
// ============================================================================

// CreateCorrectionRequest creates a new correction request
func (r *ComplianceRepository) CreateCorrectionRequest(ctx context.Context, req *CorrectionRequest) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.Status == "" {
		req.Status = CorrectionStatusPending
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO time_correction_requests (
				id, employee_id, time_entry_id, requested_date,
				requested_clock_in, requested_clock_out, request_type, reason, status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			req.ID, req.EmployeeID, req.TimeEntryID, req.RequestedDate,
			req.RequestedClockIn, req.RequestedClockOut, req.RequestType, req.Reason, req.Status,
		).Scan(&req.CreatedAt, &req.UpdatedAt)
	})
}

// GetCorrectionRequestByID gets a correction request by ID
func (r *ComplianceRepository) GetCorrectionRequestByID(ctx context.Context, id string) (*CorrectionRequest, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var req CorrectionRequest

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT r.id, r.employee_id, r.time_entry_id, r.requested_date,
			       r.requested_clock_in, r.requested_clock_out, r.request_type, r.reason,
			       r.status, r.reviewed_by, r.reviewed_at, r.rejection_reason,
			       r.created_at, r.updated_at,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name,
			       CONCAT(rev.first_name, ' ', rev.last_name) as reviewer_name
			FROM time_correction_requests r
			LEFT JOIN employees e ON r.employee_id = e.id
			LEFT JOIN employees rev ON r.reviewed_by = rev.id
			WHERE r.id = $1 AND r.deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &req, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("correction_request")
	}
	if err != nil {
		return nil, err
	}

	return &req, nil
}

// ListPendingCorrectionRequests lists all pending correction requests
func (r *ComplianceRepository) ListPendingCorrectionRequests(ctx context.Context) ([]*CorrectionRequest, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var requests []*CorrectionRequest

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT r.id, r.employee_id, r.time_entry_id, r.requested_date,
			       r.requested_clock_in, r.requested_clock_out, r.request_type, r.reason,
			       r.status, r.reviewed_by, r.reviewed_at, r.rejection_reason,
			       r.created_at, r.updated_at,
			       CONCAT(e.first_name, ' ', e.last_name) as employee_name
			FROM time_correction_requests r
			LEFT JOIN employees e ON r.employee_id = e.id
			WHERE r.status = 'pending' AND r.deleted_at IS NULL
			ORDER BY r.created_at ASC
		`
		return r.db.SelectContext(ctx, &requests, query)
	})

	if err != nil {
		return nil, err
	}

	return requests, nil
}

// ListCorrectionRequestsByEmployee lists correction requests for an employee
func (r *ComplianceRepository) ListCorrectionRequestsByEmployee(ctx context.Context, employeeID string) ([]*CorrectionRequest, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var requests []*CorrectionRequest

	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT r.id, r.employee_id, r.time_entry_id, r.requested_date,
			       r.requested_clock_in, r.requested_clock_out, r.request_type, r.reason,
			       r.status, r.reviewed_by, r.reviewed_at, r.rejection_reason,
			       r.created_at, r.updated_at,
			       CONCAT(rev.first_name, ' ', rev.last_name) as reviewer_name
			FROM time_correction_requests r
			LEFT JOIN employees rev ON r.reviewed_by = rev.id
			WHERE r.employee_id = $1 AND r.deleted_at IS NULL
			ORDER BY r.created_at DESC
		`
		return r.db.SelectContext(ctx, &requests, query, employeeID)
	})

	if err != nil {
		return nil, err
	}

	return requests, nil
}

// UpdateCorrectionRequestStatus updates the status of a correction request
func (r *ComplianceRepository) UpdateCorrectionRequestStatus(ctx context.Context, id string, status string, reviewerID string, rejectionReason *string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE time_correction_requests
			SET status = $2, reviewed_by = $3, reviewed_at = NOW(), rejection_reason = $4
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query, id, status, reviewerID, rejectionReason)
		if err != nil {
			return err
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return errors.NotFound("correction_request")
		}
		return nil
	})
}
