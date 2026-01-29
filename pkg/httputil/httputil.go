package httputil

import (
	"encoding/json"
	"net/http"

	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/i18n"
)

// Response is a standard API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorBody  `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// ErrorBody represents an error in the response
type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// Meta contains pagination and other metadata
type Meta struct {
	Page       int   `json:"page,omitempty"`
	PerPage    int   `json:"per_page,omitempty"`
	Total      int64 `json:"total,omitempty"`
	TotalPages int   `json:"total_pages,omitempty"`
}

// JSON sends a JSON response
func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := Response{
		Success: statusCode >= 200 && statusCode < 300,
		Data:    data,
	}

	json.NewEncoder(w).Encode(response)
}

// JSONWithMeta sends a JSON response with metadata
func JSONWithMeta(w http.ResponseWriter, statusCode int, data interface{}, meta *Meta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := Response{
		Success: statusCode >= 200 && statusCode < 300,
		Data:    data,
		Meta:    meta,
	}

	json.NewEncoder(w).Encode(response)
}

// Error sends an error response (uses default locale)
func Error(w http.ResponseWriter, err error) {
	var appErr *errors.AppError
	if errors.As(err, &appErr) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(appErr.StatusCode)

		response := Response{
			Success: false,
			Error: &ErrorBody{
				Code:    appErr.Code,
				Message: appErr.Message,
				Details: appErr.Details,
			},
		}

		json.NewEncoder(w).Encode(response)
		return
	}

	// Default to internal server error
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)

	response := Response{
		Success: false,
		Error: &ErrorBody{
			Code:    "INTERNAL_ERROR",
			Message: "an unexpected error occurred",
		},
	}

	json.NewEncoder(w).Encode(response)
}

// ErrorLocalized sends a localized error response using request context
func ErrorLocalized(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *errors.AppError
	if errors.As(err, &appErr) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(appErr.StatusCode)

		// Get localized message
		message := appErr.Localize(r.Context())

		response := Response{
			Success: false,
			Error: &ErrorBody{
				Code:    appErr.Code,
				Message: message,
				Details: appErr.Details,
			},
		}

		json.NewEncoder(w).Encode(response)
		return
	}

	// Default to internal server error with localized message
	localizer := i18n.LocalizerFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)

	response := Response{
		Success: false,
		Error: &ErrorBody{
			Code:    "INTERNAL_ERROR",
			Message: localizer.T("errors.internal"),
		},
	}

	json.NewEncoder(w).Encode(response)
}

// NoContent sends a 204 No Content response
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Created sends a 201 Created response
func Created(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, data)
}

// DecodeJSON decodes the request body into the provided struct
func DecodeJSON(r *http.Request, v interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return errors.BadRequest("invalid JSON body")
	}
	return nil
}

// DecodeJSONLocalized decodes the request body with localized error
func DecodeJSONLocalized(r *http.Request, v interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		localizer := i18n.LocalizerFromContext(r.Context())
		return errors.BadRequest(localizer.T("errors.invalid_json"))
	}
	return nil
}
