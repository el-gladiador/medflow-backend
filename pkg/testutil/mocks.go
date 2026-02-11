package testutil

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

// MockDB wraps sqlmock for easier testing
type MockDB struct {
	DB   *sqlx.DB
	Mock sqlmock.Sqlmock
}

// NewMockDB creates a new mock database for unit testing.
// Use this when you want to test repository logic without a real database.
//
// Usage:
//
//	mockDB := testutil.NewMockDB(t)
//	defer mockDB.Close()
//
//	// Set up expectations
//	mockDB.ExpectQuery("SELECT").WillReturnRows(...)
//
//	// Use mockDB.DB with your repository
//	repo := repository.NewUserRepository(mockDB.DB)
func NewMockDB(t *testing.T) *MockDB {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	sqlxDB := sqlx.NewDb(db, "postgres")

	return &MockDB{
		DB:   sqlxDB,
		Mock: mock,
	}
}

// Close closes the mock database connection
func (m *MockDB) Close() error {
	return m.DB.Close()
}

// ExpectQuery sets up an expected query
func (m *MockDB) ExpectQuery(query string) *sqlmock.ExpectedQuery {
	return m.Mock.ExpectQuery(regexp.QuoteMeta(query))
}

// ExpectExec sets up an expected exec
func (m *MockDB) ExpectExec(query string) *sqlmock.ExpectedExec {
	return m.Mock.ExpectExec(regexp.QuoteMeta(query))
}

// ExpectBegin sets up an expected transaction begin
func (m *MockDB) ExpectBegin() *sqlmock.ExpectedBegin {
	return m.Mock.ExpectBegin()
}

// ExpectCommit sets up an expected commit
func (m *MockDB) ExpectCommit() *sqlmock.ExpectedCommit {
	return m.Mock.ExpectCommit()
}

// ExpectRollback sets up an expected rollback
func (m *MockDB) ExpectRollback() *sqlmock.ExpectedRollback {
	return m.Mock.ExpectRollback()
}

// ExpectationsWereMet verifies all expectations were met
func (m *MockDB) ExpectationsWereMet(t *testing.T) {
	if err := m.Mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled mock expectations: %v", err)
	}
}

// MockRows creates a new mock rows object
func MockRows(columns ...string) *sqlmock.Rows {
	return sqlmock.NewRows(columns)
}

// ExpectTenantQuery sets up expectations for a tenant-scoped query using RLS.
// This handles the transaction + SET LOCAL search_path + SET LOCAL app.current_tenant pattern.
//
// Usage:
//
//	mockDB.ExpectTenantQuery("users, public", "test-tenant-id",
//	    "SELECT * FROM users WHERE id = $1",
//	    testutil.MockRows("id", "email").AddRow(userID, email),
//	)
func (m *MockDB) ExpectTenantQuery(searchPath, tenantID, query string, rows *sqlmock.Rows) {
	m.Mock.ExpectBegin()
	m.Mock.ExpectExec(regexp.QuoteMeta("SET LOCAL search_path TO " + searchPath)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	m.Mock.ExpectExec(regexp.QuoteMeta("SET LOCAL app.current_tenant = $1")).
		WithArgs(tenantID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	m.Mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows)
	m.Mock.ExpectCommit()
}

// ExpectTenantExec sets up expectations for a tenant-scoped exec using RLS.
// This handles the transaction + SET LOCAL search_path + SET LOCAL app.current_tenant pattern.
func (m *MockDB) ExpectTenantExec(searchPath, tenantID, query string, result driver.Result) {
	m.Mock.ExpectBegin()
	m.Mock.ExpectExec(regexp.QuoteMeta("SET LOCAL search_path TO " + searchPath)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	m.Mock.ExpectExec(regexp.QuoteMeta("SET LOCAL app.current_tenant = $1")).
		WithArgs(tenantID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	m.Mock.ExpectExec(regexp.QuoteMeta(query)).WillReturnResult(result)
	m.Mock.ExpectCommit()
}

// Deprecated: ExpectTenantQuerySchema uses the old schema-per-tenant pattern.
// Use ExpectTenantQuery with RLS instead.
func (m *MockDB) ExpectTenantQuerySchema(schema, query string, rows *sqlmock.Rows) {
	m.Mock.ExpectBegin()
	m.Mock.ExpectExec(regexp.QuoteMeta("SET LOCAL search_path TO " + schema + ", public")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	m.Mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows)
	m.Mock.ExpectCommit()
}

// Deprecated: ExpectTenantExecSchema uses the old schema-per-tenant pattern.
// Use ExpectTenantExec with RLS instead.
func (m *MockDB) ExpectTenantExecSchema(schema, query string, result driver.Result) {
	m.Mock.ExpectBegin()
	m.Mock.ExpectExec(regexp.QuoteMeta("SET LOCAL search_path TO " + schema + ", public")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	m.Mock.ExpectExec(regexp.QuoteMeta(query)).WillReturnResult(result)
	m.Mock.ExpectCommit()
}

// AnyTime is a matcher for any time.Time value
type AnyTime struct{}

// Match satisfies the sqlmock.Argument interface
func (a AnyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

// AnyUUID is a matcher for any UUID string
type AnyUUID struct{}

// Match satisfies the sqlmock.Argument interface
func (a AnyUUID) Match(v driver.Value) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	// Simple UUID format check
	matched, _ := regexp.MatchString(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, s)
	return matched
}

// MockPublisher is a mock event publisher for testing
type MockPublisher struct {
	PublishedEvents []PublishedEvent
}

// PublishedEvent represents an event that was published
type PublishedEvent struct {
	Type    string
	Payload interface{}
}

// NewMockPublisher creates a new mock publisher
func NewMockPublisher() *MockPublisher {
	return &MockPublisher{
		PublishedEvents: make([]PublishedEvent, 0),
	}
}

// Publish records an event for later verification
func (m *MockPublisher) Publish(ctx context.Context, eventType string, payload interface{}) error {
	m.PublishedEvents = append(m.PublishedEvents, PublishedEvent{
		Type:    eventType,
		Payload: payload,
	})
	return nil
}

// AssertEventPublished checks if an event of the given type was published
func (m *MockPublisher) AssertEventPublished(t *testing.T, eventType string) {
	t.Helper()
	for _, e := range m.PublishedEvents {
		if e.Type == eventType {
			return
		}
	}
	t.Errorf("expected event %q to be published, but it wasn't", eventType)
}

// AssertNoEventsPublished checks that no events were published
func (m *MockPublisher) AssertNoEventsPublished(t *testing.T) {
	t.Helper()
	if len(m.PublishedEvents) > 0 {
		t.Errorf("expected no events, but got %d: %+v", len(m.PublishedEvents), m.PublishedEvents)
	}
}

// Reset clears all published events
func (m *MockPublisher) Reset() {
	m.PublishedEvents = make([]PublishedEvent, 0)
}

// MockLogger is a no-op logger for tests
type MockLogger struct {
	Entries []LogEntry
}

// LogEntry represents a logged message
type LogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

// NewMockLogger creates a new mock logger
func NewMockLogger() *MockLogger {
	return &MockLogger{
		Entries: make([]LogEntry, 0),
	}
}

// Info logs an info message
func (m *MockLogger) Info(msg string, fields ...interface{}) {
	m.Entries = append(m.Entries, LogEntry{Level: "info", Message: msg})
}

// Error logs an error message
func (m *MockLogger) Error(msg string, fields ...interface{}) {
	m.Entries = append(m.Entries, LogEntry{Level: "error", Message: msg})
}

// Debug logs a debug message
func (m *MockLogger) Debug(msg string, fields ...interface{}) {
	m.Entries = append(m.Entries, LogEntry{Level: "debug", Message: msg})
}

// Warn logs a warning message
func (m *MockLogger) Warn(msg string, fields ...interface{}) {
	m.Entries = append(m.Entries, LogEntry{Level: "warn", Message: msg})
}
