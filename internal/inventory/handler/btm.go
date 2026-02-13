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

// BtmHandler handles BtM (controlled substance) HTTP endpoints
type BtmHandler struct {
	btmService *service.BtmService
	logger     *logger.Logger
}

// NewBtmHandler creates a new BtM handler
func NewBtmHandler(svc *service.BtmService, log *logger.Logger) *BtmHandler {
	return &BtmHandler{
		btmService: svc,
		logger:     log,
	}
}

// ReceiveSubstance handles POST /btm/{itemId}/receipt
func (h *BtmHandler) ReceiveSubstance(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")

	var req service.BtmReceiveRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	req.ItemID = itemID

	entry, err := h.btmService.ReceiveSubstance(r.Context(), &req)
	if err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to receive BtM substance")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, entry)
}

// DispenseSubstance handles POST /btm/{itemId}/dispense
func (h *BtmHandler) DispenseSubstance(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")

	var req service.BtmDispenseRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	req.ItemID = itemID

	entry, err := h.btmService.DispenseSubstance(r.Context(), &req)
	if err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to dispense BtM substance")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, entry)
}

// DisposeSubstance handles POST /btm/{itemId}/disposal
func (h *BtmHandler) DisposeSubstance(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")

	var req service.BtmDisposeRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	req.ItemID = itemID

	entry, err := h.btmService.DisposeSubstance(r.Context(), &req)
	if err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to dispose BtM substance")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, entry)
}

// CorrectEntry handles POST /btm/{itemId}/correction
func (h *BtmHandler) CorrectEntry(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")

	var req service.BtmCorrectionRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	req.ItemID = itemID

	entry, err := h.btmService.CorrectEntry(r.Context(), &req)
	if err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to correct BtM entry")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, entry)
}

// InventoryCheck handles POST /btm/{itemId}/check
func (h *BtmHandler) InventoryCheck(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")

	var req service.BtmCheckRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	req.ItemID = itemID

	entry, err := h.btmService.InventoryCheck(r.Context(), &req)
	if err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to create BtM inventory check")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, entry)
}

// GetRegister handles GET /btm/{itemId}/register (paginated)
func (h *BtmHandler) GetRegister(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	entries, total, err := h.btmService.GetRegister(r.Context(), itemID, page, perPage)
	if err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to get BtM register")
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, entries, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetBalance handles GET /btm/{itemId}/balance
func (h *BtmHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")

	balance, err := h.btmService.GetBalance(r.Context(), itemID)
	if err != nil {
		h.logger.Error().Err(err).Str("item_id", itemID).Msg("failed to get BtM balance")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]interface{}{
		"item_id": itemID,
		"balance": balance,
	})
}

// ListAuthorizedPersonnel handles GET /btm/authorized-personnel
func (h *BtmHandler) ListAuthorizedPersonnel(w http.ResponseWriter, r *http.Request) {
	persons, err := h.btmService.ListAuthorizedPersonnel(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list BtM authorized personnel")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, persons)
}

// CreateAuthorizedPerson handles POST /btm/authorized-personnel
func (h *BtmHandler) CreateAuthorizedPerson(w http.ResponseWriter, r *http.Request) {
	var person repository.BtmAuthorizedPerson
	if err := httputil.DecodeJSON(r, &person); err != nil {
		httputil.Error(w, err)
		return
	}

	// Set authorized_by from the current user
	userID := httputil.GetUserID(r.Context())
	if userID != "" {
		person.AuthorizedBy = &userID
	}
	person.AuthorizedAt = time.Now()

	if err := h.btmService.CreateAuthorizedPerson(r.Context(), &person); err != nil {
		h.logger.Error().Err(err).Msg("failed to create BtM authorized person")
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, person)
}

// RevokeAuthorization handles PUT /btm/authorized-personnel/{id}/revoke
func (h *BtmHandler) RevokeAuthorization(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := httputil.GetUserID(r.Context())

	if err := h.btmService.RevokeAuthorization(r.Context(), id, userID, userID); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("failed to revoke BtM authorization")
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}
