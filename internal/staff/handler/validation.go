package handler

import (
	"net/http"

	"github.com/medflow/medflow-backend/internal/staff/validation"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// ValidationHandler handles validation endpoints
type ValidationHandler struct {
	validator *validation.GermanValidator
	logger    *logger.Logger
}

// NewValidationHandler creates a new validation handler
func NewValidationHandler(v *validation.GermanValidator, log *logger.Logger) *ValidationHandler {
	return &ValidationHandler{
		validator: v,
		logger:    log,
	}
}

// ValidateIBAN validates a German IBAN
func (h *ValidationHandler) ValidateIBAN(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IBAN string `json:"iban" validate:"required"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	result := h.validator.ValidateIBAN(req.IBAN)
	httputil.JSON(w, http.StatusOK, result)
}

// ValidateTaxID validates a German Tax ID
func (h *ValidationHandler) ValidateTaxID(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaxID string `json:"tax_id" validate:"required"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	result := h.validator.ValidateTaxID(req.TaxID)
	httputil.JSON(w, http.StatusOK, result)
}

// ValidateSVNumber validates a German Social Insurance Number
func (h *ValidationHandler) ValidateSVNumber(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SVNumber string `json:"sv_number" validate:"required"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	result := h.validator.ValidateSVNumber(req.SVNumber)
	httputil.JSON(w, http.StatusOK, result)
}
