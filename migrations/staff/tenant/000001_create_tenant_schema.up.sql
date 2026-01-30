-- MedFlow Schema-per-Tenant: Tenant Schema
-- Migration: Create staff/employee management tables

-- Employees table (staff members)
CREATE TABLE employees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,  -- Link to user account if they have system access (cross-service reference)

    -- Personal info
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    date_of_birth DATE,
    gender VARCHAR(20),
    nationality VARCHAR(100),

    -- Contact
    email VARCHAR(255),
    phone VARCHAR(50),
    mobile VARCHAR(50),

    -- Employment details
    employee_number VARCHAR(50) UNIQUE,
    job_title VARCHAR(255),
    department VARCHAR(100),
    employment_type VARCHAR(50) NOT NULL DEFAULT 'full_time',

    -- Dates
    hire_date DATE NOT NULL,
    termination_date DATE,
    probation_end_date DATE,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'active',

    -- Profile
    avatar_url TEXT,
    notes TEXT,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,  -- Cross-service reference to user
    updated_by UUID,  -- Cross-service reference to user

    -- Constraints
    CONSTRAINT employees_employment_type_valid CHECK (
        employment_type IN ('full_time', 'part_time', 'contractor', 'intern', 'temporary')
    ),
    CONSTRAINT employees_status_valid CHECK (
        status IN ('active', 'on_leave', 'suspended', 'terminated', 'pending')
    ),
    CONSTRAINT employees_email_format CHECK (
        email IS NULL OR email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'
    )
);

-- Employee Addresses
CREATE TABLE employee_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,

    -- Address type
    address_type VARCHAR(50) NOT NULL DEFAULT 'home',

    -- German address format
    street VARCHAR(255) NOT NULL,
    house_number VARCHAR(20),
    address_line2 VARCHAR(255),
    postal_code VARCHAR(20) NOT NULL,
    city VARCHAR(100) NOT NULL,
    state VARCHAR(100),
    country VARCHAR(100) NOT NULL DEFAULT 'Germany',

    -- Status
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT employee_addresses_type_valid CHECK (
        address_type IN ('home', 'mailing', 'emergency')
    )
);

-- Employee Emergency Contacts
CREATE TABLE employee_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,

    -- Contact details
    contact_type VARCHAR(50) NOT NULL DEFAULT 'emergency',
    name VARCHAR(255) NOT NULL,
    relationship VARCHAR(100),
    phone VARCHAR(50) NOT NULL,
    email VARCHAR(255),

    -- Priority
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT employee_contacts_type_valid CHECK (
        contact_type IN ('emergency', 'family', 'doctor', 'other')
    )
);

-- Employee Financial Details (German payroll)
CREATE TABLE employee_financials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,

    -- Bank details (IBAN format for Germany)
    iban VARCHAR(34),
    bic VARCHAR(11),
    bank_name VARCHAR(255),
    account_holder VARCHAR(255),

    -- Tax info (German)
    tax_id VARCHAR(20),  -- Steuer-ID (11 digits)
    tax_class VARCHAR(10),  -- Steuerklasse (1-6)
    church_tax BOOLEAN DEFAULT FALSE,
    child_allowance DECIMAL(5,2) DEFAULT 0,  -- Kinderfreibetrag

    -- Salary
    salary_type VARCHAR(50) DEFAULT 'monthly',
    base_salary_cents INTEGER,
    currency VARCHAR(3) DEFAULT 'EUR',

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT employee_financials_unique UNIQUE (employee_id),
    CONSTRAINT employee_financials_salary_type_valid CHECK (
        salary_type IN ('hourly', 'monthly', 'annual')
    )
);

-- Employee Social Insurance (German specific)
CREATE TABLE employee_social_insurance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,

    -- German social insurance numbers
    social_security_number VARCHAR(20),  -- Sozialversicherungsnummer
    health_insurance_provider VARCHAR(255),
    health_insurance_number VARCHAR(50),

    -- Insurance status
    pension_insurance BOOLEAN DEFAULT TRUE,
    unemployment_insurance BOOLEAN DEFAULT TRUE,
    health_insurance BOOLEAN DEFAULT TRUE,
    care_insurance BOOLEAN DEFAULT TRUE,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT employee_social_insurance_unique UNIQUE (employee_id)
);

-- Employee Documents
CREATE TABLE employee_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,

    -- Document info
    document_type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,

    -- File reference
    file_path TEXT NOT NULL,
    file_size_bytes INTEGER,
    mime_type VARCHAR(100),

    -- Validity
    issue_date DATE,
    expiry_date DATE,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'active',

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    uploaded_by UUID,  -- Cross-service reference to user

    -- Constraints
    CONSTRAINT employee_documents_type_valid CHECK (
        document_type IN (
            'contract', 'id_card', 'passport', 'work_permit',
            'certificate', 'qualification', 'training',
            'medical', 'other'
        )
    ),
    CONSTRAINT employee_documents_status_valid CHECK (
        status IN ('active', 'expired', 'superseded', 'archived')
    )
);

-- Indexes
CREATE INDEX idx_employees_user ON employees(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_employees_status ON employees(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_employees_department ON employees(department) WHERE deleted_at IS NULL;
CREATE INDEX idx_employees_number ON employees(employee_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_employees_name ON employees(last_name, first_name) WHERE deleted_at IS NULL;
CREATE INDEX idx_employees_hire_date ON employees(hire_date) WHERE deleted_at IS NULL;

CREATE INDEX idx_employee_addresses_employee ON employee_addresses(employee_id);
CREATE INDEX idx_employee_addresses_primary ON employee_addresses(employee_id, is_primary) WHERE is_primary = TRUE;

CREATE INDEX idx_employee_contacts_employee ON employee_contacts(employee_id);
CREATE INDEX idx_employee_contacts_primary ON employee_contacts(employee_id, is_primary) WHERE is_primary = TRUE;

CREATE INDEX idx_employee_documents_employee ON employee_documents(employee_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_employee_documents_type ON employee_documents(document_type) WHERE deleted_at IS NULL;
CREATE INDEX idx_employee_documents_expiry ON employee_documents(expiry_date) WHERE deleted_at IS NULL AND expiry_date IS NOT NULL;

-- Triggers
CREATE TRIGGER employees_updated_at
    BEFORE UPDATE ON employees
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER employee_addresses_updated_at
    BEFORE UPDATE ON employee_addresses
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER employee_contacts_updated_at
    BEFORE UPDATE ON employee_contacts
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER employee_financials_updated_at
    BEFORE UPDATE ON employee_financials
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER employee_social_insurance_updated_at
    BEFORE UPDATE ON employee_social_insurance
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER employee_documents_updated_at
    BEFORE UPDATE ON employee_documents
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
