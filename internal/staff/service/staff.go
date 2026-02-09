package service

import (
	"context"
	"fmt"

	"github.com/medflow/medflow-backend/internal/staff/client"
	"github.com/medflow/medflow-backend/internal/staff/events"
	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/internal/staff/validation"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// StaffService handles staff business logic
type StaffService struct {
	employeeRepo *repository.EmployeeRepository
	publisher    *events.StaffEventPublisher
	validator    *validation.GermanValidator
	userClient   *client.UserClient
	logger       *logger.Logger
}

// NewStaffService creates a new staff service
func NewStaffService(
	employeeRepo *repository.EmployeeRepository,
	publisher *events.StaffEventPublisher,
	validator *validation.GermanValidator,
	log *logger.Logger,
) *StaffService {
	return &StaffService{
		employeeRepo: employeeRepo,
		publisher:    publisher,
		validator:    validator,
		logger:       log,
	}
}

// SetUserClient sets the user client for credential management
// This is set separately to avoid circular dependency during initialization
func (s *StaffService) SetUserClient(uc *client.UserClient) {
	s.userClient = uc
}

// CredentialResult is the response for adding credentials to an employee
type CredentialResult struct {
	EmployeeID string `json:"employee_id"`
	UserID     string `json:"user_id"`
	Email      string `json:"email"`
	RoleName   string `json:"role_name"`
}

// CredentialStatus is the response for getting credential status of an employee
type CredentialStatus struct {
	HasCredentials bool    `json:"has_credentials"`
	UserID         *string `json:"user_id,omitempty"`
	Email          *string `json:"email,omitempty"`
	RoleName       *string `json:"role_name,omitempty"`
	Status         *string `json:"status,omitempty"`
}

// Create creates a new employee
func (s *StaffService) Create(ctx context.Context, emp *repository.Employee) error {
	if err := s.employeeRepo.Create(ctx, emp); err != nil {
		return err
	}

	// Publish event
	s.publisher.PublishEmployeeCreated(ctx, emp)

	return nil
}

// GetByID gets an employee by ID
func (s *StaffService) GetByID(ctx context.Context, id string) (*repository.Employee, error) {
	return s.employeeRepo.GetByID(ctx, id)
}

// GetByUserID gets an employee by their linked user ID
func (s *StaffService) GetByUserID(ctx context.Context, userID string) (*repository.Employee, error) {
	return s.employeeRepo.GetByUserID(ctx, userID)
}

// List lists employees with pagination
func (s *StaffService) List(ctx context.Context, page, perPage int) ([]*repository.Employee, int64, error) {
	return s.employeeRepo.List(ctx, page, perPage)
}

// Update updates an employee
func (s *StaffService) Update(ctx context.Context, emp *repository.Employee) error {
	if err := s.employeeRepo.Update(ctx, emp); err != nil {
		return err
	}

	// Publish event
	s.publisher.PublishEmployeeUpdated(ctx, emp)

	return nil
}

// Delete soft deletes an employee
func (s *StaffService) Delete(ctx context.Context, id string) error {
	if err := s.employeeRepo.SoftDelete(ctx, id); err != nil {
		return err
	}

	// Publish event
	s.publisher.PublishEmployeeDeleted(ctx, id)

	return nil
}

// GetAddress gets an employee's address
func (s *StaffService) GetAddress(ctx context.Context, employeeID string) (*repository.EmployeeAddress, error) {
	return s.employeeRepo.GetAddress(ctx, employeeID)
}

// SaveAddress saves an employee's address
func (s *StaffService) SaveAddress(ctx context.Context, addr *repository.EmployeeAddress) error {
	return s.employeeRepo.SaveAddress(ctx, addr)
}

// GetContact gets an employee's contact info
func (s *StaffService) GetContact(ctx context.Context, employeeID string) (*repository.EmployeeContact, error) {
	return s.employeeRepo.GetContact(ctx, employeeID)
}

// SaveContact saves an employee's contact info
func (s *StaffService) SaveContact(ctx context.Context, contact *repository.EmployeeContact) error {
	return s.employeeRepo.SaveContact(ctx, contact)
}

// GetFinancials gets an employee's financial data
func (s *StaffService) GetFinancials(ctx context.Context, employeeID string) (*repository.EmployeeFinancials, error) {
	return s.employeeRepo.GetFinancials(ctx, employeeID)
}

// SaveFinancials saves an employee's financial data
func (s *StaffService) SaveFinancials(ctx context.Context, fin *repository.EmployeeFinancials) error {
	// Validate IBAN if provided
	if fin.IBAN != nil && *fin.IBAN != "" {
		result := s.validator.ValidateIBAN(*fin.IBAN)
		if !result.Valid {
			return &ValidationError{Field: "iban", Message: result.Message}
		}
	}

	// Validate Tax ID (SteuerID) if provided
	if fin.TaxID != nil && *fin.TaxID != "" {
		result := s.validator.ValidateTaxID(*fin.TaxID)
		if !result.Valid {
			return &ValidationError{Field: "tax_id", Message: result.Message}
		}
	}

	return s.employeeRepo.SaveFinancials(ctx, fin)
}

// ListFiles lists files for an employee
func (s *StaffService) ListFiles(ctx context.Context, employeeID string) ([]*repository.EmployeeFile, error) {
	return s.employeeRepo.ListFiles(ctx, employeeID)
}

// CreateFile creates a file record
func (s *StaffService) CreateFile(ctx context.Context, file *repository.EmployeeFile) error {
	return s.employeeRepo.CreateFile(ctx, file)
}

// DeleteFile deletes a file record
func (s *StaffService) DeleteFile(ctx context.Context, id string) error {
	return s.employeeRepo.DeleteFile(ctx, id)
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ============================================================================
// CREDENTIAL MANAGEMENT
// ============================================================================

// AddCredentialsToEmployee creates a user account and links it to an existing employee
// Role hierarchy validation: actor can only assign roles at their level or lower
func (s *StaffService) AddCredentialsToEmployee(
	ctx context.Context,
	employeeID string,
	password string,
	roleName string,
	actorID string,
) (*CredentialResult, error) {
	if s.userClient == nil {
		return nil, fmt.Errorf("user client not configured")
	}

	// 1. Get employee (validates tenant context)
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return nil, err
	}

	// 2. Verify employee has email and no existing user_id
	if emp.Email == nil || *emp.Email == "" {
		return nil, errors.BadRequest("employee must have an email to create credentials")
	}

	if emp.UserID != nil {
		return nil, errors.Conflict("employee already has user credentials")
	}

	// 3. Get actor's role level for hierarchy validation
	actor, err := s.userClient.GetUserByID(ctx, actorID)
	if err != nil {
		s.logger.Error().Err(err).Str("actor_id", actorID).Msg("failed to get actor user")
		return nil, errors.Forbidden("unable to verify actor permissions")
	}

	// 4. Get requested role's level
	requestedRole, err := s.userClient.GetRole(ctx, roleName)
	if err != nil {
		s.logger.Error().Err(err).Str("role", roleName).Msg("failed to get role")
		return nil, errors.BadRequest("invalid role: " + roleName)
	}

	// 5. Validate role hierarchy: actor can only assign roles at their level or lower
	if requestedRole.Level > actor.RoleLevel {
		s.logger.Warn().
			Str("actor_id", actorID).
			Int("actor_level", actor.RoleLevel).
			Str("requested_role", roleName).
			Int("requested_level", requestedRole.Level).
			Msg("role hierarchy violation: actor cannot assign higher-level role")
		return nil, errors.Forbidden("cannot assign role with higher permission level")
	}

	// 6. Create user via UserClient (tenant headers forwarded automatically)
	userReq := &client.CreateUserRequest{
		Email:     *emp.Email,
		Password:  password,
		FirstName: emp.FirstName,
		LastName:  emp.LastName,
		RoleName:  roleName,
		AvatarURL: emp.AvatarURL,
	}

	user, err := s.userClient.CreateUser(ctx, userReq)
	if err != nil {
		s.logger.Error().Err(err).Str("email", *emp.Email).Msg("failed to create user account")
		return nil, err
	}

	s.logger.Info().
		Str("employee_id", employeeID).
		Str("user_id", user.ID).
		Str("email", *emp.Email).
		Str("role", roleName).
		Msg("user account created for employee")

	// 7. Update employee.user_id
	if err := s.employeeRepo.UpdateUserID(ctx, employeeID, user.ID); err != nil {
		// User exists but employee not linked - log for potential manual reconciliation
		s.logger.Error().Err(err).
			Str("employee_id", employeeID).
			Str("user_id", user.ID).
			Msg("failed to link user to employee - user created but not linked")
		return &CredentialResult{
			EmployeeID: employeeID,
			UserID:     user.ID,
			Email:      *emp.Email,
			RoleName:   roleName,
		}, fmt.Errorf("user created (ID: %s) but failed to link to employee: %w", user.ID, err)
	}

	// 8. Publish event
	tenantID, _ := tenant.TenantID(ctx)
	tenantSlug, _ := tenant.TenantSlug(ctx)
	tenantSchema, _ := tenant.TenantSchema(ctx)
	s.publisher.PublishEmployeeCredentialsAdded(
		ctx,
		employeeID,
		user.ID,
		*emp.Email,
		roleName,
		actorID,
		tenantID,
		tenantSlug,
		tenantSchema,
	)

	return &CredentialResult{
		EmployeeID: employeeID,
		UserID:     user.ID,
		Email:      *emp.Email,
		RoleName:   roleName,
	}, nil
}

// RemoveCredentialsFromEmployee soft-deletes the user account and unlinks from employee
// The user is soft-deleted (can't login) but audit records are preserved
func (s *StaffService) RemoveCredentialsFromEmployee(
	ctx context.Context,
	employeeID string,
	actorID string,
	reason string,
) error {
	if s.userClient == nil {
		return fmt.Errorf("user client not configured")
	}

	// 1. Get employee (validates tenant context)
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return err
	}

	// 2. Verify employee has credentials
	if emp.UserID == nil {
		return errors.BadRequest("employee does not have user credentials")
	}

	userID := *emp.UserID
	email := ""
	if emp.Email != nil {
		email = *emp.Email
	}

	// 3. Soft-delete user via UserClient (disables login but preserves audit)
	if err := s.userClient.SoftDeleteUser(ctx, userID); err != nil {
		s.logger.Error().Err(err).
			Str("employee_id", employeeID).
			Str("user_id", userID).
			Msg("failed to soft-delete user")
		return err
	}

	s.logger.Info().
		Str("employee_id", employeeID).
		Str("user_id", userID).
		Str("reason", reason).
		Msg("user account soft-deleted")

	// 4. Clear employee.user_id
	if err := s.employeeRepo.ClearUserID(ctx, employeeID); err != nil {
		// User soft-deleted but employee still linked - log for reconciliation
		s.logger.Error().Err(err).
			Str("employee_id", employeeID).
			Str("user_id", userID).
			Msg("failed to unlink user from employee - user soft-deleted but still linked")
		return fmt.Errorf("user soft-deleted but failed to unlink from employee: %w", err)
	}

	// 5. Publish event
	tenantID, _ := tenant.TenantID(ctx)
	tenantSlug, _ := tenant.TenantSlug(ctx)
	tenantSchema, _ := tenant.TenantSchema(ctx)
	s.publisher.PublishEmployeeCredentialsRemoved(
		ctx,
		employeeID,
		userID,
		email,
		actorID,
		reason,
		tenantID,
		tenantSlug,
		tenantSchema,
	)

	return nil
}

// GetCredentialStatus returns the credential status for an employee
func (s *StaffService) GetCredentialStatus(ctx context.Context, employeeID string) (*CredentialStatus, error) {
	// 1. Get employee (validates tenant context)
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return nil, err
	}

	// 2. If no user_id, return empty status
	if emp.UserID == nil {
		return &CredentialStatus{
			HasCredentials: false,
		}, nil
	}

	// 3. If userClient is available, fetch user details
	if s.userClient != nil {
		user, err := s.userClient.GetUserByID(ctx, *emp.UserID)
		if err != nil {
			s.logger.Warn().Err(err).
				Str("employee_id", employeeID).
				Str("user_id", *emp.UserID).
				Msg("failed to fetch user details for credential status")
			// Return basic status without user details
			return &CredentialStatus{
				HasCredentials: true,
				UserID:         emp.UserID,
				Email:          emp.Email,
			}, nil
		}

		return &CredentialStatus{
			HasCredentials: true,
			UserID:         emp.UserID,
			Email:          &user.Email,
			RoleName:       &user.RoleName,
			Status:         &user.Status,
		}, nil
	}

	// 4. Return basic status if no user client
	return &CredentialStatus{
		HasCredentials: true,
		UserID:         emp.UserID,
		Email:          emp.Email,
	}, nil
}
