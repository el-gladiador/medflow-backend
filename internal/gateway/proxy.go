package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/medflow/medflow-backend/pkg/config"
	"github.com/medflow/medflow-backend/pkg/errors"
	pkghttp "github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// Proxy handles reverse proxying to backend services
type Proxy struct {
	cfg       *config.Config
	log       *logger.Logger
	authProxy *httputil.ReverseProxy
	userProxy *httputil.ReverseProxy
	staffProxy *httputil.ReverseProxy
	inventoryProxy *httputil.ReverseProxy
}

// NewProxy creates a new proxy instance
func NewProxy(cfg *config.Config, log *logger.Logger) *Proxy {
	p := &Proxy{
		cfg: cfg,
		log: log,
	}

	p.authProxy = p.createProxy(cfg.Services.AuthServiceURL)
	p.userProxy = p.createProxy(cfg.Services.UserServiceURL)
	p.staffProxy = p.createProxy(cfg.Services.StaffServiceURL)
	p.inventoryProxy = p.createProxy(cfg.Services.InventoryServiceURL)

	return p
}

func (p *Proxy) createProxy(targetURL string) *httputil.ReverseProxy {
	target, _ := url.Parse(targetURL)

	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		p.log.Error().Err(err).Str("path", r.URL.Path).Msg("proxy error")
		pkghttp.Error(w, errors.Internal("service unavailable"))
	}

	return proxy
}

// ForwardToAuth forwards requests to the auth service
func (p *Proxy) ForwardToAuth(w http.ResponseWriter, r *http.Request) {
	p.authProxy.ServeHTTP(w, r)
}

// ForwardToUsers forwards requests to the user service
func (p *Proxy) ForwardToUsers(w http.ResponseWriter, r *http.Request) {
	p.userProxy.ServeHTTP(w, r)
}

// ForwardToUsersPublic forwards public requests to the user service
// These are unauthenticated endpoints (currently unused - invitations removed)
func (p *Proxy) ForwardToUsersPublic(w http.ResponseWriter, r *http.Request) {
	p.userProxy.ServeHTTP(w, r)
}

// ForwardToStaff forwards requests to the staff service
func (p *Proxy) ForwardToStaff(w http.ResponseWriter, r *http.Request) {
	p.staffProxy.ServeHTTP(w, r)
}

// ForwardToInventory forwards requests to the inventory service
func (p *Proxy) ForwardToInventory(w http.ResponseWriter, r *http.Request) {
	p.inventoryProxy.ServeHTTP(w, r)
}

// AuthMiddleware validates JWT tokens and adds user context
func (p *Proxy) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			pkghttp.Error(w, errors.Unauthorized("missing authorization header"))
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			pkghttp.Error(w, errors.Unauthorized("invalid authorization header format"))
			return
		}

		tokenString := parts[1]

		// Parse and validate token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(p.cfg.JWT.Secret), nil
		})

		if err != nil {
			p.log.Debug().Err(err).Msg("token validation failed")
			if strings.Contains(err.Error(), "expired") {
				pkghttp.Error(w, errors.TokenExpired())
			} else {
				pkghttp.Error(w, errors.TokenInvalid())
			}
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			pkghttp.Error(w, errors.TokenInvalid())
			return
		}

		// Extract user info from claims
		userID, _ := claims["sub"].(string)
		email, _ := claims["email"].(string)
		role, _ := claims["role"].(string)

		// Extract tenant info from claims (NEW - multi-tenancy support)
		tenantID, _ := claims["tenant_id"].(string)
		tenantSlug, _ := claims["tenant_slug"].(string)
		tenantSchema, _ := claims["tenant_schema"].(string)

		// Validate tenant context is present (CRITICAL for multi-tenancy security)
		// For now, we log a warning for old tokens without tenant context
		// TODO: Uncomment the error return below to enforce tenant requirement after full rollout
		if tenantID == "" || tenantSchema == "" {
			p.log.Warn().
				Str("user_id", userID).
				Msg("JWT missing tenant context - old token or misconfigured")
			// pkghttp.Error(w, errors.Forbidden("missing tenant context in token"))
			// return
		}

		// Add user info to request context
		ctx := pkghttp.WithUserContext(r.Context(), userID, email, role)

		// Add tenant info to request context (NEW)
		if tenantID != "" {
			ctx = tenant.WithTenantContext(ctx, tenantID, tenantSlug, tenantSchema)
		}

		// Add user info to headers for downstream services
		r.Header.Set("X-User-ID", userID)
		r.Header.Set("X-User-Email", email)
		r.Header.Set("X-User-Role", role)

		// Add tenant info to headers for downstream services (NEW)
		if tenantID != "" {
			r.Header.Set("X-Tenant-ID", tenantID)
			r.Header.Set("X-Tenant-Slug", tenantSlug)
			r.Header.Set("X-Tenant-Schema", tenantSchema)
		}

		// Add permissions if present
		if perms, ok := claims["permissions"].([]interface{}); ok {
			permStrings := make([]string, len(perms))
			for i, p := range perms {
				permStrings[i], _ = p.(string)
			}
			permsJSON, _ := json.Marshal(permStrings)
			r.Header.Set("X-User-Permissions", string(permsJSON))
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RateLimiter middleware for rate limiting (placeholder)
func (p *Proxy) RateLimiter(next http.Handler) http.Handler {
	// TODO: Implement rate limiting with Redis or in-memory store
	return next
}
