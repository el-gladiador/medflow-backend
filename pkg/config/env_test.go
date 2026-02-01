package config

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	// Test with existing env var
	os.Setenv("TEST_GET_ENV_VAR", "test_value")
	defer os.Unsetenv("TEST_GET_ENV_VAR")

	got := GetEnv("TEST_GET_ENV_VAR", "default")
	if got != "test_value" {
		t.Errorf("GetEnv() = %v, want %v", got, "test_value")
	}

	// Test with non-existing env var
	got = GetEnv("NON_EXISTING_VAR", "default_value")
	if got != "default_value" {
		t.Errorf("GetEnv() = %v, want %v", got, "default_value")
	}
}

func TestRequireEnv(t *testing.T) {
	// Test with existing env var
	os.Setenv("TEST_REQUIRE_ENV_VAR", "required_value")
	defer os.Unsetenv("TEST_REQUIRE_ENV_VAR")

	got := RequireEnv("TEST_REQUIRE_ENV_VAR")
	if got != "required_value" {
		t.Errorf("RequireEnv() = %v, want %v", got, "required_value")
	}

	// Test panic with non-existing env var
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("RequireEnv() should panic for missing env var")
		}
	}()
	RequireEnv("DEFINITELY_NON_EXISTING_VAR_12345")
}

func TestGetEnvironment(t *testing.T) {
	// Save original and restore after test
	original := os.Getenv("MEDFLOW_SERVER_ENVIRONMENT")
	defer func() {
		if original != "" {
			os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", original)
		} else {
			os.Unsetenv("MEDFLOW_SERVER_ENVIRONMENT")
		}
	}()

	tests := []struct {
		envValue string
		want     string
	}{
		{"development", "development"},
		{"DEVELOPMENT", "development"},
		{"staging", "staging"},
		{"STAGING", "staging"},
		{"production", "production"},
		{"PRODUCTION", "production"},
		{"", "development"}, // default
	}

	for _, tt := range tests {
		if tt.envValue != "" {
			os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", tt.envValue)
		} else {
			os.Unsetenv("MEDFLOW_SERVER_ENVIRONMENT")
		}

		got := GetEnvironment()
		if got != tt.want {
			t.Errorf("GetEnvironment() with %q = %v, want %v", tt.envValue, got, tt.want)
		}
	}
}

func TestIsDevelopment(t *testing.T) {
	original := os.Getenv("MEDFLOW_SERVER_ENVIRONMENT")
	defer func() {
		if original != "" {
			os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", original)
		} else {
			os.Unsetenv("MEDFLOW_SERVER_ENVIRONMENT")
		}
	}()

	os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", "development")
	if !IsDevelopment() {
		t.Error("IsDevelopment() should return true for development environment")
	}

	os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", "production")
	if IsDevelopment() {
		t.Error("IsDevelopment() should return false for production environment")
	}
}

func TestIsProduction(t *testing.T) {
	original := os.Getenv("MEDFLOW_SERVER_ENVIRONMENT")
	defer func() {
		if original != "" {
			os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", original)
		} else {
			os.Unsetenv("MEDFLOW_SERVER_ENVIRONMENT")
		}
	}()

	os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", "production")
	if !IsProduction() {
		t.Error("IsProduction() should return true for production environment")
	}

	os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", "development")
	if IsProduction() {
		t.Error("IsProduction() should return false for development environment")
	}
}

func TestIsStaging(t *testing.T) {
	original := os.Getenv("MEDFLOW_SERVER_ENVIRONMENT")
	defer func() {
		if original != "" {
			os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", original)
		} else {
			os.Unsetenv("MEDFLOW_SERVER_ENVIRONMENT")
		}
	}()

	os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", "staging")
	if !IsStaging() {
		t.Error("IsStaging() should return true for staging environment")
	}

	os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", "production")
	if IsStaging() {
		t.Error("IsStaging() should return false for production environment")
	}
}

func TestIsProductionLike(t *testing.T) {
	original := os.Getenv("MEDFLOW_SERVER_ENVIRONMENT")
	defer func() {
		if original != "" {
			os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", original)
		} else {
			os.Unsetenv("MEDFLOW_SERVER_ENVIRONMENT")
		}
	}()

	os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", "production")
	if !IsProductionLike() {
		t.Error("IsProductionLike() should return true for production")
	}

	os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", "staging")
	if !IsProductionLike() {
		t.Error("IsProductionLike() should return true for staging")
	}

	os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", "development")
	if IsProductionLike() {
		t.Error("IsProductionLike() should return false for development")
	}
}
