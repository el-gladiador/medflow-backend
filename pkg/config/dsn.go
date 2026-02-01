package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParsedDatabaseURL holds the parsed components of a database connection URL.
type ParsedDatabaseURL struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	Options  map[string]string
}

// ParseDatabaseURL parses a PostgreSQL connection URL into its components.
// Supports URLs in the format: postgres://user:password@host:port/database?sslmode=disable
func ParseDatabaseURL(rawURL string) (*ParsedDatabaseURL, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("database URL is empty")
	}

	// Handle both postgres:// and postgresql:// schemes
	rawURL = strings.Replace(rawURL, "postgresql://", "postgres://", 1)

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	if u.Scheme != "postgres" {
		return nil, fmt.Errorf("invalid database URL scheme: %s (expected postgres or postgresql)", u.Scheme)
	}

	result := &ParsedDatabaseURL{
		Host:    u.Hostname(),
		Options: make(map[string]string),
	}

	// Parse port
	if portStr := u.Port(); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port in database URL: %w", err)
		}
		result.Port = port
	} else {
		result.Port = 5432 // Default PostgreSQL port
	}

	// Parse user and password
	if u.User != nil {
		result.User = u.User.Username()
		result.Password, _ = u.User.Password()
	}

	// Parse database name (remove leading slash)
	result.Database = strings.TrimPrefix(u.Path, "/")

	// Parse query parameters
	for key, values := range u.Query() {
		if len(values) > 0 {
			result.Options[key] = values[0]
		}
	}

	// Extract SSLMode from options
	if sslMode, ok := result.Options["sslmode"]; ok {
		result.SSLMode = sslMode
		delete(result.Options, "sslmode")
	} else {
		result.SSLMode = "disable" // Default for development
	}

	return result, nil
}

// BuildDatabaseURL constructs a PostgreSQL connection URL from individual components.
func BuildDatabaseURL(host string, port int, user, password, database, sslMode string) string {
	if sslMode == "" {
		sslMode = "disable"
	}

	// URL encode the password to handle special characters
	encodedPassword := url.QueryEscape(password)

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		user, encodedPassword, host, port, database, sslMode,
	)
}

// ToDSN converts the parsed URL to a libpq-style DSN string.
func (p *ParsedDatabaseURL) ToDSN() string {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.Database, p.SSLMode,
	)

	// Append additional options
	for key, value := range p.Options {
		dsn += fmt.Sprintf(" %s=%s", key, value)
	}

	return dsn
}

// ToURL converts the parsed components back to a URL string.
func (p *ParsedDatabaseURL) ToURL() string {
	return BuildDatabaseURL(p.Host, p.Port, p.User, p.Password, p.Database, p.SSLMode)
}
