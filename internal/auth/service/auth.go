package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	jwtManager *jwt.Manager
	config     *config.Config
	logger     *logger.Logger
}

// NewAuthService creates a new auth service
func NewAuthService(repo *repository.SessionRepository, jwtManager *jwt.Manager, cfg *config.Config, log *logger.Logger) *AuthService {
	return &AuthService{
		repo:       repo,
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
	Name        string   `json:"name"`
	Avatar      string   `json:"avatar"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	IsManager   bool     `json:"is_manager"`
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, req *LoginRequest, userAgent, ipAddress string) (*LoginResponse, error) {
	// Call user service to validate credentials
	user, err := s.validateCredentials(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(s.jwtManager.GetRefreshExpiry())

	// Generate tokens
	tokenInfo := &jwt.UserInfo{
		ID:          user.ID,
		Email:       user.Email,
		Name:        user.Name,
		Role:        user.Role,
		Permissions: user.Permissions,
		IsManager:   user.IsManager,
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

	// Get user info from user service
	user, err := s.getUserInfo(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	// Update last used
	s.repo.UpdateLastUsed(ctx, session.ID)

	// Generate new tokens
	tokenInfo := &jwt.UserInfo{
		ID:          user.ID,
		Email:       user.Email,
		Name:        user.Name,
		Role:        user.Role,
		Permissions: user.Permissions,
		IsManager:   user.IsManager,
	}

	return s.jwtManager.GenerateTokenPair(tokenInfo, session.ID)
}

// GetCurrentUser gets the current user from token claims
func (s *AuthService) GetCurrentUser(ctx context.Context, userID string) (*UserInfo, error) {
	return s.getUserInfo(ctx, userID)
}

// validateCredentials validates user credentials against user service
func (s *AuthService) validateCredentials(ctx context.Context, email, password string) (*UserInfo, error) {
	url := fmt.Sprintf("%s/api/v1/internal/validate-credentials", s.config.Services.UserServiceURL)

	body := fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, errors.Internal("failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(stringReader(body))

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

	return result.Data, nil
}

// getUserInfo fetches user info from user service
func (s *AuthService) getUserInfo(ctx context.Context, userID string) (*UserInfo, error) {
	url := fmt.Sprintf("%s/api/v1/internal/users/%s", s.config.Services.UserServiceURL, userID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Internal("failed to create request")
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

type stringReaderType struct {
	s string
	i int
}

func stringReader(s string) io.Reader {
	return &stringReaderType{s: s}
}

func (sr *stringReaderType) Read(p []byte) (n int, err error) {
	if sr.i >= len(sr.s) {
		return 0, io.EOF
	}
	n = copy(p, sr.s[sr.i:])
	sr.i += n
	return
}
