package handler

import (
	"net/http"
	"strconv"

	"github.com/medflow/medflow-backend/internal/user/repository"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// AuditHandler handles audit log endpoints
type AuditHandler struct {
	repo   *repository.AuditRepository
	logger *logger.Logger
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(repo *repository.AuditRepository, log *logger.Logger) *AuditHandler {
	return &AuditHandler{
		repo:   repo,
		logger: log,
	}
}

// List lists audit logs
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	filter := &repository.ListFilter{
		ActorID:      r.URL.Query().Get("actor_id"),
		TargetUserID: r.URL.Query().Get("target_user_id"),
		Action:       r.URL.Query().Get("action"),
	}

	logs, total, err := h.repo.List(r.Context(), filter, page, perPage)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, logs, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}
