package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/auth/jwt"
	"github.com/medflow/medflow-backend/internal/auth/repository"
	"github.com/medflow/medflow-backend/pkg/config"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return uuid.New().String()
}

// AuthService handles authentication logic
type AuthService struct {
	repo       *repository.SessionRepository
	lookupRepo *repository.UserTenantLookupRepository
	jwtManager *jwt.Manager
	config     *config.Config
	logger     *logger.Logger
}

// NewAuthService creates a new auth service
func NewAuthService(
	repo *repository.SessionRepository,
	lookupRepo *repository.UserTenantLookupRepository,
	jwtManager *jwt.Manager,
	cfg *config.Config,
	log *logger.Logger,
) *AuthService {
	return &AuthService{
		repo:       repo,
		lookupRepo: lookupRepo,
		jwtManager: jwtManager,
		config:     cfg,
		logger:     log,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Identifier string  `json:"identifier" validate:"required,min=1"` // Email or username
	Password   string  `json:"password" validate:"required,min=6"`
	TenantSlug *string `json:"tenant_slug,omitempty"` // From subdomain (required for username login)
}

// LoginResponse represents a login response
type LoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
	User         *UserInfo `json:"user"`
}

// UserInfo represents user information
type UserInfo struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	FirstName   string   `json:"first_name"`
	LastName    string   `json:"last_name"`
	AvatarURL   string   `json:"avatar_url"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	IsManager   bool     `json:"is_manager"`

	// Tenant context - populated by user service during login
	TenantID   string `json:"tenant_id,omitempty"`
	TenantSlug string `json:"tenant_slug,omitempty"`
}

// FullName returns the user's full name
func (u *UserInfo) FullName() string {
	return u.FirstName + " " + u.LastName
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, req *LoginRequest, userAgent, ipAddress string) (*LoginResponse, error) {
	// Call user service to validate credentials (identifier can be email or username)
	user, err := s.validateCredentials(ctx, req.Identifier, req.Password, req.TenantSlug)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(s.jwtManager.GetRefreshExpiry())

	// Generate tokens with tenant context
	tokenInfo := &jwt.UserInfo{
		ID:          user.ID,
		Email:       user.Email,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Role:        user.Role,
		Permissions: user.Permissions,
		IsManager:   user.IsManager,

		// Pass tenant context from user service to JWT
		TenantID:   user.TenantID,
		TenantSlug: user.TenantSlug,
	}

	// Generate a session ID first
	sessionID := generateSessionID()

	// Generate tokens with the session ID
	tokens, err := s.jwtManager.GenerateTokenPair(tokenInfo, sessionID)
	if err != nil {
		return nil, errors.Internal("failed to generate tokens")
	}

	// Create session with the actual refresh token
	_, err = s.repo.CreateWithID(ctx, sessionID, user.ID, tokens.RefreshToken, expiresAt, userAgent, ipAddress)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to create session")
		return nil, errors.Internal("failed to create session")
	}

	return &LoginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
		TokenType:    tokens.TokenType,
		User:         user,
	}, nil
}

// Logout invalidates a session
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if err := s.repo.RevokeByRefreshToken(ctx, refreshToken); err != nil {
		s.logger.Warn().Err(err).Msg("failed to revoke session")
	}
	return nil
}

// Refresh refreshes the access token using a refresh token
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*jwt.TokenPair, error) {
	// Validate refresh token
	claims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Get session
	session, err := s.repo.GetByRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, errors.Unauthorized("invalid session")
	}

	// Check session is not revoked
	if session.RevokedAt != nil {
		return nil, errors.Unauthorized("session revoked")
	}

	// Get user info from user service (pass tenant context from refresh token claims)
	user, err := s.getUserInfo(ctx, claims.UserID, claims.TenantID, claims.TenantSlug)
	if err != nil {
		return nil, err
	}

	// Generate new tokens (preserve tenant context from claims)
	tokenInfo := &jwt.UserInfo{
		ID:          user.ID,
		Email:       user.Email,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Role:        user.Role,
		Permissions: user.Permissions,
		IsManager:   user.IsManager,

		// Preserve tenant context for new tokens
		TenantID:   claims.TenantID,
		TenantSlug: claims.TenantSlug,
	}

	tokens, err := s.jwtManager.GenerateTokenPair(tokenInfo, session.ID)
	if err != nil {
		return nil, errors.Internal("failed to generate tokens")
	}

	// CRITICAL: Update session with new refresh token hash for token rotation
	if err := s.repo.UpdateRefreshTokenHash(ctx, session.ID, tokens.RefreshToken); err != nil {
		s.logger.Error().Err(err).Msg("failed to update refresh token hash")
		return nil, errors.Internal("failed to update session")
	}

	return tokens, nil
}

// GetCurrentUser gets the current user from token claims
func (s *AuthService) GetCurrentUser(ctx context.Context, userID, tenantID, tenantSlug string) (*UserInfo, error) {
	return s.getUserInfo(ctx, userID, tenantID, tenantSlug)
}

// isEmail checks if the identifier looks like an email address
func isEmail(identifier string) bool {
	return strings.Contains(identifier, "@")
}

// validateCredentials validates user credentials against user service
// Supports both email and username login:
// - Email: Uses O(1) lookup table for tenant resolution (tenant_slug optional but validated if provided)
// - Username: Requires tenant_slug from subdomain since username is only unique within tenant
func (s *AuthService) validateCredentials(ctx context.Context, identifier, password string, tenantSlug *string) (*UserInfo, error) {
	var lookup *repository.UserTenantLookup
	var err error

	if isEmail(identifier) {
		// O(1) tenant lookup using the user-tenant lookup table
		lookup, err = s.lookupRepo.GetByEmail(ctx, identifier)
		if err != nil {
			s.logger.Debug().Str("email", identifier).Msg("email not found in lookup table")
			return nil, errors.InvalidCredentials()
		}

		// If tenant_slug was provided (from subdomain), validate it matches
		if tenantSlug != nil && *tenantSlug != "" && *tenantSlug != lookup.TenantSlug {
			s.logger.Debug().
				Str("email", identifier).
				Str("expected_tenant", *tenantSlug).
				Str("actual_tenant", lookup.TenantSlug).
				Msg("tenant mismatch: email belongs to different tenant")
			return nil, errors.BadRequest("tenant_mismatch")
		}

		s.logger.Debug().
			Str("email", identifier).
			Str("tenant_id", lookup.TenantID).
			Str("tenant_slug", lookup.TenantSlug).
			Msg("tenant resolved from lookup table (email)")
	} else {
		// Username login: REQUIRES tenant_slug from subdomain
		// Username is only unique within a tenant (e.g., "admin" exists in many clinics)
		if tenantSlug == nil || *tenantSlug == "" {
			s.logger.Debug().
				Str("username", identifier).
				Msg("username login attempted without tenant_slug (subdomain required)")
			return nil, errors.BadRequest("username_requires_subdomain")
		}

		// Lookup by username AND tenant slug
		lookup, err = s.lookupRepo.GetByUsernameAndSlug(ctx, identifier, *tenantSlug)
		if err != nil {
			s.logger.Debug().
				Str("username", identifier).
				Str("tenant_slug", *tenantSlug).
				Msg("username not found in tenant")
			return nil, errors.InvalidCredentials()
		}

		s.logger.Debug().
			Str("username", identifier).
			Str("tenant_id", lookup.TenantID).
			Str("tenant_slug", lookup.TenantSlug).
			Msg("tenant resolved from lookup table (username + tenant_slug)")
	}

	// Call user service WITH tenant headers for tenant-scoped validation
	url := fmt.Sprintf("%s/api/v1/internal/validate-credentials", s.config.Services.UserServiceURL)

	requestBody := struct {
		Identifier string `json:"identifier"` // Can be email or username
		Password   string `json:"password"`
	}{Identifier: identifier, Password: password}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, errors.Internal("failed to encode request")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, errors.Internal("failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")

	// Forward tenant headers for RLS isolation
	req.Header.Set("X-Tenant-ID", lookup.TenantID)
	req.Header.Set("X-Tenant-Slug", lookup.TenantSlug)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to call user service")
		return nil, errors.Internal("authentication service unavailable")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.InvalidCredentials()
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Internal("failed to validate credentials")
	}

	var result struct {
		Success bool      `json:"success"`
		Data    *UserInfo `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.Internal("failed to parse response")
	}

	// Ensure tenant context is populated in response
	if result.Data != nil {
		result.Data.TenantID = lookup.TenantID
		result.Data.TenantSlug = lookup.TenantSlug
	}

	return result.Data, nil
}

// getUserInfo fetches user info from user service
func (s *AuthService) getUserInfo(ctx context.Context, userID, tenantID, tenantSlug string) (*UserInfo, error) {
	url := fmt.Sprintf("%s/api/v1/internal/users/%s", s.config.Services.UserServiceURL, userID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Internal("failed to create request")
	}

	// Forward tenant headers to user service (required for RLS-based tenant isolation)
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	if tenantSlug != "" {
		req.Header.Set("X-Tenant-Slug", tenantSlug)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to call user service")
		return nil, errors.Internal("user service unavailable")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.NotFound("user")
	}

	if resp.StatusCode != http.StatusOK {
		s.logger.Error().Int("status", resp.StatusCode).Str("user_id", userID).Msg("user service returned error")
		return nil, errors.Internal("failed to get user info")
	}

	var result struct {
		Success bool      `json:"success"`
		Data    *UserInfo `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.Internal("failed to parse response")
	}

	return result.Data, nil
}
