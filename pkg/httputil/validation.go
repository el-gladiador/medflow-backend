package httputil

import (
	"github.com/go-playground/validator/v10"
	"github.com/medflow/medflow-backend/pkg/errors"
)

var validate = validator.New()

// Validate validates a struct using go-playground/validator
func Validate(v interface{}) error {
	if err := validate.Struct(v); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		details := make(map[string]string)

		for _, e := range validationErrors {
			details[e.Field()] = formatValidationError(e)
		}

		return errors.Validation(details)
	}
	return nil
}

func formatValidationError(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "this field is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + e.Param() + " characters"
	case "max":
		return "must be at most " + e.Param() + " characters"
	case "uuid":
		return "must be a valid UUID"
	case "oneof":
		return "must be one of: " + e.Param()
	default:
		return "invalid value"
	}
}

// RegisterCustomValidation registers a custom validation function
func RegisterCustomValidation(tag string, fn validator.Func) error {
	return validate.RegisterValidation(tag, fn)
}
