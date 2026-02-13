package service

import (
	"context"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// RadiationService handles radiation protection business logic (StrlSchV/RoV compliance)
type RadiationService struct {
	radiationRepo *repository.RadiationRepository
	auditService  *AuditService
	logger        *logger.Logger
}

// NewRadiationService creates a new radiation service
func NewRadiationService(radiationRepo *repository.RadiationRepository, auditService *AuditService, log *logger.Logger) *RadiationService {
	return &RadiationService{
		radiationRepo: radiationRepo,
		auditService:  auditService,
		logger:        log,
	}
}

// --- Radiation Devices ---

// CreateDevice creates a new radiation device
func (s *RadiationService) CreateDevice(ctx context.Context, device *repository.RadiationDevice) error {
	if err := s.radiationRepo.CreateDevice(ctx, device); err != nil {
		return err
	}

	s.auditService.RecordCreate(ctx, "radiation_device", device.ID, map[string]interface{}{
		"item_id":            device.ItemID,
		"device_category":    device.DeviceCategory,
		"responsible_person": device.ResponsiblePerson,
	})

	return nil
}

// GetDevice gets a radiation device by ID
func (s *RadiationService) GetDevice(ctx context.Context, id string) (*repository.RadiationDevice, error) {
	return s.radiationRepo.GetDevice(ctx, id)
}

// ListDevices lists all radiation devices
func (s *RadiationService) ListDevices(ctx context.Context) ([]*repository.RadiationDevice, error) {
	return s.radiationRepo.ListDevices(ctx)
}

// UpdateDevice updates a radiation device
func (s *RadiationService) UpdateDevice(ctx context.Context, device *repository.RadiationDevice) error {
	if err := s.radiationRepo.UpdateDevice(ctx, device); err != nil {
		return err
	}

	s.auditService.RecordUpdate(ctx, "radiation_device", device.ID, map[string]interface{}{
		"device_category":    device.DeviceCategory,
		"responsible_person": device.ResponsiblePerson,
	}, nil)

	return nil
}

// DeleteDevice soft-deletes a radiation device
func (s *RadiationService) DeleteDevice(ctx context.Context, id string) error {
	if err := s.radiationRepo.DeleteDevice(ctx, id); err != nil {
		return err
	}

	s.auditService.RecordDelete(ctx, "radiation_device", id, nil)

	return nil
}

// --- Constancy Tests ---

// CreateTest creates a new constancy test
func (s *RadiationService) CreateTest(ctx context.Context, test *repository.ConstancyTest) error {
	if err := s.radiationRepo.CreateTest(ctx, test); err != nil {
		return err
	}

	s.auditService.RecordCreate(ctx, "constancy_test", test.ID, map[string]interface{}{
		"device_id":  test.DeviceID,
		"test_type":  test.TestType,
		"result":     test.Result,
	})

	return nil
}

// ListTestsByDevice lists constancy tests for a device
func (s *RadiationService) ListTestsByDevice(ctx context.Context, deviceID string) ([]*repository.ConstancyTest, error) {
	return s.radiationRepo.ListTestsByDevice(ctx, deviceID)
}

// UpdateTest updates a constancy test
func (s *RadiationService) UpdateTest(ctx context.Context, test *repository.ConstancyTest) error {
	if err := s.radiationRepo.UpdateTest(ctx, test); err != nil {
		return err
	}

	s.auditService.RecordUpdate(ctx, "constancy_test", test.ID, map[string]interface{}{
		"test_type": test.TestType,
		"result":    test.Result,
	}, nil)

	return nil
}

// DeleteTest soft-deletes a constancy test
func (s *RadiationService) DeleteTest(ctx context.Context, id string) error {
	if err := s.radiationRepo.DeleteTest(ctx, id); err != nil {
		return err
	}

	s.auditService.RecordDelete(ctx, "constancy_test", id, nil)

	return nil
}

// --- Expert Inspections ---

// CreateExpertInspection creates a new expert inspection record
func (s *RadiationService) CreateExpertInspection(ctx context.Context, insp *repository.ExpertInspection) error {
	if err := s.radiationRepo.CreateExpertInspection(ctx, insp); err != nil {
		return err
	}

	s.auditService.RecordCreate(ctx, "expert_inspection", insp.ID, map[string]interface{}{
		"device_id":      insp.DeviceID,
		"inspector_name": insp.InspectorName,
		"result":         insp.Result,
	})

	return nil
}

// ListExpertInspectionsByDevice lists expert inspections for a device
func (s *RadiationService) ListExpertInspectionsByDevice(ctx context.Context, deviceID string) ([]*repository.ExpertInspection, error) {
	return s.radiationRepo.ListExpertInspectionsByDevice(ctx, deviceID)
}

// UpdateExpertInspection updates an expert inspection
func (s *RadiationService) UpdateExpertInspection(ctx context.Context, insp *repository.ExpertInspection) error {
	if err := s.radiationRepo.UpdateExpertInspection(ctx, insp); err != nil {
		return err
	}

	s.auditService.RecordUpdate(ctx, "expert_inspection", insp.ID, map[string]interface{}{
		"inspector_name": insp.InspectorName,
		"result":         insp.Result,
	}, nil)

	return nil
}

// DeleteExpertInspection soft-deletes an expert inspection
func (s *RadiationService) DeleteExpertInspection(ctx context.Context, id string) error {
	if err := s.radiationRepo.DeleteExpertInspection(ctx, id); err != nil {
		return err
	}

	s.auditService.RecordDelete(ctx, "expert_inspection", id, nil)

	return nil
}

// --- Staff Radiation Certifications ---

// CreateCertification creates a new staff radiation certification
func (s *RadiationService) CreateCertification(ctx context.Context, cert *repository.StaffRadiationCertification) error {
	if err := s.radiationRepo.CreateCertification(ctx, cert); err != nil {
		return err
	}

	s.auditService.RecordCreate(ctx, "staff_radiation_certification", cert.ID, map[string]interface{}{
		"employee_id":        cert.EmployeeID,
		"employee_name":      cert.EmployeeName,
		"certification_type": cert.CertificationType,
	})

	return nil
}

// ListCertifications lists all staff radiation certifications
func (s *RadiationService) ListCertifications(ctx context.Context) ([]*repository.StaffRadiationCertification, error) {
	return s.radiationRepo.ListCertifications(ctx)
}

// UpdateCertification updates a staff radiation certification
func (s *RadiationService) UpdateCertification(ctx context.Context, cert *repository.StaffRadiationCertification) error {
	if err := s.radiationRepo.UpdateCertification(ctx, cert); err != nil {
		return err
	}

	s.auditService.RecordUpdate(ctx, "staff_radiation_certification", cert.ID, map[string]interface{}{
		"certification_type": cert.CertificationType,
		"expiry_date":        cert.ExpiryDate,
	}, nil)

	return nil
}

// DeleteCertification soft-deletes a staff radiation certification
func (s *RadiationService) DeleteCertification(ctx context.Context, id string) error {
	if err := s.radiationRepo.DeleteCertification(ctx, id); err != nil {
		return err
	}

	s.auditService.RecordDelete(ctx, "staff_radiation_certification", id, nil)

	return nil
}

// --- Dosimetry Records ---

// CreateDosimetryRecord creates a new dosimetry record
func (s *RadiationService) CreateDosimetryRecord(ctx context.Context, record *repository.DosimetryRecord) error {
	if err := s.radiationRepo.CreateDosimetryRecord(ctx, record); err != nil {
		return err
	}

	s.auditService.RecordCreate(ctx, "dosimetry_record", record.ID, map[string]interface{}{
		"employee_id":   record.EmployeeID,
		"employee_name": record.EmployeeName,
		"dose_msv":      record.DoseMsv,
		"body_region":   record.BodyRegion,
	})

	return nil
}

// ListDosimetryByEmployee lists dosimetry records for an employee
func (s *RadiationService) ListDosimetryByEmployee(ctx context.Context, employeeID string) ([]*repository.DosimetryRecord, error) {
	return s.radiationRepo.ListDosimetryByEmployee(ctx, employeeID)
}

// ListAllDosimetry lists all dosimetry records with pagination
func (s *RadiationService) ListAllDosimetry(ctx context.Context, page, perPage int) ([]*repository.DosimetryRecord, int64, error) {
	return s.radiationRepo.ListAllDosimetry(ctx, page, perPage)
}
