package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// RecallHandler handles recall/FSN and safety officer HTTP endpoints
type RecallHandler struct {
	recallService *service.RecallService
	safetyRepo    *repository.SafetyOfficerRepository
	logger        *logger.Logger
}

// NewRecallHandler creates a new recall handler
func NewRecallHandler(svc *service.RecallService, safetyRepo *repository.SafetyOfficerRepository, log *logger.Logger) *RecallHandler {
	return &RecallHandler{
		recallService: svc,
		safetyRepo:    safetyRepo,
		logger:        log,
	}
}

// CreateNotice handles POST /recalls/notices
func (h *RecallHandler) CreateNotice(w http.ResponseWriter, r *http.Request) {
	var notice repository.FieldSafetyNotice
	if err := httputil.DecodeJSON(r, &notice); err != nil {
		httputil.Error(w, err)
		return
	}

	if notice.ReceivedDate.IsZero() {
		notice.ReceivedDate = time.Now()
	}

	if err := h.recallService.CreateFieldSafetyNotice(r.Context(), &notice); err != nil {
		h.logger.Error().Err(err).Msg("failed to create field safety notice")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, notice)
}

// GetNotice handles GET /recalls/notices/{id}
func (h *RecallHandler) GetNotice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	notice, err := h.recallService.GetNotice(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, notice)
}

// ListNotices handles GET /recalls/notices (supports status, page, per_page query params)
func (h *RecallHandler) ListNotices(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	status := r.URL.Query().Get("status")

	notices, total, err := h.recallService.ListNotices(r.Context(), status, page, perPage)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list field safety notices")
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, notices, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// UpdateNoticeStatus handles PUT /recalls/notices/{id}/status
func (h *RecallHandler) UpdateNoticeStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Status string `json:"status"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.recallService.UpdateNoticeStatus(r.Context(), id, req.Status); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to update notice status")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// ListMatchesByNotice handles GET /recalls/notices/{id}/matches
func (h *RecallHandler) ListMatchesByNotice(w http.ResponseWriter, r *http.Request) {
	noticeID := chi.URLParam(r, "id")

	matches, err := h.recallService.ListMatchesByNotice(r.Context(), noticeID)
	if err != nil {
		h.logger.Error().Err(err).Str("notice_id", noticeID).Msg("failed to list recall matches")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, matches)
}

// ResolveMatch handles PUT /recalls/matches/{id}/resolve
func (h *RecallHandler) ResolveMatch(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "id")

	var req struct {
		ActionTaken string `json:"action_taken"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	actionBy := httputil.GetUserID(r.Context())

	if err := h.recallService.ResolveMatch(r.Context(), matchID, req.ActionTaken, actionBy); err != nil {
		h.logger.Error().Err(err).Str("match_id", matchID).Msg("failed to resolve recall match")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// ListPendingMatches handles GET /recalls/matches/pending
func (h *RecallHandler) ListPendingMatches(w http.ResponseWriter, r *http.Request) {
	matches, total, err := h.recallService.ListPendingMatches(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list pending recall matches")
		httputil.Error(w, err)
		return
	}

	httputil.JSONWithMeta(w, http.StatusOK, matches, &httputil.Meta{
		Total: total,
	})
}

// Safety Officer endpoints

// ListSafetyOfficers handles GET /safety-officers
func (h *RecallHandler) ListSafetyOfficers(w http.ResponseWriter, r *http.Request) {
	officers, err := h.safetyRepo.List(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list safety officers")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, officers)
}

// CreateSafetyOfficer handles POST /safety-officers
func (h *RecallHandler) CreateSafetyOfficer(w http.ResponseWriter, r *http.Request) {
	var officer repository.SafetyOfficer
	if err := httputil.DecodeJSON(r, &officer); err != nil {
		httputil.Error(w, err)
		return
	}

	officer.IsActive = true

	if err := h.safetyRepo.Create(r.Context(), &officer); err != nil {
		h.logger.Error().Err(err).Msg("failed to create safety officer")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, officer)
}

// UpdateSafetyOfficer handles PUT /safety-officers/{id}
func (h *RecallHandler) UpdateSafetyOfficer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var officer repository.SafetyOfficer
	if err := httputil.DecodeJSON(r, &officer); err != nil {
		httputil.Error(w, err)
		return
	}

	officer.ID = id

	if err := h.safetyRepo.Update(r.Context(), &officer); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to update safety officer")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, officer)
}

// DeleteSafetyOfficer handles DELETE /safety-officers/{id}
func (h *RecallHandler) DeleteSafetyOfficer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.safetyRepo.Delete(r.Context(), id); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to delete safety officer")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}
