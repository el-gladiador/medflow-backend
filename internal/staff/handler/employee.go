package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/internal/staff/service"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// EmployeeHandler handles employee endpoints
type EmployeeHandler struct {
	service *service.StaffService
	logger  *logger.Logger
}

// NewEmployeeHandler creates a new employee handler
func NewEmployeeHandler(svc *service.StaffService, log *logger.Logger) *EmployeeHandler {
	return &EmployeeHandler{
		service: svc,
		logger:  log,
	}
}

// List lists all employees
func (h *EmployeeHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	employees, total, err := h.service.List(r.Context(), page, perPage)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	httputil.JSONWithMeta(w, http.StatusOK, employees, &httputil.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// Get gets an employee by ID
func (h *EmployeeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	employee, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, employee)
}

// Create creates a new employee
func (h *EmployeeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var emp repository.Employee
	if err := httputil.DecodeJSON(r, &emp); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.Create(r.Context(), &emp); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, emp)
}

// Update updates an employee
func (h *EmployeeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var emp repository.Employee
	if err := httputil.DecodeJSON(r, &emp); err != nil {
		httputil.Error(w, err)
		return
	}

	emp.ID = id

	if err := h.service.Update(r.Context(), &emp); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, emp)
}

// Delete deletes an employee
func (h *EmployeeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.Delete(r.Context(), id); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

// UpdatePersonal updates an employee's personal information
func (h *EmployeeHandler) UpdatePersonal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Get existing employee
	emp, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	// Decode and merge personal info
	var req struct {
		FirstName   string  `json:"first_name"`
		LastName    string  `json:"last_name"`
		AvatarURL   *string `json:"avatar_url"`
		Gender      *string `json:"gender"`
		Nationality *string `json:"nationality"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if req.FirstName != "" {
		emp.FirstName = req.FirstName
	}
	if req.LastName != "" {
		emp.LastName = req.LastName
	}
	if req.AvatarURL != nil {
		emp.AvatarURL = req.AvatarURL
	}
	if req.Gender != nil {
		emp.Gender = req.Gender
	}
	if req.Nationality != nil {
		emp.Nationality = req.Nationality
	}

	if err := h.service.Update(r.Context(), emp); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, emp)
}

// UpdateContact updates an employee's contact information
func (h *EmployeeHandler) UpdateContact(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var contact repository.EmployeeContact
	if err := httputil.DecodeJSON(r, &contact); err != nil {
		httputil.Error(w, err)
		return
	}

	contact.EmployeeID = id

	if err := h.service.SaveContact(r.Context(), &contact); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, contact)
}

// UpdateFinancials updates an employee's financial information
func (h *EmployeeHandler) UpdateFinancials(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var fin repository.EmployeeFinancials
	if err := httputil.DecodeJSON(r, &fin); err != nil {
		httputil.Error(w, err)
		return
	}

	fin.EmployeeID = id

	if err := h.service.SaveFinancials(r.Context(), &fin); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, fin)
}

// ListFiles lists files for an employee
func (h *EmployeeHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	files, err := h.service.ListFiles(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, files)
}

// UploadFile handles file upload
func (h *EmployeeHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httputil.Error(w, err)
		return
	}

	// TODO: Implement actual file upload to storage
	// For now, just create a record

	var file repository.EmployeeFile
	file.EmployeeID = id
	file.Name = r.FormValue("name")
	file.FileType = r.FormValue("type")
	file.Category = strPtr(r.FormValue("category"))
	file.FilePath = "/uploads/" + file.Name // Placeholder

	userID := r.Header.Get("X-User-ID")
	if userID != "" {
		file.UploadedBy = &userID
	}

	if err := h.service.CreateFile(r.Context(), &file); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Created(w, file)
}

// DeleteFile deletes a file
func (h *EmployeeHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileId")

	if err := h.service.DeleteFile(r.Context(), fileID); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.NoContent(w)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
