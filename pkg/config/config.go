package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	RabbitMQ RabbitMQConfig
	JWT      JWTConfig
	Services ServicesConfig
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Host         string        `mapstructure:"host"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	Environment  string        `mapstructure:"environment"`
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// DSN returns the PostgreSQL connection string
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// RabbitMQConfig holds RabbitMQ connection configuration
type RabbitMQConfig struct {
	URL             string        `mapstructure:"url"`
	ReconnectDelay  time.Duration `mapstructure:"reconnect_delay"`
	MaxRetries      int           `mapstructure:"max_retries"`
	PrefetchCount   int           `mapstructure:"prefetch_count"`
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret           string        `mapstructure:"secret"`
	AccessExpiry     time.Duration `mapstructure:"access_expiry"`
	RefreshExpiry    time.Duration `mapstructure:"refresh_expiry"`
	Issuer           string        `mapstructure:"issuer"`
}

// ServicesConfig holds URLs for other services
type ServicesConfig struct {
	AuthServiceURL      string `mapstructure:"auth_service_url"`
	UserServiceURL      string `mapstructure:"user_service_url"`
	StaffServiceURL     string `mapstructure:"staff_service_url"`
	InventoryServiceURL string `mapstructure:"inventory_service_url"`
}

// Load loads configuration from environment and config files
func Load(serviceName string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v, serviceName)

	// Read from environment variables
	v.SetEnvPrefix("MEDFLOW")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read from config file if exists
	v.SetConfigName(serviceName)
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/medflow")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper, serviceName string) {
	// Server defaults
	port := getDefaultPort(serviceName)
	v.SetDefault("server.port", port)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.read_timeout", 30*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)
	v.SetDefault("server.environment", "development")

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", getDefaultDBPort(serviceName))
	v.SetDefault("database.user", "medflow")
	v.SetDefault("database.password", "devpassword")
	v.SetDefault("database.database", getDefaultDBName(serviceName))
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", 5*time.Minute)

	// RabbitMQ defaults
	v.SetDefault("rabbitmq.url", "amqp://medflow:devpassword@localhost:5672/")
	v.SetDefault("rabbitmq.reconnect_delay", 5*time.Second)
	v.SetDefault("rabbitmq.max_retries", 5)
	v.SetDefault("rabbitmq.prefetch_count", 10)

	// JWT defaults
	v.SetDefault("jwt.secret", "dev-secret-change-in-production")
	v.SetDefault("jwt.access_expiry", 15*time.Minute)
	v.SetDefault("jwt.refresh_expiry", 7*24*time.Hour)
	v.SetDefault("jwt.issuer", "medflow")

	// Services defaults
	v.SetDefault("services.auth_service_url", "http://localhost:8081")
	v.SetDefault("services.user_service_url", "http://localhost:8082")
	v.SetDefault("services.staff_service_url", "http://localhost:8083")
	v.SetDefault("services.inventory_service_url", "http://localhost:8084")
}

func getDefaultPort(serviceName string) int {
	ports := map[string]int{
		"api-gateway":       8080,
		"auth-service":      8081,
		"user-service":      8082,
		"staff-service":     8083,
		"inventory-service": 8084,
	}
	if port, ok := ports[serviceName]; ok {
		return port
	}
	return 8080
}

func getDefaultDBPort(serviceName string) int {
	ports := map[string]int{
		"auth-service":      5433,
		"user-service":      5434,
		"staff-service":     5435,
		"inventory-service": 5436,
	}
	if port, ok := ports[serviceName]; ok {
		return port
	}
	return 5432
}

func getDefaultDBName(serviceName string) string {
	names := map[string]string{
		"auth-service":      "medflow_auth",
		"user-service":      "medflow_users",
		"staff-service":     "medflow_staff",
		"inventory-service": "medflow_inventory",
	}
	if name, ok := names[serviceName]; ok {
		return name
	}
	return "medflow"
}
