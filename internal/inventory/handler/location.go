package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// LocationHandler handles location endpoints
type LocationHandler struct {
	repo   *repository.LocationRepository
	logger *logger.Logger
}

// NewLocationHandler creates a new location handler
func NewLocationHandler(repo *repository.LocationRepository, log *logger.Logger) *LocationHandler {
	return &LocationHandler{
		repo:   repo,
		logger: log,
	}
}

// GetTree returns the full location hierarchy
func (h *LocationHandler) GetTree(w http.ResponseWriter, r *http.Request) {
	tree, err := h.repo.GetTree(r.Context())
	if err != nil {
		httputil.Error(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, tree)
}

// Room handlers

func (h *LocationHandler) ListRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.repo.ListRooms(r.Context())
	if err != nil {
		httputil.Error(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, rooms)
}

func (h *LocationHandler) GetRoom(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	room, err := h.repo.GetRoom(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, room)
}

func (h *LocationHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	var room repository.StorageRoom
	if err := httputil.DecodeJSON(r, &room); err != nil {
		httputil.Error(w, err)
		return
	}

	room.IsActive = true
	if err := h.repo.CreateRoom(r.Context(), &room); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, room)
}

func (h *LocationHandler) UpdateRoom(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var room repository.StorageRoom
	if err := httputil.DecodeJSON(r, &room); err != nil {
		httputil.Error(w, err)
		return
	}

	room.ID = id
	if err := h.repo.UpdateRoom(r.Context(), &room); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, room)
}

func (h *LocationHandler) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.DeleteRoom(r.Context(), id); err != nil {
		httputil.Error(w, err)
		return
	}
	httputil.NoContent(w)
}

// Cabinet handlers

func (h *LocationHandler) ListCabinets(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room_id")
	var cabinets []*repository.StorageCabinet
	var err error

	if roomID != "" {
		cabinets, err = h.repo.ListCabinets(r.Context(), roomID)
	} else {
		cabinets, err = h.repo.ListAllCabinets(r.Context())
	}

	if err != nil {
		httputil.Error(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, cabinets)
}

func (h *LocationHandler) GetCabinet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cabinet, err := h.repo.GetCabinet(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, cabinet)
}

func (h *LocationHandler) CreateCabinet(w http.ResponseWriter, r *http.Request) {
	var cabinet repository.StorageCabinet
	if err := httputil.DecodeJSON(r, &cabinet); err != nil {
		httputil.Error(w, err)
		return
	}

	cabinet.IsActive = true
	if err := h.repo.CreateCabinet(r.Context(), &cabinet); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, cabinet)
}

func (h *LocationHandler) UpdateCabinet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var cabinet repository.StorageCabinet
	if err := httputil.DecodeJSON(r, &cabinet); err != nil {
		httputil.Error(w, err)
		return
	}

	cabinet.ID = id
	if err := h.repo.UpdateCabinet(r.Context(), &cabinet); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, cabinet)
}

func (h *LocationHandler) DeleteCabinet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.DeleteCabinet(r.Context(), id); err != nil {
		httputil.Error(w, err)
		return
	}
	httputil.NoContent(w)
}

// Shelf handlers

func (h *LocationHandler) ListShelves(w http.ResponseWriter, r *http.Request) {
	cabinetID := r.URL.Query().Get("cabinet_id")
	var shelves []*repository.StorageShelf
	var err error

	if cabinetID != "" {
		shelves, err = h.repo.ListShelves(r.Context(), cabinetID)
	} else {
		shelves, err = h.repo.ListAllShelves(r.Context())
	}

	if err != nil {
		httputil.Error(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, shelves)
}

func (h *LocationHandler) GetShelf(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	shelf, err := h.repo.GetShelf(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, shelf)
}

func (h *LocationHandler) CreateShelf(w http.ResponseWriter, r *http.Request) {
	var shelf repository.StorageShelf
	if err := httputil.DecodeJSON(r, &shelf); err != nil {
		httputil.Error(w, err)
		return
	}

	shelf.IsActive = true
	if err := h.repo.CreateShelf(r.Context(), &shelf); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, shelf)
}

func (h *LocationHandler) UpdateShelf(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var shelf repository.StorageShelf
	if err := httputil.DecodeJSON(r, &shelf); err != nil {
		httputil.Error(w, err)
		return
	}

	shelf.ID = id
	if err := h.repo.UpdateShelf(r.Context(), &shelf); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, shelf)
}

func (h *LocationHandler) DeleteShelf(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.DeleteShelf(r.Context(), id); err != nil {
		httputil.Error(w, err)
		return
	}
	httputil.NoContent(w)
}
