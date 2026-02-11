package handler

import (
	"net/http"
	"strings"

	"github.com/medflow/medflow-backend/internal/auth/service"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	service *service.AuthService
	logger  *logger.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(svc *service.AuthService, log *logger.Logger) *AuthHandler {
	return &AuthHandler{
		service: svc,
		logger:  log,
	}
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req service.LoginRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := httputil.Validate(&req); err != nil {
		httputil.Error(w, err)
		return
	}

	userAgent := r.UserAgent()
	ipAddress := r.RemoteAddr

	response, err := h.service.Login(r.Context(), &req, userAgent, ipAddress)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, response)
}

// Logout handles user logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := httputil.DecodeJSON(r, &req); err != nil {
		// Try to get from Authorization header instead
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 {
				req.RefreshToken = parts[1]
			}
		}
	}

	if err := h.service.Logout(r.Context(), req.RefreshToken); err != nil {
		h.logger.Warn().Err(err).Msg("logout error")
	}

	httputil.NoContent(w)
}

// Refresh handles token refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := httputil.Validate(&req); err != nil {
		httputil.Error(w, err)
		return
	}

	tokens, err := h.service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, tokens)
}

// Me returns the current user's information
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		httputil.Error(w, errors.Unauthorized("not authenticated"))
		return
	}

	// Extract tenant headers (set by API Gateway's AuthMiddleware)
	tenantID := r.Header.Get("X-Tenant-ID")
	tenantSlug := r.Header.Get("X-Tenant-Slug")

	user, err := h.service.GetCurrentUser(r.Context(), userID, tenantID, tenantSlug)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, user)
}
