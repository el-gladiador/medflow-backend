package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// Employee represents an employee
// DB fields use English names to match the database schema
// JSON fields use the same English names for API consistency
type Employee struct {
	// Core identity
	ID             string  `db:"id" json:"id"`
	UserID         *string `db:"user_id" json:"user_id,omitempty"`
	EmployeeNumber *string `db:"employee_number" json:"employee_number,omitempty"`

	// Personal info
	FirstName   string     `db:"first_name" json:"first_name"`
	LastName    string     `db:"last_name" json:"last_name"`
	AvatarURL   *string    `db:"avatar_url" json:"avatar_url,omitempty"`
	DateOfBirth *time.Time `db:"date_of_birth" json:"date_of_birth,omitempty"`
	Gender      *string    `db:"gender" json:"gender,omitempty"`
	Nationality *string    `db:"nationality" json:"nationality,omitempty"`

	// Employment info
	JobTitle       *string    `db:"job_title" json:"job_title,omitempty"`
	Department     *string    `db:"department" json:"department,omitempty"`
	EmploymentType string     `db:"employment_type" json:"employment_type"` // full_time, part_time, contractor, intern, temporary
	HireDate       time.Time  `db:"hire_date" json:"hire_date"`
	ProbationEnd   *time.Time `db:"probation_end_date" json:"probation_end_date,omitempty"`
	TerminationDate *time.Time `db:"termination_date" json:"termination_date,omitempty"`

	// Status and metadata
	Status    string     `db:"status" json:"status"` // active, on_leave, suspended, terminated, pending
	Notes     *string    `db:"notes" json:"notes,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"-"`
	CreatedBy *string    `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy *string    `db:"updated_by" json:"updated_by,omitempty"`

	// Transient fields for API convenience (not stored in employees table)
	Email  *string `db:"email" json:"email,omitempty"`
	Phone  *string `db:"phone" json:"phone,omitempty"`
	Mobile *string `db:"mobile" json:"mobile,omitempty"`

	// Legacy field aliases for backwards compatibility (use new names in new code)
	// These are exported but deprecated
	Vorname        string  `db:"-" json:"-"` // Use FirstName
	Nachname       string  `db:"-" json:"-"` // Use LastName
	Personalnummer string  `db:"-" json:"-"` // Use EmployeeNumber
	Rolle          string  `db:"-" json:"-"` // Use JobTitle
	Abteilung      *string `db:"-" json:"-"` // Use Department
	Anstellungsart string  `db:"-" json:"-"` // Use EmploymentType
	IsActive       bool    `db:"-" json:"-"` // Use Status == "active"
}

// EmployeeAddress represents an employee's address
type EmployeeAddress struct {
	ID          string    `db:"id" json:"id"`
	EmployeeID  string    `db:"employee_id" json:"employee_id"`
	AddressType string    `db:"address_type" json:"address_type"` // home, mailing, emergency
	Street      string    `db:"street" json:"street"`
	HouseNumber *string   `db:"house_number" json:"house_number,omitempty"`
	AddressLine2 *string  `db:"address_line2" json:"address_line2,omitempty"`
	PostalCode  string    `db:"postal_code" json:"postal_code"`
	City        string    `db:"city" json:"city"`
	State       *string   `db:"state" json:"state,omitempty"`
	Country     string    `db:"country" json:"country"`
	IsPrimary   bool      `db:"is_primary" json:"is_primary"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// EmployeeContact represents employee emergency contact information
// Note: This table stores emergency contacts, not the employee's own contact info
// Employee's own email/phone are in the employees table
type EmployeeContact struct {
	ID           string    `db:"id" json:"id"`
	EmployeeID   string    `db:"employee_id" json:"employee_id"`
	ContactType  string    `db:"contact_type" json:"contact_type"` // emergency, family, doctor, other
	Name         string    `db:"name" json:"name"`
	Relationship *string   `db:"relationship" json:"relationship,omitempty"`
	Phone        string    `db:"phone" json:"phone"`
	Email        *string   `db:"email" json:"email,omitempty"`
	IsPrimary    bool      `db:"is_primary" json:"is_primary"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// EmployeeFinancials represents employee financial data (German payroll)
type EmployeeFinancials struct {
	ID             string   `db:"id" json:"id"`
	EmployeeID     string   `db:"employee_id" json:"employee_id"`
	IBAN           *string  `db:"iban" json:"iban,omitempty"`
	BIC            *string  `db:"bic" json:"bic,omitempty"`
	BankName       *string  `db:"bank_name" json:"bank_name,omitempty"`
	AccountHolder  *string  `db:"account_holder" json:"account_holder,omitempty"`
	TaxID          *string  `db:"tax_id" json:"tax_id,omitempty"`          // Steuer-ID (11 digits)
	TaxClass       *string  `db:"tax_class" json:"tax_class,omitempty"`    // Steuerklasse (1-6)
	ChurchTax      bool     `db:"church_tax" json:"church_tax"`
	ChildAllowance *float64 `db:"child_allowance" json:"child_allowance,omitempty"` // Kinderfreibetrag
	SalaryType     *string  `db:"salary_type" json:"salary_type,omitempty"`          // hourly, monthly, annual
	BaseSalaryCents *int    `db:"base_salary_cents" json:"base_salary_cents,omitempty"`
	Currency       *string  `db:"currency" json:"currency,omitempty"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`

	// Legacy field aliases (deprecated - use new field names)
	SteuerID     string `db:"-" json:"-"` // Use TaxID
	Steuerklasse string `db:"-" json:"-"` // Use TaxClass
}

// EmployeeFile represents an uploaded file
type EmployeeFile struct {
	ID          string    `db:"id" json:"id"`
	EmployeeID  string    `db:"employee_id" json:"employee_id"`
	Name        string    `db:"name" json:"name"`
	FileType    string    `db:"file_type" json:"file_type"`
	FilePath    string    `db:"file_path" json:"file_path"`
	FileSize    *int      `db:"file_size" json:"file_size,omitempty"`
	MimeType    *string   `db:"mime_type" json:"mime_type,omitempty"`
	Category    *string   `db:"category" json:"category,omitempty"`
	UploadedAt  time.Time `db:"uploaded_at" json:"uploaded_at"`
	UploadedBy  *string   `db:"uploaded_by" json:"uploaded_by,omitempty"`
}

// EmployeeRepository handles employee persistence
type EmployeeRepository struct {
	db *database.DB
}

// NewEmployeeRepository creates a new employee repository
func NewEmployeeRepository(db *database.DB) *EmployeeRepository {
	return &EmployeeRepository{db: db}
}

// Create creates a new employee
// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *EmployeeRepository) Create(ctx context.Context, emp *Employee) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	if emp.ID == "" {
		emp.ID = uuid.New().String()
	}

	// Set defaults for required fields
	if emp.EmploymentType == "" {
		emp.EmploymentType = "full_time"
	}
	if emp.Status == "" {
		emp.Status = "active"
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO employees (
				id, user_id, employee_number, first_name, last_name, avatar_url,
				date_of_birth, gender, nationality,
				job_title, department, employment_type, hire_date,
				probation_end_date, termination_date, status, notes,
				email, phone, mobile, created_by
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
			) RETURNING created_at, updated_at
		`

		return r.db.QueryRowxContext(ctx, query,
			emp.ID, emp.UserID, emp.EmployeeNumber, emp.FirstName, emp.LastName, emp.AvatarURL,
			emp.DateOfBirth, emp.Gender, emp.Nationality,
			emp.JobTitle, emp.Department, emp.EmploymentType, emp.HireDate,
			emp.ProbationEnd, emp.TerminationDate, emp.Status, emp.Notes,
			emp.Email, emp.Phone, emp.Mobile, emp.CreatedBy,
		).Scan(&emp.CreatedAt, &emp.UpdatedAt)
	})
}

// GetByID gets an employee by ID
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *EmployeeRepository) GetByID(ctx context.Context, id string) (*Employee, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var emp Employee

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, user_id, employee_number, first_name, last_name, avatar_url,
			       date_of_birth, gender, nationality,
			       job_title, department, employment_type, hire_date,
			       probation_end_date, termination_date, status, notes,
			       email, phone, mobile, created_by, updated_by,
			       created_at, updated_at
			FROM employees
			WHERE id = $1 AND deleted_at IS NULL
		`

		return r.db.GetContext(ctx, &emp, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("employee")
	}
	if err != nil {
		return nil, err
	}

	return &emp, nil
}

// List lists employees with pagination
// TENANT-ISOLATED: Returns only employees from the tenant's schema
func (r *EmployeeRepository) List(ctx context.Context, page, perPage int) ([]*Employee, int64, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, 0, err // Fail-fast if tenant context missing
	}

	var total int64
	var employees []*Employee

	// Execute queries with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		// Count total employees
		countQuery := `SELECT COUNT(*) FROM employees WHERE deleted_at IS NULL`
		if err := r.db.GetContext(ctx, &total, countQuery); err != nil {
			return err
		}

		// Get paginated employees
		offset := (page - 1) * perPage
		query := `
			SELECT id, user_id, employee_number, first_name, last_name, avatar_url,
			       date_of_birth, gender, nationality,
			       job_title, department, employment_type, hire_date,
			       probation_end_date, termination_date, status, notes,
			       email, phone, mobile, created_by, updated_by,
			       created_at, updated_at
			FROM employees
			WHERE deleted_at IS NULL
			ORDER BY last_name, first_name
			LIMIT $1 OFFSET $2
		`

		return r.db.SelectContext(ctx, &employees, query, perPage, offset)
	})

	if err != nil {
		return nil, 0, err
	}

	return employees, total, nil
}

// Update updates an employee
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *EmployeeRepository) Update(ctx context.Context, emp *Employee) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE employees SET
				user_id = $2, employee_number = $3, first_name = $4, last_name = $5, avatar_url = $6,
				date_of_birth = $7, gender = $8, nationality = $9,
				job_title = $10, department = $11, employment_type = $12, hire_date = $13,
				probation_end_date = $14, termination_date = $15, status = $16, notes = $17,
				email = $18, phone = $19, mobile = $20, updated_by = $21
			WHERE id = $1 AND deleted_at IS NULL
		`

		result, err := r.db.ExecContext(ctx, query,
			emp.ID, emp.UserID, emp.EmployeeNumber, emp.FirstName, emp.LastName, emp.AvatarURL,
			emp.DateOfBirth, emp.Gender, emp.Nationality,
			emp.JobTitle, emp.Department, emp.EmploymentType, emp.HireDate,
			emp.ProbationEnd, emp.TerminationDate, emp.Status, emp.Notes,
			emp.Email, emp.Phone, emp.Mobile, emp.UpdatedBy,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("employee")
		}

		return nil
	})
}

// SoftDelete soft deletes an employee
// TENANT-ISOLATED: Soft deletes only in the tenant's schema
func (r *EmployeeRepository) SoftDelete(ctx context.Context, id string) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `UPDATE employees SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("employee")
		}

		return nil
	})
}

// GetAddress gets an employee's address
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *EmployeeRepository) GetAddress(ctx context.Context, employeeID string) (*EmployeeAddress, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var addr EmployeeAddress

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, employee_id, address_type, street, house_number, address_line2,
			       postal_code, city, state, country, is_primary, created_at, updated_at
			FROM employee_addresses WHERE employee_id = $1 AND is_primary = true LIMIT 1
		`
		return r.db.GetContext(ctx, &addr, query, employeeID)
	})

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &addr, nil
}

// SaveAddress saves an employee's address
// TENANT-ISOLATED: Inserts/updates only in the tenant's schema
func (r *EmployeeRepository) SaveAddress(ctx context.Context, addr *EmployeeAddress) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	if addr.ID == "" {
		addr.ID = uuid.New().String()
	}

	// Set default values
	if addr.Country == "" {
		addr.Country = "Germany"
	}
	if addr.AddressType == "" {
		addr.AddressType = "home"
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO employee_addresses (id, employee_id, address_type, street, house_number, address_line2, postal_code, city, state, country, is_primary)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (id)
			DO UPDATE SET address_type = $3, street = $4, house_number = $5, address_line2 = $6, postal_code = $7, city = $8, state = $9, country = $10, updated_at = NOW()
		`

		_, err := r.db.ExecContext(ctx, query,
			addr.ID, addr.EmployeeID, addr.AddressType, addr.Street, addr.HouseNumber, addr.AddressLine2, addr.PostalCode, addr.City, addr.State, addr.Country, addr.IsPrimary,
		)
		return err
	})
}

// GetContact gets an employee's contact (emergency contact)
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *EmployeeRepository) GetContact(ctx context.Context, employeeID string) (*EmployeeContact, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var contact EmployeeContact

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, employee_id, contact_type, name, relationship, phone, email,
			       is_primary, created_at, updated_at
			FROM employee_contacts WHERE employee_id = $1 AND is_primary = true LIMIT 1
		`
		return r.db.GetContext(ctx, &contact, query, employeeID)
	})

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &contact, nil
}

// SaveContact saves an employee's contact (emergency contact)
// TENANT-ISOLATED: Inserts/updates only in the tenant's schema
func (r *EmployeeRepository) SaveContact(ctx context.Context, contact *EmployeeContact) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	if contact.ID == "" {
		contact.ID = uuid.New().String()
	}

	// Set default values
	if contact.ContactType == "" {
		contact.ContactType = "emergency"
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO employee_contacts (id, employee_id, contact_type, name, relationship, phone, email, is_primary)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id)
			DO UPDATE SET contact_type = $3, name = $4, relationship = $5, phone = $6, email = $7, updated_at = NOW()
		`

		_, err := r.db.ExecContext(ctx, query,
			contact.ID, contact.EmployeeID, contact.ContactType, contact.Name,
			contact.Relationship, contact.Phone, contact.Email, contact.IsPrimary,
		)
		return err
	})
}

// GetFinancials gets an employee's financial data
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *EmployeeRepository) GetFinancials(ctx context.Context, employeeID string) (*EmployeeFinancials, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var fin EmployeeFinancials

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, employee_id, iban, bic, bank_name, account_holder,
			       tax_id, tax_class, church_tax, child_allowance,
			       salary_type, base_salary_cents, currency,
			       created_at, updated_at
			FROM employee_financials WHERE employee_id = $1 LIMIT 1
		`
		return r.db.GetContext(ctx, &fin, query, employeeID)
	})

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &fin, nil
}

// SaveFinancials saves an employee's financial data
// TENANT-ISOLATED: Inserts/updates only in the tenant's schema
func (r *EmployeeRepository) SaveFinancials(ctx context.Context, fin *EmployeeFinancials) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	if fin.ID == "" {
		fin.ID = uuid.New().String()
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO employee_financials (
				id, employee_id, iban, bic, bank_name, account_holder,
				tax_id, tax_class, church_tax, child_allowance,
				salary_type, base_salary_cents, currency
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (employee_id)
			DO UPDATE SET
				iban = $3, bic = $4, bank_name = $5, account_holder = $6,
				tax_id = $7, tax_class = $8, church_tax = $9, child_allowance = $10,
				salary_type = $11, base_salary_cents = $12, currency = $13, updated_at = NOW()
		`

		_, err := r.db.ExecContext(ctx, query,
			fin.ID, fin.EmployeeID, fin.IBAN, fin.BIC, fin.BankName, fin.AccountHolder,
			fin.TaxID, fin.TaxClass, fin.ChurchTax, fin.ChildAllowance,
			fin.SalaryType, fin.BaseSalaryCents, fin.Currency,
		)
		return err
	})
}

// ListFiles lists files for an employee
// TENANT-ISOLATED: Queries only the tenant's schema
func (r *EmployeeRepository) ListFiles(ctx context.Context, employeeID string) ([]*EmployeeFile, error) {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err // Fail-fast if tenant context missing
	}

	var files []*EmployeeFile

	// Execute query with tenant's search_path
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `SELECT * FROM employee_files WHERE employee_id = $1 ORDER BY uploaded_at DESC`
		return r.db.SelectContext(ctx, &files, query, employeeID)
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// CreateFile creates a file record
// TENANT-ISOLATED: Inserts only into the tenant's schema
func (r *EmployeeRepository) CreateFile(ctx context.Context, file *EmployeeFile) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	if file.ID == "" {
		file.ID = uuid.New().String()
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO employee_files (id, employee_id, name, file_type, file_path, file_size, mime_type, category, uploaded_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING uploaded_at
		`

		return r.db.QueryRowxContext(ctx, query,
			file.ID, file.EmployeeID, file.Name, file.FileType, file.FilePath,
			file.FileSize, file.MimeType, file.Category, file.UploadedBy,
		).Scan(&file.UploadedAt)
	})
}

// DeleteFile deletes a file record
// TENANT-ISOLATED: Deletes only from the tenant's schema
func (r *EmployeeRepository) DeleteFile(ctx context.Context, id string) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `DELETE FROM employee_files WHERE id = $1`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("file")
		}

		return nil
	})
}

// NullifyUserReferences nullifies user_id for all employees referencing a deleted user
// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *EmployeeRepository) NullifyUserReferences(ctx context.Context, userID string) error {
	// Extract tenant schema from context
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err // Fail-fast if tenant context missing
	}

	// Execute query with tenant's search_path
	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `UPDATE employees SET user_id = NULL WHERE user_id = $1`
		_, err := r.db.ExecContext(ctx, query, userID)
		return err
	})
}
