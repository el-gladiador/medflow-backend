package config

import (
	"os"
	"strings"
)

// Environment constants
const (
	EnvDevelopment = "development"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// GetEnv returns the value of an environment variable or a default value if not set.
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// RequireEnv returns the value of an environment variable or panics if not set.
// Use this for configuration that MUST be provided in production.
func RequireEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic("required environment variable not set: " + key)
	}
	return value
}

// GetEnvironment returns the current environment (development, staging, production).
// Defaults to development if not set.
func GetEnvironment() string {
	env := GetEnv("MEDFLOW_SERVER_ENVIRONMENT", EnvDevelopment)
	return strings.ToLower(env)
}

// IsDevelopment returns true if running in development environment.
func IsDevelopment() bool {
	return GetEnvironment() == EnvDevelopment
}

// IsStaging returns true if running in staging environment.
func IsStaging() bool {
	return GetEnvironment() == EnvStaging
}

// IsProduction returns true if running in production environment.
func IsProduction() bool {
	return GetEnvironment() == EnvProduction
}

// IsProductionLike returns true if running in staging or production environment.
// Use this when you need to enforce production-like configuration requirements.
func IsProductionLike() bool {
	return IsStaging() || IsProduction()
}
