package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/user/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// UserHandler handles user endpoints
type UserHandler struct {
	service *service.UserService
	logger  *logger.Logger
}

// NewUserHandler creates a new user handler
func NewUserHandler(svc *service.UserService, log *logger.Logger) *UserHandler {
	return &UserHandler{
		service: svc,
		logger:  log,
	}
}

// List lists all users
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	users, total, err := h.service.List(r.Context(), page, perPage)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, users, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// Get gets a user by ID
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Support /users/me to get current user
	if id == "me" {
		id = r.Header.Get("X-User-ID")
	}

	user, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, user)
}

// GetUserInternal gets a user for internal service calls
func (h *UserHandler) GetUserInternal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	user, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	// Return user info for auth service
	response := map[string]interface{}{
		"id":          user.ID,
		"email":       user.Email,
		"name":        user.Name,
		"avatar":      user.Avatar,
		"role":        user.Role.Name,
		"permissions": user.GetEffectivePermissions(),
		"is_manager":  user.Role.IsManager,
	}

	httputil.JSON(w, http.StatusOK, response)
}

// Create creates a new user
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req service.CreateUserRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := httputil.Validate(&req); err != nil {
		httputil.Error(w, err)
		return
	}

	actorID := r.Header.Get("X-User-ID")
	actorName := r.Header.Get("X-User-Email")

	user, err := h.service.Create(r.Context(), &req, actorID, actorName)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, user)
}

// Update updates a user
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req service.UpdateUserRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	actorID := r.Header.Get("X-User-ID")
	actorName := r.Header.Get("X-User-Email")

	user, err := h.service.Update(r.Context(), id, &req, actorID, actorName)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, user)
}

// Delete deletes a user
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	actorID := r.Header.Get("X-User-ID")
	actorName := r.Header.Get("X-User-Email")

	if err := h.service.Delete(r.Context(), id, actorID, actorName); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// ChangeRole changes a user's role
func (h *UserHandler) ChangeRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Role string `json:"role" validate:"required"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := httputil.Validate(&req); err != nil {
		httputil.Error(w, err)
		return
	}

	actorID := r.Header.Get("X-User-ID")
	actorName := r.Header.Get("X-User-Email")

	user, err := h.service.ChangeRole(r.Context(), id, req.Role, actorID, actorName)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, user)
}

// GetPermissions gets a user's permissions
func (h *UserHandler) GetPermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	user, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	response := map[string]interface{}{
		"role_permissions":      user.Role.Permissions,
		"permission_overrides":  user.PermissionOverrides,
		"effective_permissions": user.GetEffectivePermissions(),
	}

	httputil.JSON(w, http.StatusOK, response)
}

// GrantPermission grants a permission to a user
func (h *UserHandler) GrantPermission(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Permission string `json:"permission" validate:"required"`
		Reason     string `json:"reason"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := httputil.Validate(&req); err != nil {
		httputil.Error(w, err)
		return
	}

	actorID := r.Header.Get("X-User-ID")
	actorName := r.Header.Get("X-User-Email")

	if err := h.service.GrantPermission(r.Context(), id, req.Permission, req.Reason, actorID, actorName); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// RevokePermission revokes a permission from a user
func (h *UserHandler) RevokePermission(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Permission string `json:"permission" validate:"required"`
		Reason     string `json:"reason"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := httputil.Validate(&req); err != nil {
		httputil.Error(w, err)
		return
	}

	actorID := r.Header.Get("X-User-ID")
	actorName := r.Header.Get("X-User-Email")

	if err := h.service.RevokePermission(r.Context(), id, req.Permission, req.Reason, actorID, actorName); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// GrantAccessGiver grants access giver status to a user
func (h *UserHandler) GrantAccessGiver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Scope []string `json:"scope" validate:"required"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := httputil.Validate(&req); err != nil {
		httputil.Error(w, err)
		return
	}

	actorID := r.Header.Get("X-User-ID")
	actorName := r.Header.Get("X-User-Email")

	if err := h.service.GrantAccessGiver(r.Context(), id, req.Scope, actorID, actorName); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// RevokeAccessGiver revokes access giver status from a user
func (h *UserHandler) RevokeAccessGiver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	actorID := r.Header.Get("X-User-ID")
	actorName := r.Header.Get("X-User-Email")

	if err := h.service.RevokeAccessGiver(r.Context(), id, actorID, actorName); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// ValidateCredentials validates user credentials (internal endpoint)
func (h *UserHandler) ValidateCredentials(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	user, tenantInfo, err := h.service.ValidateCredentials(r.Context(), req.Email, req.Password)
	if err != nil {
		h.logger.Error().Err(err).Str("email", req.Email).Msg("validate credentials failed")
		httputil.Error(w, err)
		return
	}

	// Extract permissions from role and overrides
	permissions := user.GetEffectivePermissions()

	// Return user info WITH tenant context for auth service
	response := map[string]interface{}{
		"id":     user.ID,
		"email":  user.Email,
		"name":   user.Name,
		"avatar": user.Avatar,
		"role":   user.Role.Name,

		"permissions": permissions,
		"is_manager":  user.Role.IsManager,

		// Tenant context - critical for multi-tenancy
		"tenant_id":     tenantInfo.ID,
		"tenant_slug":   tenantInfo.Slug,
		"tenant_schema": tenantInfo.SchemaName,
	}

	httputil.JSON(w, http.StatusOK, response)
}
