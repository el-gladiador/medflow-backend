package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/config"
	"github.com/medflow/medflow-backend/pkg/errors"
)

// Claims represents the JWT claims
type Claims struct {
	jwt.RegisteredClaims
	UserID      string   `json:"user_id"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions,omitempty"`
	IsManager   bool     `json:"is_manager"`

	// Tenant context - added for multi-tenancy support
	TenantID     string `json:"tenant_id"`
	TenantSlug   string `json:"tenant_slug"`
	TenantSchema string `json:"tenant_schema"`
}

// RefreshClaims represents refresh token claims
type RefreshClaims struct {
	jwt.RegisteredClaims
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`

	// Tenant context - needed for token refresh to call user service
	TenantID     string `json:"tenant_id"`
	TenantSlug   string `json:"tenant_slug"`
	TenantSchema string `json:"tenant_schema"`
}

// Manager handles JWT operations
type Manager struct {
	config *config.JWTConfig
}

// NewManager creates a new JWT manager
func NewManager(cfg *config.JWTConfig) *Manager {
	return &Manager{config: cfg}
}

// UserInfo contains user information for token generation
type UserInfo struct {
	ID          string
	Email       string
	Name        string
	Role        string
	Permissions []string
	IsManager   bool

	// Tenant context - populated during login
	TenantID     string
	TenantSlug   string
	TenantSchema string
}

// TokenPair contains access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// GenerateTokenPair generates both access and refresh tokens
func (m *Manager) GenerateTokenPair(user *UserInfo, sessionID string) (*TokenPair, error) {
	now := time.Now()
	accessExpiry := now.Add(m.config.AccessExpiry)
	refreshExpiry := now.Add(m.config.RefreshExpiry)

	// Generate access token
	accessClaims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.config.Issuer,
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
		UserID:      user.ID,
		Email:       user.Email,
		Name:        user.Name,
		Role:        user.Role,
		Permissions: user.Permissions,
		IsManager:   user.IsManager,

		// Include tenant context in JWT
		TenantID:     user.TenantID,
		TenantSlug:   user.TenantSlug,
		TenantSchema: user.TenantSchema,
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(m.config.Secret))
	if err != nil {
		return nil, err
	}

	// Generate refresh token
	refreshClaims := RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.config.Issuer,
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(refreshExpiry),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
		UserID:    user.ID,
		SessionID: sessionID,

		// Include tenant context for refresh flow
		TenantID:     user.TenantID,
		TenantSlug:   user.TenantSlug,
		TenantSchema: user.TenantSchema,
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(m.config.Secret))
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresAt:    accessExpiry,
		TokenType:    "Bearer",
	}, nil
}

// ValidateAccessToken validates an access token and returns the claims
func (m *Manager) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.TokenInvalid()
		}
		return []byte(m.config.Secret), nil
	})

	if err != nil {
		if err.Error() == "token has invalid claims: token is expired" {
			return nil, errors.TokenExpired()
		}
		return nil, errors.TokenInvalid()
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.TokenInvalid()
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token and returns the claims
func (m *Manager) ValidateRefreshToken(tokenString string) (*RefreshClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RefreshClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.TokenInvalid()
		}
		return []byte(m.config.Secret), nil
	})

	if err != nil {
		if err.Error() == "token has invalid claims: token is expired" {
			return nil, errors.TokenExpired()
		}
		return nil, errors.TokenInvalid()
	}

	claims, ok := token.Claims.(*RefreshClaims)
	if !ok || !token.Valid {
		return nil, errors.TokenInvalid()
	}

	return claims, nil
}

// GetTokenExpiry returns the access token expiry duration
func (m *Manager) GetTokenExpiry() time.Duration {
	return m.config.AccessExpiry
}

// GetRefreshExpiry returns the refresh token expiry duration
func (m *Manager) GetRefreshExpiry() time.Duration {
	return m.config.RefreshExpiry
}
