package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/user/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// InvitationHandler handles invitation endpoints
type InvitationHandler struct {
	service *service.InvitationService
	logger  *logger.Logger
}

// NewInvitationHandler creates a new invitation handler
func NewInvitationHandler(svc *service.InvitationService, log *logger.Logger) *InvitationHandler {
	return &InvitationHandler{
		service: svc,
		logger:  log,
	}
}

// Create creates a new invitation
func (h *InvitationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req service.CreateInvitationRequest
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

	response, err := h.service.Create(r.Context(), &req, actorID, actorName)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, response)
}

// Get gets an invitation by ID
func (h *InvitationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	inv, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, inv.ToResponse())
}

// GetByToken gets an invitation by token (public endpoint)
func (h *InvitationHandler) GetByToken(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	info, err := h.service.GetByToken(r.Context(), token)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, info)
}

// List lists all invitations
func (h *InvitationHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	status := r.URL.Query().Get("status")

	invitations, total, err := h.service.List(r.Context(), page, perPage, status)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	// Convert to response format
	responses := make([]*struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Role      string `json:"role"`
		RoleDE    string `json:"role_de"`
		Status    string `json:"status"`
		ExpiresAt string `json:"expires_at"`
		CreatedAt string `json:"created_at"`
	}, 0, len(invitations))

	for _, inv := range invitations {
		resp := &struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			Role      string `json:"role"`
			RoleDE    string `json:"role_de"`
			Status    string `json:"status"`
			ExpiresAt string `json:"expires_at"`
			CreatedAt string `json:"created_at"`
		}{
			ID:        inv.ID,
			Email:     inv.Email,
			Status:    string(inv.Status),
			ExpiresAt: inv.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
			CreatedAt: inv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if inv.Role != nil {
			resp.Role = inv.Role.DisplayName
			resp.RoleDE = inv.Role.DisplayNameDE
		}
		responses = append(responses, resp)
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, responses, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// Accept accepts an invitation
func (h *InvitationHandler) Accept(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	var req struct {
		Name     string `json:"name" validate:"required"`
		Password string `json:"password" validate:"required,min=8"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := httputil.Validate(&req); err != nil {
		httputil.Error(w, err)
		return
	}

	acceptReq := &service.AcceptInvitationRequest{
		Token:    token,
		Name:     req.Name,
		Password: req.Password,
	}

	response, err := h.service.Accept(r.Context(), acceptReq)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	// Return the created user (without password hash)
	httputil.Created(w, map[string]interface{}{
		"user": map[string]interface{}{
			"id":    response.User.ID,
			"email": response.User.Email,
			"name":  response.User.Name,
			"role":  response.User.Role.Name,
		},
		"message": "Account created successfully. Please log in.",
	})
}

// Revoke revokes an invitation
func (h *InvitationHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	actorID := r.Header.Get("X-User-ID")
	actorName := r.Header.Get("X-User-Email")

	if err := h.service.Revoke(r.Context(), id, actorID, actorName); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// Resend resends an invitation with a new token
func (h *InvitationHandler) Resend(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	actorID := r.Header.Get("X-User-ID")
	actorName := r.Header.Get("X-User-Email")

	response, err := h.service.Resend(r.Context(), id, actorID, actorName)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, response)
}
