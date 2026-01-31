package service

import (
	"context"

	"github.com/medflow/medflow-backend/internal/staff/events"
	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/internal/staff/validation"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// StaffService handles staff business logic
type StaffService struct {
	employeeRepo *repository.EmployeeRepository
	publisher    *events.StaffEventPublisher
	validator    *validation.GermanValidator
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
