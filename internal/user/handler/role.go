package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/user/repository"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// RoleHandler handles role endpoints
type RoleHandler struct {
	repo   *repository.RoleRepository
	logger *logger.Logger
}

// NewRoleHandler creates a new role handler
func NewRoleHandler(repo *repository.RoleRepository, log *logger.Logger) *RoleHandler {
	return &RoleHandler{
		repo:   repo,
		logger: log,
	}
}

// List lists all roles
func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	roles, err := h.repo.List(r.Context())
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, roles)
}

// Get gets a role by ID
func (h *RoleHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	role, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, role)
}
