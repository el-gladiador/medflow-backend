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

// RadiationDevice represents a radiation-emitting device under StrlSchV/RoV
type RadiationDevice struct {
	ID                  string     `db:"id" json:"id"`
	ItemID              string     `db:"item_id" json:"item_id"`
	DeviceCategory      string     `db:"device_category" json:"device_category"`
	ApprovalNumber      *string    `db:"approval_number" json:"approval_number,omitempty"`
	Location            *string    `db:"location" json:"location,omitempty"`
	ResponsiblePerson   string     `db:"responsible_person" json:"responsible_person"`
	ResponsiblePersonID *string    `db:"responsible_person_id" json:"responsible_person_id,omitempty"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt           *time.Time `db:"deleted_at" json:"-"`
}

// ConstancyTest represents a radiation device constancy test (Konstanzpruefung)
type ConstancyTest struct {
	ID            string     `db:"id" json:"id"`
	DeviceID      string     `db:"device_id" json:"device_id"`
	TestDate      time.Time  `db:"test_date" json:"test_date"`
	TestType      string     `db:"test_type" json:"test_type"`
	Result        string     `db:"result" json:"result"`
	PerformedBy   string     `db:"performed_by" json:"performed_by"`
	PerformedByID *string    `db:"performed_by_id" json:"performed_by_id,omitempty"`
	NextDueDate   *time.Time `db:"next_due_date" json:"next_due_date,omitempty"`
	Notes         *string    `db:"notes" json:"notes,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt     *time.Time `db:"deleted_at" json:"-"`
}

// ExpertInspection represents an expert radiation inspection (Sachverstaendigenpruefung)
type ExpertInspection struct {
	ID                    string     `db:"id" json:"id"`
	DeviceID              string     `db:"device_id" json:"device_id"`
	InspectionDate        time.Time  `db:"inspection_date" json:"inspection_date"`
	InspectorName         string     `db:"inspector_name" json:"inspector_name"`
	InspectorOrganization *string    `db:"inspector_organization" json:"inspector_organization,omitempty"`
	Result                string     `db:"result" json:"result"`
	ReportReference       *string    `db:"report_reference" json:"report_reference,omitempty"`
	NextDueDate           *time.Time `db:"next_due_date" json:"next_due_date,omitempty"`
	Notes                 *string    `db:"notes" json:"notes,omitempty"`
	CreatedAt             time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt             *time.Time `db:"deleted_at" json:"-"`
}

// StaffRadiationCertification represents a staff radiation certification (Fachkunde/Kenntnisse)
type StaffRadiationCertification struct {
	ID                string     `db:"id" json:"id"`
	EmployeeID        string     `db:"employee_id" json:"employee_id"`
	EmployeeName      string     `db:"employee_name" json:"employee_name"`
	CertificationType string     `db:"certification_type" json:"certification_type"`
	IssuedDate        time.Time  `db:"issued_date" json:"issued_date"`
	ExpiryDate        time.Time  `db:"expiry_date" json:"expiry_date"`
	IssuingAuthority  *string    `db:"issuing_authority" json:"issuing_authority,omitempty"`
	CertificateNumber *string    `db:"certificate_number" json:"certificate_number,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt         *time.Time `db:"deleted_at" json:"-"`
}

// DosimetryRecord represents a dosimetry measurement record (Personendosimetrie)
type DosimetryRecord struct {
	ID                     string     `db:"id" json:"id"`
	EmployeeID             string     `db:"employee_id" json:"employee_id"`
	EmployeeName           string     `db:"employee_name" json:"employee_name"`
	MeasurementPeriodStart time.Time  `db:"measurement_period_start" json:"measurement_period_start"`
	MeasurementPeriodEnd   time.Time  `db:"measurement_period_end" json:"measurement_period_end"`
	DosimeterType          string     `db:"dosimeter_type" json:"dosimeter_type"`
	DoseMsv                float64    `db:"dose_msv" json:"dose_msv"`
	BodyRegion             string     `db:"body_region" json:"body_region"`
	Notes                  *string    `db:"notes" json:"notes,omitempty"`
	CreatedAt              time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt              *time.Time `db:"deleted_at" json:"-"`
}

// RadiationRepository handles radiation protection data persistence
type RadiationRepository struct {
	db *database.DB
}

// NewRadiationRepository creates a new radiation repository
func NewRadiationRepository(db *database.DB) *RadiationRepository {
	return &RadiationRepository{db: db}
}

// --- Radiation Devices ---

// CreateDevice creates a new radiation device
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *RadiationRepository) CreateDevice(ctx context.Context, device *RadiationDevice) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if device.ID == "" {
		device.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO radiation_devices (
				id, tenant_id, item_id, device_category, approval_number,
				location, responsible_person, responsible_person_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			device.ID, tenantID, device.ItemID, device.DeviceCategory,
			device.ApprovalNumber, device.Location, device.ResponsiblePerson,
			device.ResponsiblePersonID,
		).Scan(&device.CreatedAt, &device.UpdatedAt)
	})
}

// GetDevice gets a radiation device by ID
// TENANT-ISOLATED: Queries via RLS
func (r *RadiationRepository) GetDevice(ctx context.Context, id string) (*RadiationDevice, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var device RadiationDevice
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, device_category, approval_number, location,
			       responsible_person, responsible_person_id, created_at, updated_at
			FROM radiation_devices WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &device, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("radiation_device")
	}
	if err != nil {
		return nil, err
	}

	return &device, nil
}

// ListDevices lists all radiation devices
// TENANT-ISOLATED: Returns only devices via RLS
func (r *RadiationRepository) ListDevices(ctx context.Context) ([]*RadiationDevice, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var devices []*RadiationDevice
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, device_category, approval_number, location,
			       responsible_person, responsible_person_id, created_at, updated_at
			FROM radiation_devices WHERE deleted_at IS NULL
			ORDER BY created_at DESC
		`
		return r.db.SelectContext(ctx, &devices, query)
	})

	if err != nil {
		return nil, err
	}

	return devices, nil
}

// UpdateDevice updates a radiation device
// TENANT-ISOLATED: Updates via RLS
func (r *RadiationRepository) UpdateDevice(ctx context.Context, device *RadiationDevice) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE radiation_devices SET
				item_id = $2, device_category = $3, approval_number = $4,
				location = $5, responsible_person = $6, responsible_person_id = $7,
				updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			device.ID, device.ItemID, device.DeviceCategory, device.ApprovalNumber,
			device.Location, device.ResponsiblePerson, device.ResponsiblePersonID,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("radiation_device")
		}

		return nil
	})
}

// DeleteDevice soft-deletes a radiation device
// TENANT-ISOLATED: Deletes via RLS
func (r *RadiationRepository) DeleteDevice(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE radiation_devices SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("radiation_device")
		}

		return nil
	})
}

// --- Constancy Tests ---

// CreateTest creates a new constancy test
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *RadiationRepository) CreateTest(ctx context.Context, test *ConstancyTest) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if test.ID == "" {
		test.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO constancy_tests (
				id, tenant_id, device_id, test_date, test_type, result,
				performed_by, performed_by_id, next_due_date, notes
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			test.ID, tenantID, test.DeviceID, test.TestDate, test.TestType,
			test.Result, test.PerformedBy, test.PerformedByID, test.NextDueDate, test.Notes,
		).Scan(&test.CreatedAt, &test.UpdatedAt)
	})
}

// ListTestsByDevice lists constancy tests for a device
// TENANT-ISOLATED: Returns only tests via RLS
func (r *RadiationRepository) ListTestsByDevice(ctx context.Context, deviceID string) ([]*ConstancyTest, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var tests []*ConstancyTest
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, device_id, test_date, test_type, result,
			       performed_by, performed_by_id, next_due_date, notes, created_at, updated_at
			FROM constancy_tests WHERE device_id = $1 AND deleted_at IS NULL
			ORDER BY test_date DESC
		`
		return r.db.SelectContext(ctx, &tests, query, deviceID)
	})

	if err != nil {
		return nil, err
	}

	return tests, nil
}

// UpdateTest updates a constancy test
// TENANT-ISOLATED: Updates via RLS
func (r *RadiationRepository) UpdateTest(ctx context.Context, test *ConstancyTest) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE constancy_tests SET
				test_date = $2, test_type = $3, result = $4,
				performed_by = $5, performed_by_id = $6, next_due_date = $7,
				notes = $8, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			test.ID, test.TestDate, test.TestType, test.Result,
			test.PerformedBy, test.PerformedByID, test.NextDueDate, test.Notes,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("constancy_test")
		}

		return nil
	})
}

// DeleteTest soft-deletes a constancy test
// TENANT-ISOLATED: Deletes via RLS
func (r *RadiationRepository) DeleteTest(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE constancy_tests SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("constancy_test")
		}

		return nil
	})
}

// --- Expert Inspections ---

// CreateExpertInspection creates a new expert inspection record
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *RadiationRepository) CreateExpertInspection(ctx context.Context, insp *ExpertInspection) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if insp.ID == "" {
		insp.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO expert_inspections (
				id, tenant_id, device_id, inspection_date, inspector_name,
				inspector_organization, result, report_reference, next_due_date, notes
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			insp.ID, tenantID, insp.DeviceID, insp.InspectionDate, insp.InspectorName,
			insp.InspectorOrganization, insp.Result, insp.ReportReference,
			insp.NextDueDate, insp.Notes,
		).Scan(&insp.CreatedAt, &insp.UpdatedAt)
	})
}

// ListExpertInspectionsByDevice lists expert inspections for a device
// TENANT-ISOLATED: Returns only inspections via RLS
func (r *RadiationRepository) ListExpertInspectionsByDevice(ctx context.Context, deviceID string) ([]*ExpertInspection, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var inspections []*ExpertInspection
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, device_id, inspection_date, inspector_name, inspector_organization,
			       result, report_reference, next_due_date, notes, created_at, updated_at
			FROM expert_inspections WHERE device_id = $1 AND deleted_at IS NULL
			ORDER BY inspection_date DESC
		`
		return r.db.SelectContext(ctx, &inspections, query, deviceID)
	})

	if err != nil {
		return nil, err
	}

	return inspections, nil
}

// UpdateExpertInspection updates an expert inspection
// TENANT-ISOLATED: Updates via RLS
func (r *RadiationRepository) UpdateExpertInspection(ctx context.Context, insp *ExpertInspection) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE expert_inspections SET
				inspection_date = $2, inspector_name = $3, inspector_organization = $4,
				result = $5, report_reference = $6, next_due_date = $7, notes = $8,
				updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			insp.ID, insp.InspectionDate, insp.InspectorName, insp.InspectorOrganization,
			insp.Result, insp.ReportReference, insp.NextDueDate, insp.Notes,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("expert_inspection")
		}

		return nil
	})
}

// DeleteExpertInspection soft-deletes an expert inspection
// TENANT-ISOLATED: Deletes via RLS
func (r *RadiationRepository) DeleteExpertInspection(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE expert_inspections SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("expert_inspection")
		}

		return nil
	})
}

// --- Staff Radiation Certifications ---

// CreateCertification creates a new staff radiation certification
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *RadiationRepository) CreateCertification(ctx context.Context, cert *StaffRadiationCertification) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if cert.ID == "" {
		cert.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO staff_radiation_certifications (
				id, tenant_id, employee_id, employee_name, certification_type,
				issued_date, expiry_date, issuing_authority, certificate_number
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			cert.ID, tenantID, cert.EmployeeID, cert.EmployeeName, cert.CertificationType,
			cert.IssuedDate, cert.ExpiryDate, cert.IssuingAuthority, cert.CertificateNumber,
		).Scan(&cert.CreatedAt, &cert.UpdatedAt)
	})
}

// ListCertifications lists all staff radiation certifications
// TENANT-ISOLATED: Returns only certifications via RLS
func (r *RadiationRepository) ListCertifications(ctx context.Context) ([]*StaffRadiationCertification, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var certs []*StaffRadiationCertification
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, employee_id, employee_name, certification_type,
			       issued_date, expiry_date, issuing_authority, certificate_number,
			       created_at, updated_at
			FROM staff_radiation_certifications WHERE deleted_at IS NULL
			ORDER BY expiry_date ASC
		`
		return r.db.SelectContext(ctx, &certs, query)
	})

	if err != nil {
		return nil, err
	}

	return certs, nil
}

// UpdateCertification updates a staff radiation certification
// TENANT-ISOLATED: Updates via RLS
func (r *RadiationRepository) UpdateCertification(ctx context.Context, cert *StaffRadiationCertification) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE staff_radiation_certifications SET
				employee_id = $2, employee_name = $3, certification_type = $4,
				issued_date = $5, expiry_date = $6, issuing_authority = $7,
				certificate_number = $8, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			cert.ID, cert.EmployeeID, cert.EmployeeName, cert.CertificationType,
			cert.IssuedDate, cert.ExpiryDate, cert.IssuingAuthority, cert.CertificateNumber,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("staff_radiation_certification")
		}

		return nil
	})
}

// DeleteCertification soft-deletes a staff radiation certification
// TENANT-ISOLATED: Deletes via RLS
func (r *RadiationRepository) DeleteCertification(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE staff_radiation_certifications SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("staff_radiation_certification")
		}

		return nil
	})
}

// --- Dosimetry Records ---

// CreateDosimetryRecord creates a new dosimetry record
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *RadiationRepository) CreateDosimetryRecord(ctx context.Context, record *DosimetryRecord) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO dosimetry_records (
				id, tenant_id, employee_id, employee_name,
				measurement_period_start, measurement_period_end,
				dosimeter_type, dose_msv, body_region, notes
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			record.ID, tenantID, record.EmployeeID, record.EmployeeName,
			record.MeasurementPeriodStart, record.MeasurementPeriodEnd,
			record.DosimeterType, record.DoseMsv, record.BodyRegion, record.Notes,
		).Scan(&record.CreatedAt, &record.UpdatedAt)
	})
}

// ListDosimetryByEmployee lists dosimetry records for an employee
// TENANT-ISOLATED: Returns only records via RLS
func (r *RadiationRepository) ListDosimetryByEmployee(ctx context.Context, employeeID string) ([]*DosimetryRecord, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var records []*DosimetryRecord
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, employee_id, employee_name, measurement_period_start, measurement_period_end,
			       dosimeter_type, dose_msv, body_region, notes, created_at, updated_at
			FROM dosimetry_records WHERE employee_id = $1 AND deleted_at IS NULL
			ORDER BY measurement_period_end DESC
		`
		return r.db.SelectContext(ctx, &records, query, employeeID)
	})

	if err != nil {
		return nil, err
	}

	return records, nil
}

// ListAllDosimetry lists all dosimetry records with pagination
// TENANT-ISOLATED: Returns only records via RLS
func (r *RadiationRepository) ListAllDosimetry(ctx context.Context, page, perPage int) ([]*DosimetryRecord, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var records []*DosimetryRecord

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		countQuery := `SELECT COUNT(*) FROM dosimetry_records WHERE deleted_at IS NULL`
		if err := r.db.GetContext(ctx, &total, countQuery); err != nil {
			return err
		}

		query := `
			SELECT id, employee_id, employee_name, measurement_period_start, measurement_period_end,
			       dosimeter_type, dose_msv, body_region, notes, created_at, updated_at
			FROM dosimetry_records WHERE deleted_at IS NULL
			ORDER BY measurement_period_end DESC
			LIMIT $1 OFFSET $2
		`
		offset := (page - 1) * perPage
		return r.db.SelectContext(ctx, &records, query, perPage, offset)
	})

	if err != nil {
		return nil, 0, err
	}

	return records, total, nil
}
