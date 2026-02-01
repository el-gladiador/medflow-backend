package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
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
	TenantID     string `json:"tenant_id,omitempty"`
	TenantSlug   string `json:"tenant_slug,omitempty"`
	TenantSchema string `json:"tenant_schema,omitempty"`
}

// FullName returns the user's full name
func (u *UserInfo) FullName() string {
	return u.FirstName + " " + u.LastName
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, req *LoginRequest, userAgent, ipAddress string) (*LoginResponse, error) {
	// Call user service to validate credentials
	user, err := s.validateCredentials(ctx, req.Email, req.Password)
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
		TenantID:     user.TenantID,
		TenantSlug:   user.TenantSlug,
		TenantSchema: user.TenantSchema,
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
	user, err := s.getUserInfo(ctx, claims.UserID, claims.TenantID, claims.TenantSlug, claims.TenantSchema)
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
		TenantID:     claims.TenantID,
		TenantSlug:   claims.TenantSlug,
		TenantSchema: claims.TenantSchema,
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
func (s *AuthService) GetCurrentUser(ctx context.Context, userID, tenantID, tenantSlug, tenantSchema string) (*UserInfo, error) {
	return s.getUserInfo(ctx, userID, tenantID, tenantSlug, tenantSchema)
}

// validateCredentials validates user credentials against user service
// Uses the user-tenant lookup table for O(1) tenant resolution
func (s *AuthService) validateCredentials(ctx context.Context, email, password string) (*UserInfo, error) {
	// Step 1: O(1) tenant lookup using the user-tenant lookup table
	lookup, err := s.lookupRepo.GetByEmail(ctx, email)
	if err != nil {
		s.logger.Debug().Str("email", email).Msg("email not found in lookup table")
		// Return generic invalid credentials to avoid email enumeration
		return nil, errors.InvalidCredentials()
	}

	s.logger.Debug().
		Str("email", email).
		Str("tenant_id", lookup.TenantID).
		Str("tenant_schema", lookup.TenantSchema).
		Msg("tenant resolved from lookup table")

	// Step 2: Call user service WITH tenant headers for tenant-scoped validation
	url := fmt.Sprintf("%s/api/v1/internal/validate-credentials", s.config.Services.UserServiceURL)

	requestBody := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{Email: email, Password: password}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, errors.Internal("failed to encode request")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, errors.Internal("failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")

	// CRITICAL: Forward tenant headers for Schema-per-Tenant isolation
	req.Header.Set("X-Tenant-ID", lookup.TenantID)
	req.Header.Set("X-Tenant-Slug", lookup.TenantSlug)
	req.Header.Set("X-Tenant-Schema", lookup.TenantSchema)

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
		result.Data.TenantSchema = lookup.TenantSchema
	}

	return result.Data, nil
}

// getUserInfo fetches user info from user service
func (s *AuthService) getUserInfo(ctx context.Context, userID, tenantID, tenantSlug, tenantSchema string) (*UserInfo, error) {
	url := fmt.Sprintf("%s/api/v1/internal/users/%s", s.config.Services.UserServiceURL, userID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Internal("failed to create request")
	}

	// Forward tenant headers to user service (required for Schema-per-Tenant isolation)
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	if tenantSlug != "" {
		req.Header.Set("X-Tenant-Slug", tenantSlug)
	}
	if tenantSchema != "" {
		req.Header.Set("X-Tenant-Schema", tenantSchema)
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
