package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// BtmEntry represents a single entry in the BtM (Betaeubungsmittel) controlled substance register.
// BtM entries are append-only: corrections are new entries that reference the original.
type BtmEntry struct {
	ID                 string    `db:"id" json:"id"`
	ItemID             string    `db:"item_id" json:"item_id"`
	EntryNumber        int       `db:"entry_number" json:"entry_number"`
	EntryType          string    `db:"entry_type" json:"entry_type"` // receipt, dispense, disposal, correction, inventory_check
	Quantity           float64   `db:"quantity" json:"quantity"`
	RunningBalance     float64   `db:"running_balance" json:"running_balance"`
	Unit               string    `db:"unit" json:"unit"`
	SupplierName       *string   `db:"supplier_name" json:"supplier_name,omitempty"`
	DeliveryNoteNumber *string   `db:"delivery_note_number" json:"delivery_note_number,omitempty"`
	PatientIdentifier  *string   `db:"patient_identifier" json:"patient_identifier,omitempty"`
	PrescribingDoctor  *string   `db:"prescribing_doctor" json:"prescribing_doctor,omitempty"`
	Purpose            *string   `db:"purpose" json:"purpose,omitempty"`
	DisposalMethod     *string   `db:"disposal_method" json:"disposal_method,omitempty"`
	DisposalWitness    *string   `db:"disposal_witness" json:"disposal_witness,omitempty"`
	CorrectionReason   *string   `db:"correction_reason" json:"correction_reason,omitempty"`
	CorrectsEntryID    *string   `db:"corrects_entry_id" json:"corrects_entry_id,omitempty"`
	PerformedBy        string    `db:"performed_by" json:"performed_by"`
	PerformedByName    string    `db:"performed_by_name" json:"performed_by_name"`
	Notes              *string   `db:"notes" json:"notes,omitempty"`
	CreatedAt          time.Time `db:"created_at" json:"created_at"`
}

// BtmRepository handles BtM controlled substance register persistence.
// All operations are append-only with sequential entry numbers and running balances.
type BtmRepository struct {
	db *database.DB
}

// NewBtmRepository creates a new BtM repository
func NewBtmRepository(db *database.DB) *BtmRepository {
	return &BtmRepository{db: db}
}

// CreateEntry creates a new BtM register entry with auto-calculated entry number and running balance.
// TENANT-ISOLATED: Inserts with tenant_id for RLS.
// Uses FOR UPDATE locks to ensure sequential consistency.
func (r *BtmRepository) CreateEntry(ctx context.Context, entry *BtmEntry) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		// Get next entry number (locked for sequential consistency)
		var entryNumber int
		entryNumQuery := `SELECT COALESCE(MAX(entry_number), 0) + 1 FROM btm_register WHERE item_id = $1 FOR UPDATE`
		if err := r.db.GetContext(ctx, &entryNumber, entryNumQuery, entry.ItemID); err != nil {
			return fmt.Errorf("failed to get next entry number: %w", err)
		}
		entry.EntryNumber = entryNumber

		// Get current running balance (locked for consistency)
		var currentBalance sql.NullFloat64
		balanceQuery := `SELECT running_balance FROM btm_register WHERE item_id = $1 ORDER BY entry_number DESC LIMIT 1 FOR UPDATE`
		err := r.db.GetContext(ctx, &currentBalance, balanceQuery, entry.ItemID)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to get current balance: %w", err)
		}

		balance := float64(0)
		if currentBalance.Valid {
			balance = currentBalance.Float64
		}

		// Calculate new running balance based on entry type
		switch entry.EntryType {
		case "receipt":
			balance += entry.Quantity
		case "dispense", "disposal":
			balance -= entry.Quantity
		case "correction":
			// Correction quantity can be positive (add) or negative (subtract)
			balance += entry.Quantity
		case "inventory_check":
			// Inventory check sets the balance to the provided quantity
			balance = entry.Quantity
		}

		entry.RunningBalance = balance

		// Insert the entry
		query := `
			INSERT INTO btm_register (
				id, tenant_id, item_id, entry_number, entry_type, quantity, running_balance, unit,
				supplier_name, delivery_note_number, patient_identifier, prescribing_doctor,
				purpose, disposal_method, disposal_witness, correction_reason, corrects_entry_id,
				performed_by, performed_by_name, notes
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
			RETURNING created_at
		`

		return r.db.QueryRowxContext(ctx, query,
			entry.ID, tenantID, entry.ItemID, entry.EntryNumber, entry.EntryType,
			entry.Quantity, entry.RunningBalance, entry.Unit,
			entry.SupplierName, entry.DeliveryNoteNumber, entry.PatientIdentifier,
			entry.PrescribingDoctor, entry.Purpose, entry.DisposalMethod,
			entry.DisposalWitness, entry.CorrectionReason, entry.CorrectsEntryID,
			entry.PerformedBy, entry.PerformedByName, entry.Notes,
		).Scan(&entry.CreatedAt)
	})
}

// ListByItem lists BtM register entries for an item with pagination, ordered by entry number
// TENANT-ISOLATED: Returns only entries via RLS
func (r *BtmRepository) ListByItem(ctx context.Context, itemID string, page, perPage int) ([]*BtmEntry, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var entries []*BtmEntry

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		countQuery := `SELECT COUNT(*) FROM btm_register WHERE item_id = $1`
		if err := r.db.GetContext(ctx, &total, countQuery, itemID); err != nil {
			return err
		}

		offset := (page - 1) * perPage
		query := `
			SELECT id, item_id, entry_number, entry_type, quantity, running_balance, unit,
			       supplier_name, delivery_note_number, patient_identifier, prescribing_doctor,
			       purpose, disposal_method, disposal_witness, correction_reason, corrects_entry_id,
			       performed_by, performed_by_name, notes, created_at
			FROM btm_register
			WHERE item_id = $1
			ORDER BY entry_number ASC
			LIMIT $2 OFFSET $3
		`

		return r.db.SelectContext(ctx, &entries, query, itemID, perPage, offset)
	})

	if err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

// GetRunningBalance gets the current running balance for an item
// TENANT-ISOLATED: Queries via RLS
func (r *BtmRepository) GetRunningBalance(ctx context.Context, itemID string) (float64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return 0, err
	}

	var balance sql.NullFloat64

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `SELECT running_balance FROM btm_register WHERE item_id = $1 ORDER BY entry_number DESC LIMIT 1`
		return r.db.GetContext(ctx, &balance, query, itemID)
	})

	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	if !balance.Valid {
		return 0, nil
	}

	return balance.Float64, nil
}

// GetLastEntry gets the most recent entry for an item
// TENANT-ISOLATED: Queries via RLS
func (r *BtmRepository) GetLastEntry(ctx context.Context, itemID string) (*BtmEntry, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var entry BtmEntry

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, entry_number, entry_type, quantity, running_balance, unit,
			       supplier_name, delivery_note_number, patient_identifier, prescribing_doctor,
			       purpose, disposal_method, disposal_witness, correction_reason, corrects_entry_id,
			       performed_by, performed_by_name, notes, created_at
			FROM btm_register
			WHERE item_id = $1
			ORDER BY entry_number DESC
			LIMIT 1
		`
		return r.db.GetContext(ctx, &entry, query, itemID)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("btm_entry")
	}
	if err != nil {
		return nil, err
	}

	return &entry, nil
}
