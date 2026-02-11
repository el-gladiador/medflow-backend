package database

import (
	"strings"

	"github.com/lib/pq"
	"github.com/medflow/medflow-backend/pkg/errors"
)

// MapPQError converts a PostgreSQL error to an AppError with meaningful messages.
// Returns nil if the error is not a pq.Error.
func MapPQError(err error) *errors.AppError {
	pqErr, ok := err.(*pq.Error)
	if !ok {
		return nil
	}

	switch pqErr.Code {
	// Check constraint violation (23514)
	case "23514":
		return mapCheckConstraint(pqErr)

	// Unique constraint violation (23505)
	case "23505":
		return errors.Conflict(formatConstraintMessage(pqErr))

	// Foreign key violation (23503)
	case "23503":
		return errors.BadRequest("referenced record does not exist")

	// Not null violation (23502)
	case "23502":
		col := pqErr.Column
		if col == "" {
			col = "required field"
		}
		return errors.Validation(map[string]string{
			col: "must not be empty",
		})

	default:
		return nil
	}
}

// mapCheckConstraint maps specific CHECK constraint names to user-friendly messages.
func mapCheckConstraint(pqErr *pq.Error) *errors.AppError {
	constraint := pqErr.Constraint

	switch {
	case strings.Contains(constraint, "email_format"):
		return errors.Validation(map[string]string{
			"email": "must be a valid email address",
		})

	case strings.Contains(constraint, "employment_type_valid"):
		return errors.Validation(map[string]string{
			"employment_type": "must be one of: full_time, part_time, contractor, intern, temporary",
		})

	case strings.Contains(constraint, "status_valid"):
		return errors.Validation(map[string]string{
			"status": "must be one of: active, on_leave, suspended, terminated, pending",
		})

	default:
		return errors.BadRequest("data validation failed: " + constraint)
	}
}

// formatConstraintMessage creates a user-friendly message for unique constraint violations.
func formatConstraintMessage(pqErr *pq.Error) string {
	constraint := pqErr.Constraint

	switch {
	case strings.Contains(constraint, "employee_number") || strings.Contains(constraint, "tenant_number"):
		return "an employee with this employee number already exists"
	case strings.Contains(constraint, "email"):
		return "an employee with this email already exists"
	default:
		return "a record with these values already exists"
	}
}
