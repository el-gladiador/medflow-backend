package service

import (
	"context"
	"fmt"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// BtmReceiveRequest represents a request to receive a controlled substance
type BtmReceiveRequest struct {
	ItemID             string  `json:"item_id"`
	Quantity           float64 `json:"quantity"`
	Unit               string  `json:"unit"`
	SupplierName       string  `json:"supplier_name"`
	DeliveryNoteNumber string  `json:"delivery_note_number"`
	Notes              string  `json:"notes,omitempty"`
}

// BtmDispenseRequest represents a request to dispense a controlled substance
type BtmDispenseRequest struct {
	ItemID            string  `json:"item_id"`
	Quantity          float64 `json:"quantity"`
	Unit              string  `json:"unit"`
	PatientIdentifier string  `json:"patient_identifier"`
	PrescribingDoctor string  `json:"prescribing_doctor"`
	Purpose           string  `json:"purpose,omitempty"`
	Notes             string  `json:"notes,omitempty"`
}

// BtmDisposeRequest represents a request to dispose of a controlled substance
type BtmDisposeRequest struct {
	ItemID          string  `json:"item_id"`
	Quantity        float64 `json:"quantity"`
	Unit            string  `json:"unit"`
	DisposalMethod  string  `json:"disposal_method"`
	DisposalWitness string  `json:"disposal_witness"`
	Notes           string  `json:"notes,omitempty"`
}

// BtmCorrectionRequest represents a request to correct a BtM register entry
type BtmCorrectionRequest struct {
	ItemID           string  `json:"item_id"`
	Quantity         float64 `json:"quantity"`
	Unit             string  `json:"unit"`
	CorrectionReason string  `json:"correction_reason"`
	CorrectsEntryID  string  `json:"corrects_entry_id"`
	Notes            string  `json:"notes,omitempty"`
}

// BtmCheckRequest represents a request for an inventory check of a controlled substance
type BtmCheckRequest struct {
	ItemID   string  `json:"item_id"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Notes    string  `json:"notes,omitempty"`
}

// BtmService handles BtM (Betaeubungsmittel) controlled substance business logic.
// Enforces authorization checks and maintains an append-only register per BtMG requirements.
type BtmService struct {
	btmRepo      *repository.BtmRepository
	btmAuthRepo  *repository.BtmAuthRepository
	itemRepo     *repository.ItemRepository
	auditService *AuditService
	logger       *logger.Logger
}

// NewBtmService creates a new BtM service
func NewBtmService(
	btmRepo *repository.BtmRepository,
	btmAuthRepo *repository.BtmAuthRepository,
	itemRepo *repository.ItemRepository,
	auditService *AuditService,
	log *logger.Logger,
) *BtmService {
	return &BtmService{
		btmRepo:      btmRepo,
		btmAuthRepo:  btmAuthRepo,
		itemRepo:     itemRepo,
		auditService: auditService,
		logger:       log,
	}
}

// ReceiveSubstance records receipt of a controlled substance
func (s *BtmService) ReceiveSubstance(ctx context.Context, req *BtmReceiveRequest) (*repository.BtmEntry, error) {
	userID := httputil.GetUserID(ctx)

	// Validate authorization (requires full access)
	if err := s.checkAuthorization(ctx, userID, "full"); err != nil {
		return nil, err
	}

	// Validate item exists
	if _, err := s.itemRepo.GetByID(ctx, req.ItemID); err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	entry := &repository.BtmEntry{
		ItemID:    req.ItemID,
		EntryType: "receipt",
		Quantity:  req.Quantity,
		Unit:      req.Unit,
		PerformedBy:     userID,
		PerformedByName: userID, // In production, resolve from user service
	}

	if req.SupplierName != "" {
		entry.SupplierName = &req.SupplierName
	}
	if req.DeliveryNoteNumber != "" {
		entry.DeliveryNoteNumber = &req.DeliveryNoteNumber
	}
	if req.Notes != "" {
		entry.Notes = &req.Notes
	}

	if err := s.btmRepo.CreateEntry(ctx, entry); err != nil {
		return nil, fmt.Errorf("failed to create BtM receipt entry: %w", err)
	}

	// Record audit
	s.auditService.RecordAction(ctx, "btm", req.ItemID, "btm_receipt", map[string]interface{}{
		"quantity":            req.Quantity,
		"unit":               req.Unit,
		"supplier":           req.SupplierName,
		"delivery_note":      req.DeliveryNoteNumber,
		"entry_number":       entry.EntryNumber,
		"running_balance":    entry.RunningBalance,
	})

	return entry, nil
}

// DispenseSubstance records dispensing of a controlled substance to a patient
func (s *BtmService) DispenseSubstance(ctx context.Context, req *BtmDispenseRequest) (*repository.BtmEntry, error) {
	userID := httputil.GetUserID(ctx)

	// Validate authorization (requires full or dispense_only)
	if err := s.checkAuthorization(ctx, userID, "dispense_only"); err != nil {
		return nil, err
	}

	// Validate item exists
	if _, err := s.itemRepo.GetByID(ctx, req.ItemID); err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Validate sufficient balance
	balance, err := s.btmRepo.GetRunningBalance(ctx, req.ItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get running balance: %w", err)
	}
	if balance < req.Quantity {
		return nil, errors.BadRequest(fmt.Sprintf("insufficient BtM balance: %.2f available, %.2f requested", balance, req.Quantity))
	}

	entry := &repository.BtmEntry{
		ItemID:          req.ItemID,
		EntryType:       "dispense",
		Quantity:        req.Quantity,
		Unit:            req.Unit,
		PerformedBy:     userID,
		PerformedByName: userID,
	}

	if req.PatientIdentifier != "" {
		entry.PatientIdentifier = &req.PatientIdentifier
	}
	if req.PrescribingDoctor != "" {
		entry.PrescribingDoctor = &req.PrescribingDoctor
	}
	if req.Purpose != "" {
		entry.Purpose = &req.Purpose
	}
	if req.Notes != "" {
		entry.Notes = &req.Notes
	}

	if err := s.btmRepo.CreateEntry(ctx, entry); err != nil {
		return nil, fmt.Errorf("failed to create BtM dispense entry: %w", err)
	}

	// Record audit
	s.auditService.RecordAction(ctx, "btm", req.ItemID, "btm_dispense", map[string]interface{}{
		"quantity":          req.Quantity,
		"unit":             req.Unit,
		"patient":          req.PatientIdentifier,
		"doctor":           req.PrescribingDoctor,
		"entry_number":     entry.EntryNumber,
		"running_balance":  entry.RunningBalance,
	})

	return entry, nil
}

// DisposeSubstance records disposal of a controlled substance (requires witness)
func (s *BtmService) DisposeSubstance(ctx context.Context, req *BtmDisposeRequest) (*repository.BtmEntry, error) {
	userID := httputil.GetUserID(ctx)

	// Validate authorization (requires full access)
	if err := s.checkAuthorization(ctx, userID, "full"); err != nil {
		return nil, err
	}

	// Validate item exists
	if _, err := s.itemRepo.GetByID(ctx, req.ItemID); err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Validate sufficient balance
	balance, err := s.btmRepo.GetRunningBalance(ctx, req.ItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get running balance: %w", err)
	}
	if balance < req.Quantity {
		return nil, errors.BadRequest(fmt.Sprintf("insufficient BtM balance: %.2f available, %.2f requested", balance, req.Quantity))
	}

	// Disposal requires a witness
	if req.DisposalWitness == "" {
		return nil, errors.BadRequest("disposal witness is required for BtM disposal")
	}

	entry := &repository.BtmEntry{
		ItemID:          req.ItemID,
		EntryType:       "disposal",
		Quantity:        req.Quantity,
		Unit:            req.Unit,
		PerformedBy:     userID,
		PerformedByName: userID,
	}

	if req.DisposalMethod != "" {
		entry.DisposalMethod = &req.DisposalMethod
	}
	entry.DisposalWitness = &req.DisposalWitness
	if req.Notes != "" {
		entry.Notes = &req.Notes
	}

	if err := s.btmRepo.CreateEntry(ctx, entry); err != nil {
		return nil, fmt.Errorf("failed to create BtM disposal entry: %w", err)
	}

	// Record audit
	s.auditService.RecordAction(ctx, "btm", req.ItemID, "btm_disposal", map[string]interface{}{
		"quantity":        req.Quantity,
		"unit":           req.Unit,
		"disposal_method": req.DisposalMethod,
		"witness":        req.DisposalWitness,
		"entry_number":   entry.EntryNumber,
		"running_balance": entry.RunningBalance,
	})

	return entry, nil
}

// CorrectEntry creates a correction entry that references the original
func (s *BtmService) CorrectEntry(ctx context.Context, req *BtmCorrectionRequest) (*repository.BtmEntry, error) {
	userID := httputil.GetUserID(ctx)

	// Validate authorization (requires full access)
	if err := s.checkAuthorization(ctx, userID, "full"); err != nil {
		return nil, err
	}

	// Validate correction reason
	if req.CorrectionReason == "" {
		return nil, errors.BadRequest("correction reason is required")
	}
	if req.CorrectsEntryID == "" {
		return nil, errors.BadRequest("corrects_entry_id is required")
	}

	entry := &repository.BtmEntry{
		ItemID:          req.ItemID,
		EntryType:       "correction",
		Quantity:        req.Quantity,
		Unit:            req.Unit,
		PerformedBy:     userID,
		PerformedByName: userID,
	}

	entry.CorrectionReason = &req.CorrectionReason
	entry.CorrectsEntryID = &req.CorrectsEntryID
	if req.Notes != "" {
		entry.Notes = &req.Notes
	}

	if err := s.btmRepo.CreateEntry(ctx, entry); err != nil {
		return nil, fmt.Errorf("failed to create BtM correction entry: %w", err)
	}

	// Record audit
	s.auditService.RecordAction(ctx, "btm", req.ItemID, "btm_correction", map[string]interface{}{
		"quantity":          req.Quantity,
		"unit":             req.Unit,
		"correction_reason": req.CorrectionReason,
		"corrects_entry":   req.CorrectsEntryID,
		"entry_number":     entry.EntryNumber,
		"running_balance":  entry.RunningBalance,
	})

	return entry, nil
}

// InventoryCheck records an inventory check (physical count) for a controlled substance
func (s *BtmService) InventoryCheck(ctx context.Context, req *BtmCheckRequest) (*repository.BtmEntry, error) {
	userID := httputil.GetUserID(ctx)

	// Validate authorization (requires full access)
	if err := s.checkAuthorization(ctx, userID, "full"); err != nil {
		return nil, err
	}

	entry := &repository.BtmEntry{
		ItemID:          req.ItemID,
		EntryType:       "inventory_check",
		Quantity:        req.Quantity,
		Unit:            req.Unit,
		PerformedBy:     userID,
		PerformedByName: userID,
	}

	if req.Notes != "" {
		entry.Notes = &req.Notes
	}

	if err := s.btmRepo.CreateEntry(ctx, entry); err != nil {
		return nil, fmt.Errorf("failed to create BtM inventory check entry: %w", err)
	}

	// Record audit
	s.auditService.RecordAction(ctx, "btm", req.ItemID, "btm_inventory_check", map[string]interface{}{
		"quantity":        req.Quantity,
		"unit":           req.Unit,
		"entry_number":   entry.EntryNumber,
		"running_balance": entry.RunningBalance,
	})

	return entry, nil
}

// GetRegister retrieves the BtM register for an item (paginated)
func (s *BtmService) GetRegister(ctx context.Context, itemID string, page, perPage int) ([]*repository.BtmEntry, int64, error) {
	userID := httputil.GetUserID(ctx)

	// Validate authorization (requires at least view_only)
	if err := s.checkAuthorization(ctx, userID, "view_only"); err != nil {
		return nil, 0, err
	}

	return s.btmRepo.ListByItem(ctx, itemID, page, perPage)
}

// GetBalance gets the current running balance for an item
func (s *BtmService) GetBalance(ctx context.Context, itemID string) (float64, error) {
	return s.btmRepo.GetRunningBalance(ctx, itemID)
}

// ListAuthorizedPersonnel lists all active BtM authorized personnel
func (s *BtmService) ListAuthorizedPersonnel(ctx context.Context) ([]*repository.BtmAuthorizedPerson, error) {
	return s.btmAuthRepo.List(ctx)
}

// CreateAuthorizedPerson creates a new BtM authorized person
func (s *BtmService) CreateAuthorizedPerson(ctx context.Context, person *repository.BtmAuthorizedPerson) error {
	return s.btmAuthRepo.Create(ctx, person)
}

// RevokeAuthorization revokes a BtM authorization
func (s *BtmService) RevokeAuthorization(ctx context.Context, id, revokedBy, revokedByName string) error {
	return s.btmAuthRepo.Revoke(ctx, id, revokedBy, revokedByName)
}

// checkAuthorization verifies the user has sufficient BtM authorization
func (s *BtmService) checkAuthorization(ctx context.Context, userID, requiredType string) error {
	if userID == "" {
		return errors.Forbidden("user context required for BtM operations")
	}

	authorized, err := s.btmAuthRepo.IsAuthorized(ctx, userID, requiredType)
	if err != nil {
		return fmt.Errorf("failed to check BtM authorization: %w", err)
	}

	if !authorized {
		return errors.Forbidden(fmt.Sprintf("user not authorized for BtM operation (requires %s)", requiredType))
	}

	return nil
}
