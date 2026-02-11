package config

import (
	"testing"
)

func TestParseDatabaseURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		want     *ParsedDatabaseURL
		wantErr  bool
	}{
		{
			name: "standard postgres URL",
			url:  "postgres://medflow_app:devpassword@localhost:5432/medflow?sslmode=disable",
			want: &ParsedDatabaseURL{
				Host:     "localhost",
				Port:     5432,
				User:     "medflow_app",
				Password: "devpassword",
				Database: "medflow",
				SSLMode:  "disable",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "postgresql scheme",
			url:  "postgresql://user:pass@db.example.com:5432/mydb?sslmode=require",
			want: &ParsedDatabaseURL{
				Host:     "db.example.com",
				Port:     5432,
				User:     "user",
				Password: "pass",
				Database: "mydb",
				SSLMode:  "require",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "default port when not specified",
			url:  "postgres://user:pass@localhost/mydb?sslmode=disable",
			want: &ParsedDatabaseURL{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				Database: "mydb",
				SSLMode:  "disable",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "AWS RDS URL with sslmode require",
			url:  "postgres://medflow_prod_app:securepass@medflow.cluster-xxxx.eu-central-1.rds.amazonaws.com:5432/medflow?sslmode=require",
			want: &ParsedDatabaseURL{
				Host:     "medflow.cluster-xxxx.eu-central-1.rds.amazonaws.com",
				Port:     5432,
				User:     "medflow_prod_app",
				Password: "securepass",
				Database: "medflow",
				SSLMode:  "require",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "URL with additional options",
			url:  "postgres://user:pass@localhost:5432/db?sslmode=disable&search_path=tenant_test",
			want: &ParsedDatabaseURL{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				Database: "db",
				SSLMode:  "disable",
				Options:  map[string]string{"search_path": "tenant_test"},
			},
			wantErr: false,
		},
		{
			name:    "empty URL",
			url:     "",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid scheme",
			url:     "mysql://user:pass@localhost/db",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDatabaseURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDatabaseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Host != tt.want.Host {
				t.Errorf("Host = %v, want %v", got.Host, tt.want.Host)
			}
			if got.Port != tt.want.Port {
				t.Errorf("Port = %v, want %v", got.Port, tt.want.Port)
			}
			if got.User != tt.want.User {
				t.Errorf("User = %v, want %v", got.User, tt.want.User)
			}
			if got.Password != tt.want.Password {
				t.Errorf("Password = %v, want %v", got.Password, tt.want.Password)
			}
			if got.Database != tt.want.Database {
				t.Errorf("Database = %v, want %v", got.Database, tt.want.Database)
			}
			if got.SSLMode != tt.want.SSLMode {
				t.Errorf("SSLMode = %v, want %v", got.SSLMode, tt.want.SSLMode)
			}
		})
	}
}

func TestBuildDatabaseURL(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		user     string
		password string
		database string
		sslMode  string
		want     string
	}{
		{
			name:     "standard local",
			host:     "localhost",
			port:     5432,
			user:     "medflow_app",
			password: "devpassword",
			database: "medflow",
			sslMode:  "disable",
			want:     "postgres://medflow_app:devpassword@localhost:5432/medflow?sslmode=disable",
		},
		{
			name:     "AWS RDS",
			host:     "db.eu-central-1.rds.amazonaws.com",
			port:     5432,
			user:     "medflow_prod_app",
			password: "securepass",
			database: "medflow",
			sslMode:  "require",
			want:     "postgres://medflow_prod_app:securepass@db.eu-central-1.rds.amazonaws.com:5432/medflow?sslmode=require",
		},
		{
			name:     "password with special chars",
			host:     "localhost",
			port:     5432,
			user:     "user",
			password: "pass@word#123",
			database: "db",
			sslMode:  "disable",
			want:     "postgres://user:pass%40word%23123@localhost:5432/db?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDatabaseURL(tt.host, tt.port, tt.user, tt.password, tt.database, tt.sslMode)
			if got != tt.want {
				t.Errorf("BuildDatabaseURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsedDatabaseURL_ToDSN(t *testing.T) {
	parsed := &ParsedDatabaseURL{
		Host:     "localhost",
		Port:     5432,
		User:     "medflow_app",
		Password: "devpassword",
		Database: "medflow",
		SSLMode:  "disable",
		Options:  map[string]string{},
	}

	dsn := parsed.ToDSN()
	expected := "host=localhost port=5432 user=medflow_app password=devpassword dbname=medflow sslmode=disable"

	if dsn != expected {
		t.Errorf("ToDSN() = %v, want %v", dsn, expected)
	}
}

func TestRoundTrip(t *testing.T) {
	// Test that parsing a URL and converting back gives the same result
	original := "postgres://medflow_app:devpassword@localhost:5432/medflow?sslmode=disable"

	parsed, err := ParseDatabaseURL(original)
	if err != nil {
		t.Fatalf("ParseDatabaseURL() error = %v", err)
	}

	rebuilt := parsed.ToURL()
	if rebuilt != original {
		t.Errorf("Round trip failed: got %v, want %v", rebuilt, original)
	}
}
