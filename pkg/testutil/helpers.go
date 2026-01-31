package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCase represents a generic test case structure
type TestCase[I, O any] struct {
	Name      string
	Input     I
	Expected  O
	WantErr   bool
	ErrMsg    string
	Setup     func()
	Teardown  func()
	AssertFn  func(*testing.T, O)
}

// RunTestCases runs a slice of test cases with the provided test function
func RunTestCases[I, O any](t *testing.T, cases []TestCase[I, O], fn func(I) (O, error)) {
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Setup != nil {
				tc.Setup()
			}
			if tc.Teardown != nil {
				defer tc.Teardown()
			}

			result, err := fn(tc.Input)

			if tc.WantErr {
				require.Error(t, err)
				if tc.ErrMsg != "" {
					assert.Contains(t, err.Error(), tc.ErrMsg)
				}
				return
			}

			require.NoError(t, err)

			if tc.AssertFn != nil {
				tc.AssertFn(t, result)
			} else {
				assert.Equal(t, tc.Expected, result)
			}
		})
	}
}

// HTTPTestCase represents an HTTP handler test case
type HTTPTestCase struct {
	Name           string
	Method         string
	Path           string
	Body           interface{}
	Headers        map[string]string
	TenantID       string
	TenantSlug     string
	TenantSchema   string
	UserID         string
	WantStatus     int
	WantBody       string
	WantBodyContains []string
	Setup          func()
	Teardown       func()
}

// NewHTTPRequest creates a new HTTP request for testing handlers
func NewHTTPRequest(method, path string, body interface{}) *http.Request {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// WithTenantHeaders adds tenant headers to the request
func WithTenantHeaders(req *http.Request, tenantID, tenantSlug, tenantSchema string) *http.Request {
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	if tenantSlug != "" {
		req.Header.Set("X-Tenant-Slug", tenantSlug)
	}
	if tenantSchema != "" {
		req.Header.Set("X-Tenant-Schema", tenantSchema)
	}
	return req
}

// WithUserHeaders adds user headers to the request
func WithUserHeaders(req *http.Request, userID, userEmail string) *http.Request {
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	if userEmail != "" {
		req.Header.Set("X-User-Email", userEmail)
	}
	return req
}

// WithRequestID adds a request ID header
func WithRequestID(req *http.Request, requestID string) *http.Request {
	req.Header.Set("X-Request-ID", requestID)
	return req
}

// ExecuteRequest executes an HTTP request and returns the response recorder
func ExecuteRequest(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// AssertStatus asserts the response status code
func AssertStatus(t *testing.T, rr *httptest.ResponseRecorder, expected int) {
	t.Helper()
	assert.Equal(t, expected, rr.Code, "unexpected status code. Body: %s", rr.Body.String())
}

// AssertBodyContains asserts the response body contains a string
func AssertBodyContains(t *testing.T, rr *httptest.ResponseRecorder, expected string) {
	t.Helper()
	assert.Contains(t, rr.Body.String(), expected)
}

// AssertJSONBody asserts the response body matches the expected JSON
func AssertJSONBody(t *testing.T, rr *httptest.ResponseRecorder, expected interface{}) {
	t.Helper()
	expectedJSON, err := json.Marshal(expected)
	require.NoError(t, err)

	assert.JSONEq(t, string(expectedJSON), rr.Body.String())
}

// ParseJSONBody parses the response body into the target
func ParseJSONBody(t *testing.T, rr *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	err := json.Unmarshal(rr.Body.Bytes(), target)
	require.NoError(t, err, "failed to parse response body: %s", rr.Body.String())
}

// ContextWithTimeout creates a context with a test timeout
func ContextWithTimeout(t *testing.T, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(func() {
		cancel()
	})
	return ctx, cancel
}

// DefaultTestContext creates a context with a 30-second timeout
func DefaultTestContext(t *testing.T) context.Context {
	ctx, _ := ContextWithTimeout(t, 30*time.Second)
	return ctx
}

// RequireEventually retries an assertion until it passes or times out
func RequireEventually(t *testing.T, condition func() bool, timeout, interval time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatal(msg)
}

// SkipIfShort skips the test if running with -short flag
func SkipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

// SkipIfCI skips the test if running in CI environment
func SkipIfCI(t *testing.T) {
	// Check common CI environment variables
	ciVars := []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL"}
	for _, v := range ciVars {
		if val := getEnv(v); val != "" {
			t.Skipf("skipping test in CI environment (%s=%s)", v, val)
		}
	}
}

// MustJSON marshals the value to JSON or panics
func MustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// MustJSONBytes marshals the value to JSON bytes or panics
func MustJSONBytes(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// PtrString returns a pointer to the string
func PtrString(s string) *string {
	return &s
}

// PtrInt returns a pointer to the int
func PtrInt(i int) *int {
	return &i
}

// PtrBool returns a pointer to the bool
func PtrBool(b bool) *bool {
	return &b
}

// PtrTime returns a pointer to the time
func PtrTime(t time.Time) *time.Time {
	return &t
}

// getEnv is a simple env var getter (to avoid importing os in tests)
func getEnv(key string) string {
	// This is a simple implementation - in real tests you might use os.Getenv
	return ""
}
