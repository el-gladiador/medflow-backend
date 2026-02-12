package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/medflow/medflow-backend/internal/staff/client"
	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/internal/staff/service"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// EmployeeHandler handles employee endpoints
type EmployeeHandler struct {
	service    *service.StaffService
	userClient *client.UserClient
	logger     *logger.Logger
}

// NewEmployeeHandler creates a new employee handler
func NewEmployeeHandler(svc *service.StaffService, userClient *client.UserClient, log *logger.Logger) *EmployeeHandler {
	return &EmployeeHandler{
		service:    svc,
		userClient: userClient,
		logger:     log,
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

// EmployeeCredentials represents user account credentials for an employee
type EmployeeCredentials struct {
	Username string `json:"username"`
	Password string `json:"password" validate:"required,min=8"`
	Role     string `json:"role" validate:"required"`
}

// CreateEmployeeRequest is the request structure for creating an employee
// Supports optional sub-entities (address, emergency_contact) for atomic creation
type CreateEmployeeRequest struct {
	Employee         repository.Employee         `json:"employee"`
	Credentials      *EmployeeCredentials        `json:"credentials,omitempty"`
	Address          *repository.EmployeeAddress  `json:"address,omitempty"`
	EmergencyContact *repository.EmployeeContact  `json:"emergency_contact,omitempty"`
}

// Create creates a new employee
// DEPRECATED: The optional credentials parameter is deprecated.
// Use POST /employees/{id}/credentials to add credentials after employee creation.
// This provides better separation of concerns and more reliable error handling.
func (h *EmployeeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateEmployeeRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	var userID *string

	// If credentials provided, create user account first
	if req.Credentials != nil {
		// Validate email is provided when creating user account
		if req.Employee.Email == nil || *req.Employee.Email == "" {
			httputil.Error(w, errors.BadRequest("email is required when creating user account"))
			return
		}

		h.logger.Info().
			Str("email", *req.Employee.Email).
			Str("role", req.Credentials.Role).
			Msg("creating employee with user account")

		// Create user account
		var username *string
		if req.Credentials.Username != "" {
			username = &req.Credentials.Username
		}
		userReq := &client.CreateUserRequest{
			Email:     *req.Employee.Email,
			Password:  req.Credentials.Password,
			FirstName: req.Employee.FirstName,
			LastName:  req.Employee.LastName,
			Username:  username,
			RoleName:  req.Credentials.Role,
			AvatarURL: req.Employee.AvatarURL,
		}

		user, err := h.userClient.CreateUser(r.Context(), userReq)
		if err != nil {
			h.logger.Error().Err(err).Msg("failed to create user account")
			// Convert user service errors to AppErrors for proper HTTP status codes
			errMsg := err.Error()
			if strings.Contains(errMsg, "status 400") || strings.Contains(errMsg, "VALIDATION_ERROR") {
				httputil.Error(w, errors.Validation(map[string]string{
					"credentials": "user account creation failed: invalid data (check email format and password)",
				}))
			} else if strings.Contains(errMsg, "status 409") {
				httputil.Error(w, errors.Conflict("a user with this email already exists"))
			} else {
				httputil.Error(w, errors.Internal("failed to create user account"))
			}
			return
		}

		userID = &user.ID
		h.logger.Info().Str("user_id", user.ID).Msg("user account created")
	}

	// Set user_id link
	req.Employee.UserID = userID

	// Create employee
	if err := h.service.Create(r.Context(), &req.Employee); err != nil {
		h.logger.Error().Err(err).Msg("failed to create employee")

		// Rollback: delete user account if it was created
		if userID != nil {
			h.logger.Warn().Str("user_id", *userID).Msg("rolling back user account creation")
			if rollbackErr := h.userClient.DeleteUser(r.Context(), *userID); rollbackErr != nil {
				h.logger.Error().Err(rollbackErr).Msg("failed to rollback user account")
			}
		}

		httputil.Error(w, err)
		return
	}

	// Save optional sub-entities after employee creation
	if req.Address != nil {
		req.Address.EmployeeID = req.Employee.ID
		req.Address.IsPrimary = true
		if err := h.service.SaveAddress(r.Context(), req.Address); err != nil {
			h.logger.Error().Err(err).Str("employee_id", req.Employee.ID).Msg("failed to save address for new employee")
			// Non-fatal: employee was created, address can be added later
		}
	}

	if req.EmergencyContact != nil {
		req.EmergencyContact.EmployeeID = req.Employee.ID
		req.EmergencyContact.IsPrimary = true
		if req.EmergencyContact.ContactType == "" {
			req.EmergencyContact.ContactType = "emergency"
		}
		if err := h.service.SaveContact(r.Context(), req.EmergencyContact); err != nil {
			h.logger.Error().Err(err).Str("employee_id", req.Employee.ID).Msg("failed to save emergency contact for new employee")
			// Non-fatal: employee was created, contact can be added later
		}
	}

	h.logger.Info().
		Str("employee_id", req.Employee.ID).
		Bool("has_user_account", userID != nil).
		Bool("has_address", req.Address != nil).
		Bool("has_emergency_contact", req.EmergencyContact != nil).
		Msg("employee created successfully")

	httputil.Created(w, req.Employee)
}

// Update updates an employee
func (h *EmployeeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Fetch the existing employee first to preserve fields not in the request.
	// Without this, partial JSON updates would zero out unmention fields
	// because the repository does a full-column UPDATE.
	existing, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	// Decode partial update onto the existing employee.
	// Go's JSON decoder only overwrites fields present in the JSON body,
	// leaving all other fields at their current database values.
	if err := httputil.DecodeJSON(r, existing); err != nil {
		httputil.Error(w, err)
		return
	}

	existing.ID = id // Ensure ID can't be changed via request body

	if err := h.service.Update(r.Context(), existing); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, existing)
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
		FirstName     string  `json:"first_name"`
		LastName      string  `json:"last_name"`
		AvatarURL     *string `json:"avatar_url"`
		Gender        *string `json:"gender"`
		Nationality   *string `json:"nationality"`
		BirthPlace    *string `json:"birth_place"`
		MaritalStatus *string `json:"marital_status"`
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
	if req.BirthPlace != nil {
		emp.BirthPlace = req.BirthPlace
	}
	if req.MaritalStatus != nil {
		emp.MaritalStatus = req.MaritalStatus
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

// GetMe returns the employee record for the currently authenticated user
// GET /employees/me
func (h *EmployeeHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		httputil.Error(w, errors.Unauthorized("missing user context"))
		return
	}

	emp, err := h.service.GetMyEmployee(r.Context(), userID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, emp)
}

// UpdateMyVisibility updates the show_in_staff_list flag for the current user
// PATCH /employees/me/visibility
func (h *EmployeeHandler) UpdateMyVisibility(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		httputil.Error(w, errors.Unauthorized("missing user context"))
		return
	}

	var req struct {
		ShowInStaffList bool `json:"show_in_staff_list"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.UpdateMyVisibility(r.Context(), userID, req.ShowInStaffList); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]bool{"show_in_staff_list": req.ShowInStaffList})
}

// ============================================================================
// CREDENTIAL MANAGEMENT
// ============================================================================

// AddCredentialsRequest is the request structure for adding credentials to an employee
type AddCredentialsRequest struct {
	Password string `json:"password" validate:"required,min=8"`
	Role     string `json:"role" validate:"required"`
}

// AddCredentials creates user credentials for an existing employee
// POST /employees/{id}/credentials
func (h *EmployeeHandler) AddCredentials(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	var req AddCredentialsRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	// Validate password length
	if len(req.Password) < 8 {
		httputil.Error(w, errors.BadRequest("password must be at least 8 characters"))
		return
	}

	// Set default role if not provided
	if req.Role == "" {
		req.Role = "staff"
	}

	// Get actor ID from request headers (set by API Gateway from JWT)
	actorID := r.Header.Get("X-User-ID")
	if actorID == "" {
		httputil.Error(w, errors.Unauthorized("missing user context"))
		return
	}

	h.logger.Info().
		Str("employee_id", employeeID).
		Str("role", req.Role).
		Str("actor_id", actorID).
		Msg("adding credentials to employee")

	result, err := h.service.AddCredentialsToEmployee(r.Context(), employeeID, req.Password, req.Role, actorID)
	if err != nil {
		h.logger.Error().Err(err).
			Str("employee_id", employeeID).
			Msg("failed to add credentials to employee")
		httputil.Error(w, err)
		return
	}

	h.logger.Info().
		Str("employee_id", employeeID).
		Str("user_id", result.UserID).
		Msg("credentials added successfully")

	httputil.Created(w, result)
}

// RemoveCredentialsRequest is the optional request structure for removing credentials
type RemoveCredentialsRequest struct {
	Reason string `json:"reason,omitempty"`
}

// RemoveCredentials removes user credentials from an employee
// DELETE /employees/{id}/credentials
func (h *EmployeeHandler) RemoveCredentials(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	// Parse optional reason from request body (may be empty)
	var req RemoveCredentialsRequest
	_ = httputil.DecodeJSON(r, &req) // Ignore decode errors, reason is optional

	// Get actor ID from request headers
	actorID := r.Header.Get("X-User-ID")
	if actorID == "" {
		httputil.Error(w, errors.Unauthorized("missing user context"))
		return
	}

	h.logger.Info().
		Str("employee_id", employeeID).
		Str("actor_id", actorID).
		Str("reason", req.Reason).
		Msg("removing credentials from employee")

	err := h.service.RemoveCredentialsFromEmployee(r.Context(), employeeID, actorID, req.Reason)
	if err != nil {
		h.logger.Error().Err(err).
			Str("employee_id", employeeID).
			Msg("failed to remove credentials from employee")
		httputil.Error(w, err)
		return
	}

	h.logger.Info().
		Str("employee_id", employeeID).
		Msg("credentials removed successfully")

	httputil.NoContent(w)
}

// GetCredentialStatus gets the credential status for an employee
// GET /employees/{id}/credentials
func (h *EmployeeHandler) GetCredentialStatus(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	status, err := h.service.GetCredentialStatus(r.Context(), employeeID)
	if err != nil {
		h.logger.Error().Err(err).
			Str("employee_id", employeeID).
			Msg("failed to get credential status")
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, status)
}
