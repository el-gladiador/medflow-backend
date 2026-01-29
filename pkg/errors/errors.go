package errors

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/medflow/medflow-backend/pkg/i18n"
)

// Standard error types
var (
	ErrNotFound           = errors.New("resource not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrBadRequest         = errors.New("bad request")
	ErrConflict           = errors.New("resource conflict")
	ErrInternal           = errors.New("internal server error")
	ErrValidation         = errors.New("validation error")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenInvalid       = errors.New("invalid token")
)

// AppError represents an application error with context
type AppError struct {
	Err        error             `json:"-"`
	Message    string            `json:"message"`
	MessageKey string            `json:"-"` // i18n key for localization
	Params     map[string]string `json:"-"` // Parameters for i18n interpolation
	Code       string            `json:"code"`
	StatusCode int               `json:"status_code"`
	Details    map[string]string `json:"details,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the wrapped error
func (e *AppError) Unwrap() error {
	return e.Err
}

// Localize returns a localized version of the error message
func (e *AppError) Localize(ctx context.Context) string {
	if e.MessageKey == "" {
		return e.Message
	}
	return i18n.TFromContext(ctx, e.MessageKey, e.Params)
}

// LocalizeWith returns a localized version using a specific localizer
func (e *AppError) LocalizeWith(l *i18n.Localizer) string {
	if e.MessageKey == "" {
		return e.Message
	}
	return l.T(e.MessageKey, e.Params)
}

// New creates a new AppError
func New(code string, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// NewWithKey creates a new AppError with an i18n key
func NewWithKey(code string, messageKey string, statusCode int, params ...map[string]string) *AppError {
	var p map[string]string
	if len(params) > 0 {
		p = params[0]
	}
	return &AppError{
		Code:       code,
		Message:    i18n.T(messageKey, p), // Default message in English
		MessageKey: messageKey,
		Params:     p,
		StatusCode: statusCode,
	}
}

// Wrap wraps an error with additional context
func Wrap(err error, code string, message string, statusCode int) *AppError {
	return &AppError{
		Err:        err,
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// WithDetails adds details to an AppError
func (e *AppError) WithDetails(details map[string]string) *AppError {
	e.Details = details
	return e
}

// Common error constructors

func NotFound(resource string) *AppError {
	return &AppError{
		Err:        ErrNotFound,
		Code:       "NOT_FOUND",
		Message:    fmt.Sprintf("%s not found", resource),
		MessageKey: "errors.not_found",
		Params:     map[string]string{"resource": resource},
		StatusCode: http.StatusNotFound,
	}
}

// NotFoundWithKey creates a not found error with localized resource name
func NotFoundWithKey(resourceKey string) *AppError {
	resourceName := i18n.T("resources." + resourceKey)
	return &AppError{
		Err:        ErrNotFound,
		Code:       "NOT_FOUND",
		Message:    fmt.Sprintf("%s not found", resourceName),
		MessageKey: "errors.not_found",
		Params:     map[string]string{"resource": resourceName},
		StatusCode: http.StatusNotFound,
	}
}

func Unauthorized(message string) *AppError {
	return &AppError{
		Err:        ErrUnauthorized,
		Code:       "UNAUTHORIZED",
		Message:    message,
		MessageKey: "errors.unauthorized",
		StatusCode: http.StatusUnauthorized,
	}
}

func Forbidden(message string) *AppError {
	return &AppError{
		Err:        ErrForbidden,
		Code:       "FORBIDDEN",
		Message:    message,
		MessageKey: "errors.forbidden",
		StatusCode: http.StatusForbidden,
	}
}

func BadRequest(message string) *AppError {
	return &AppError{
		Err:        ErrBadRequest,
		Code:       "BAD_REQUEST",
		Message:    message,
		MessageKey: "errors.bad_request",
		StatusCode: http.StatusBadRequest,
	}
}

func Conflict(message string) *AppError {
	return &AppError{
		Err:        ErrConflict,
		Code:       "CONFLICT",
		Message:    message,
		MessageKey: "errors.conflict",
		StatusCode: http.StatusConflict,
	}
}

func Internal(message string) *AppError {
	return &AppError{
		Err:        ErrInternal,
		Code:       "INTERNAL_ERROR",
		Message:    message,
		MessageKey: "errors.internal",
		StatusCode: http.StatusInternalServerError,
	}
}

func Validation(details map[string]string) *AppError {
	return &AppError{
		Err:        ErrValidation,
		Code:       "VALIDATION_ERROR",
		Message:    "validation failed",
		MessageKey: "errors.validation_failed",
		StatusCode: http.StatusBadRequest,
		Details:    details,
	}
}

func InvalidCredentials() *AppError {
	return &AppError{
		Err:        ErrInvalidCredentials,
		Code:       "INVALID_CREDENTIALS",
		Message:    "invalid email or password",
		MessageKey: "errors.invalid_credentials",
		StatusCode: http.StatusUnauthorized,
	}
}

func TokenExpired() *AppError {
	return &AppError{
		Err:        ErrTokenExpired,
		Code:       "TOKEN_EXPIRED",
		Message:    "token has expired",
		MessageKey: "errors.token_expired",
		StatusCode: http.StatusUnauthorized,
	}
}

func TokenInvalid() *AppError {
	return &AppError{
		Err:        ErrTokenInvalid,
		Code:       "TOKEN_INVALID",
		Message:    "invalid token",
		MessageKey: "errors.token_invalid",
		StatusCode: http.StatusUnauthorized,
	}
}

// Is checks if the error matches a target error
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As attempts to convert an error to a specific type
func As(err error, target any) bool {
	return errors.As(err, target)
}
