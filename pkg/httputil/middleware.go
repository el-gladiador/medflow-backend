package httputil

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
	UserEmailKey contextKey = "user_email"
	UserRoleKey  contextKey = "user_role"
)

// RequestID middleware adds a request ID to each request
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logger middleware logs HTTP requests
func Logger(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			requestID := GetRequestID(r.Context())
			userID := GetUserID(r.Context())

			log.Info().
				Str("request_id", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapped.statusCode).
				Dur("duration", duration).
				Str("user_id", userID).
				Str("remote_addr", r.RemoteAddr).
				Msg("HTTP request")
		})
	}
}

// Recoverer middleware recovers from panics
func Recoverer(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error().
						Interface("panic", err).
						Str("path", r.URL.Path).
						Msg("panic recovered")

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// GetUserID retrieves the user ID from context
func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}

// GetUserEmail retrieves the user email from context
func GetUserEmail(ctx context.Context) string {
	if email, ok := ctx.Value(UserEmailKey).(string); ok {
		return email
	}
	return ""
}

// GetUserRole retrieves the user role from context
func GetUserRole(ctx context.Context) string {
	if role, ok := ctx.Value(UserRoleKey).(string); ok {
		return role
	}
	return ""
}

// WithUserContext adds user information to the context
func WithUserContext(ctx context.Context, userID, email, role string) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, userID)
	ctx = context.WithValue(ctx, UserEmailKey, email)
	ctx = context.WithValue(ctx, UserRoleKey, role)
	return ctx
}

// TenantMiddleware extracts tenant context from headers (set by API Gateway)
// and adds it to the request context.
//
// This middleware is applied to all microservices (user, staff, inventory).
// It ensures every request has tenant context for database schema isolation.
//
// Headers expected (set by gateway's AuthMiddleware):
//   - X-Tenant-ID: Tenant UUID
//   - X-Tenant-Slug: Tenant slug (e.g., "test-practice")
//   - X-Tenant-Schema: Schema name (e.g., "tenant_test_practice")
//
// Security: Missing tenant context returns 403 Forbidden (fail-fast).
// Exception: /health endpoints are allowed without tenant context for monitoring.
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip tenant validation for health check endpoints
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		tenantID := r.Header.Get("X-Tenant-ID")
		tenantSlug := r.Header.Get("X-Tenant-Slug")
		tenantSchema := r.Header.Get("X-Tenant-Schema")

		// Validate tenant context is present
		// This is CRITICAL for security - prevents cross-tenant data access
		if tenantID == "" || tenantSchema == "" {
			http.Error(w, `{"error":"missing tenant context"}`, http.StatusForbidden)
			return
		}

		// Add tenant context to request
		ctx := tenant.WithTenantContext(r.Context(), tenantID, tenantSlug, tenantSchema)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
