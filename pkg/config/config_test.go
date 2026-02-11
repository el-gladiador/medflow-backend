package config

import (
	"os"
	"testing"
)

func TestDatabaseConfig_DSN(t *testing.T) {
	tests := []struct {
		name   string
		config DatabaseConfig
		want   string
	}{
		{
			name: "uses URL when set",
			config: DatabaseConfig{
				URL:      "postgres://user:pass@urlhost:5432/urldb?sslmode=require",
				Host:     "localhost",
				Port:     5432,
				User:     "medflow_app",
				Password: "devpassword",
				Database: "medflow",
				SSLMode:  "disable",
			},
			want: "host=urlhost port=5432 user=user password=pass dbname=urldb sslmode=require",
		},
		{
			name: "uses individual fields when URL is empty",
			config: DatabaseConfig{
				URL:      "",
				Host:     "localhost",
				Port:     5432,
				User:     "medflow_app",
				Password: "devpassword",
				Database: "medflow",
				SSLMode:  "disable",
			},
			want: "host=localhost port=5432 user=medflow_app password=devpassword dbname=medflow sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.DSN()
			if got != tt.want {
				t.Errorf("DSN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDatabaseConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      DatabaseConfig
		environment string
		wantErr     bool
	}{
		{
			name: "development allows localhost defaults",
			config: DatabaseConfig{
				Host: "localhost",
			},
			environment: "development",
			wantErr:     false,
		},
		{
			name: "production requires URL or non-localhost host",
			config: DatabaseConfig{
				Host: "localhost",
			},
			environment: "production",
			wantErr:     true,
		},
		{
			name: "production accepts URL",
			config: DatabaseConfig{
				URL: "postgres://user:pass@prod-db.aws.com:5432/db?sslmode=require",
			},
			environment: "production",
			wantErr:     false,
		},
		{
			name: "production accepts non-localhost host",
			config: DatabaseConfig{
				Host: "prod-db.aws.com",
			},
			environment: "production",
			wantErr:     false,
		},
		{
			name: "staging requires URL or non-localhost host",
			config: DatabaseConfig{
				Host: "",
			},
			environment: "staging",
			wantErr:     true,
		},
		{
			name: "staging accepts URL",
			config: DatabaseConfig{
				URL: "postgres://user:pass@staging-db.aws.com:5432/db?sslmode=require",
			},
			environment: "staging",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate(tt.environment)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Clear any existing env vars that might interfere
	envVarsToClean := []string{
		"MEDFLOW_DATABASE_URL",
		"MEDFLOW_DATABASE_HOST",
		"MEDFLOW_DATABASE_PORT",
		"MEDFLOW_SERVER_ENVIRONMENT",
	}
	originals := make(map[string]string)
	for _, v := range envVarsToClean {
		originals[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range originals {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	cfg, err := Load("auth-service")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check defaults are applied
	if cfg.Server.Environment != "development" {
		t.Errorf("Server.Environment = %v, want development", cfg.Server.Environment)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host = %v, want localhost", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Database.Port = %v, want 5432", cfg.Database.Port)
	}
	if cfg.Database.Database != "medflow" {
		t.Errorf("Database.Database = %v, want medflow", cfg.Database.Database)
	}
}

func TestLoadWithValidation_Development(t *testing.T) {
	// Clear any existing env vars
	envVarsToClean := []string{
		"MEDFLOW_DATABASE_URL",
		"MEDFLOW_DATABASE_HOST",
		"MEDFLOW_SERVER_ENVIRONMENT",
		"MEDFLOW_JWT_SECRET",
		"MEDFLOW_RABBITMQ_URL",
	}
	originals := make(map[string]string)
	for _, v := range envVarsToClean {
		originals[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range originals {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	// Development should work with defaults
	cfg, err := LoadWithValidation("auth-service")
	if err != nil {
		t.Fatalf("LoadWithValidation() in development should not error: %v", err)
	}
	if cfg.Server.Environment != "development" {
		t.Errorf("Server.Environment = %v, want development", cfg.Server.Environment)
	}
}

func TestLoadWithValidation_ProductionRequiresConfig(t *testing.T) {
	// Clear existing env vars
	envVarsToClean := []string{
		"MEDFLOW_DATABASE_URL",
		"MEDFLOW_DATABASE_HOST",
		"MEDFLOW_SERVER_ENVIRONMENT",
		"MEDFLOW_JWT_SECRET",
		"MEDFLOW_RABBITMQ_URL",
	}
	originals := make(map[string]string)
	for _, v := range envVarsToClean {
		originals[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range originals {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	// Set production environment but no database config
	os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", "production")

	_, err := LoadWithValidation("auth-service")
	if err == nil {
		t.Error("LoadWithValidation() should fail in production without proper config")
	}
}

func TestLoadWithValidation_ProductionWithConfig(t *testing.T) {
	// Clear existing env vars
	envVarsToClean := []string{
		"MEDFLOW_DATABASE_URL",
		"MEDFLOW_DATABASE_HOST",
		"MEDFLOW_SERVER_ENVIRONMENT",
		"MEDFLOW_JWT_SECRET",
		"MEDFLOW_RABBITMQ_URL",
	}
	originals := make(map[string]string)
	for _, v := range envVarsToClean {
		originals[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range originals {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	// Set all required production config
	os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", "production")
	os.Setenv("MEDFLOW_DATABASE_URL", "postgres://user:pass@prod-db.aws.com:5432/db?sslmode=require")
	os.Setenv("MEDFLOW_JWT_SECRET", "super-secure-production-secret-at-least-32-chars")
	os.Setenv("MEDFLOW_RABBITMQ_URL", "amqps://user:pass@prod-mq.aws.com:5671/")

	cfg, err := LoadWithValidation("auth-service")
	if err != nil {
		t.Fatalf("LoadWithValidation() with proper production config should not error: %v", err)
	}
	if cfg.Server.Environment != "production" {
		t.Errorf("Server.Environment = %v, want production", cfg.Server.Environment)
	}
}

func TestLoadWithValidation_JWTSecretRequired(t *testing.T) {
	// Clear existing env vars
	envVarsToClean := []string{
		"MEDFLOW_DATABASE_URL",
		"MEDFLOW_DATABASE_HOST",
		"MEDFLOW_SERVER_ENVIRONMENT",
		"MEDFLOW_JWT_SECRET",
		"MEDFLOW_RABBITMQ_URL",
	}
	originals := make(map[string]string)
	for _, v := range envVarsToClean {
		originals[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range originals {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	// Production with database config but default JWT secret
	os.Setenv("MEDFLOW_SERVER_ENVIRONMENT", "production")
	os.Setenv("MEDFLOW_DATABASE_URL", "postgres://user:pass@prod-db.aws.com:5432/db?sslmode=require")
	os.Setenv("MEDFLOW_RABBITMQ_URL", "amqps://user:pass@prod-mq.aws.com:5671/")
	// JWT secret will use default which should fail

	_, err := LoadWithValidation("auth-service")
	if err == nil {
		t.Error("LoadWithValidation() should fail in production with default JWT secret")
	}
}

func TestLoad_DatabaseURLOverridesFields(t *testing.T) {
	// Clear existing env vars
	envVarsToClean := []string{
		"MEDFLOW_DATABASE_URL",
		"MEDFLOW_DATABASE_HOST",
		"MEDFLOW_DATABASE_PORT",
		"MEDFLOW_DATABASE_USER",
		"MEDFLOW_DATABASE_PASSWORD",
		"MEDFLOW_DATABASE_DATABASE",
		"MEDFLOW_DATABASE_SSL_MODE",
		"MEDFLOW_SERVER_ENVIRONMENT",
	}
	originals := make(map[string]string)
	for _, v := range envVarsToClean {
		originals[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range originals {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	// Set DATABASE_URL
	os.Setenv("MEDFLOW_DATABASE_URL", "postgres://urluser:urlpass@urlhost:5555/urldb?sslmode=verify-full")

	cfg, err := Load("auth-service")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Fields should be populated from URL
	if cfg.Database.Host != "urlhost" {
		t.Errorf("Database.Host = %v, want urlhost", cfg.Database.Host)
	}
	if cfg.Database.Port != 5555 {
		t.Errorf("Database.Port = %v, want 5555", cfg.Database.Port)
	}
	if cfg.Database.User != "urluser" {
		t.Errorf("Database.User = %v, want urluser", cfg.Database.User)
	}
	if cfg.Database.Password != "urlpass" {
		t.Errorf("Database.Password = %v, want urlpass", cfg.Database.Password)
	}
	if cfg.Database.Database != "urldb" {
		t.Errorf("Database.Database = %v, want urldb", cfg.Database.Database)
	}
	if cfg.Database.SSLMode != "verify-full" {
		t.Errorf("Database.SSLMode = %v, want verify-full", cfg.Database.SSLMode)
	}
}
