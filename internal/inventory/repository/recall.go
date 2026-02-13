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

// FieldSafetyNotice represents a field safety notice (Sicherheitskorrekturmaßnahme/FSN) or recall
type FieldSafetyNotice struct {
	ID                    string     `db:"id" json:"id"`
	NoticeNumber          string     `db:"notice_number" json:"notice_number"`
	NoticeType            string     `db:"notice_type" json:"notice_type"`   // recall, safety_alert, field_correction
	Severity              string     `db:"severity" json:"severity"`         // critical, high, medium, low
	Title                 string     `db:"title" json:"title"`
	Description           *string    `db:"description" json:"description,omitempty"`
	Manufacturer          *string    `db:"manufacturer" json:"manufacturer,omitempty"`
	AffectedProduct       *string    `db:"affected_product" json:"affected_product,omitempty"`
	AffectedBatchNumbers  *string    `db:"affected_batch_numbers" json:"affected_batch_numbers,omitempty"`
	AffectedUdiDIs        *string    `db:"affected_udi_dis" json:"affected_udi_dis,omitempty"`
	AffectedSerialNumbers *string    `db:"affected_serial_numbers" json:"affected_serial_numbers,omitempty"`
	Source                *string    `db:"source" json:"source,omitempty"`
	SourceURL             *string    `db:"source_url" json:"source_url,omitempty"`
	NoticeDate            *time.Time `db:"notice_date" json:"notice_date,omitempty"`
	ReceivedDate          time.Time  `db:"received_date" json:"received_date"`
	Status                string     `db:"status" json:"status"` // open, in_progress, resolved, dismissed
	CreatedAt             time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt             *time.Time `db:"deleted_at" json:"-"`
}

// RecallMatch represents a match between a field safety notice and inventory items/batches
type RecallMatch struct {
	ID           string     `db:"id" json:"id"`
	NoticeID     string     `db:"notice_id" json:"notice_id"`
	ItemID       string     `db:"item_id" json:"item_id"`
	BatchID      *string    `db:"batch_id" json:"batch_id,omitempty"`
	MatchType    string     `db:"match_type" json:"match_type"` // udi_di, serial_number, batch_number
	MatchedValue *string    `db:"matched_value" json:"matched_value,omitempty"`
	ActionTaken  *string    `db:"action_taken" json:"action_taken,omitempty"`
	ActionDate   *time.Time `db:"action_date" json:"action_date,omitempty"`
	ActionBy     *string    `db:"action_by" json:"action_by,omitempty"`
	Status       string     `db:"status" json:"status"` // pending, in_progress, resolved, dismissed
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at" json:"-"`
	// Computed fields for API (not in DB)
	ItemName    string  `db:"-" json:"item_name,omitempty"`
	BatchNumber *string `db:"-" json:"batch_number,omitempty"`
}

// RecallRepository handles field safety notice and recall match persistence
type RecallRepository struct {
	db *database.DB
}

// NewRecallRepository creates a new recall repository
func NewRecallRepository(db *database.DB) *RecallRepository {
	return &RecallRepository{db: db}
}

// CreateNotice creates a new field safety notice
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *RecallRepository) CreateNotice(ctx context.Context, notice *FieldSafetyNotice) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if notice.ID == "" {
		notice.ID = uuid.New().String()
	}

	if notice.Status == "" {
		notice.Status = "open"
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO field_safety_notices (
				id, tenant_id, notice_number, notice_type, severity, title, description,
				manufacturer, affected_product, affected_batch_numbers, affected_udi_dis,
				affected_serial_numbers, source, source_url, notice_date, received_date, status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			RETURNING created_at, updated_at
		`

		return r.db.QueryRowxContext(ctx, query,
			notice.ID, tenantID, notice.NoticeNumber, notice.NoticeType, notice.Severity,
			notice.Title, notice.Description, notice.Manufacturer, notice.AffectedProduct,
			notice.AffectedBatchNumbers, notice.AffectedUdiDIs, notice.AffectedSerialNumbers,
			notice.Source, notice.SourceURL, notice.NoticeDate, notice.ReceivedDate, notice.Status,
		).Scan(&notice.CreatedAt, &notice.UpdatedAt)
	})
}

// GetNotice gets a field safety notice by ID
// TENANT-ISOLATED: Queries via RLS
func (r *RecallRepository) GetNotice(ctx context.Context, id string) (*FieldSafetyNotice, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var notice FieldSafetyNotice

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, notice_number, notice_type, severity, title, description,
			       manufacturer, affected_product, affected_batch_numbers, affected_udi_dis,
			       affected_serial_numbers, source, source_url, notice_date, received_date,
			       status, created_at, updated_at
			FROM field_safety_notices
			WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &notice, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("field_safety_notice")
	}
	if err != nil {
		return nil, err
	}

	return &notice, nil
}

// ListNotices lists field safety notices with optional status filter and pagination
// TENANT-ISOLATED: Returns only notices via RLS
func (r *RecallRepository) ListNotices(ctx context.Context, status string, page, perPage int) ([]*FieldSafetyNotice, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var notices []*FieldSafetyNotice

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		args := []interface{}{}
		argIdx := 1

		countQuery := `SELECT COUNT(*) FROM field_safety_notices WHERE deleted_at IS NULL`
		query := `
			SELECT id, notice_number, notice_type, severity, title, description,
			       manufacturer, affected_product, affected_batch_numbers, affected_udi_dis,
			       affected_serial_numbers, source, source_url, notice_date, received_date,
			       status, created_at, updated_at
			FROM field_safety_notices WHERE deleted_at IS NULL
		`

		if status != "" {
			countQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
			query += fmt.Sprintf(` AND status = $%d`, argIdx)
			args = append(args, status)
			argIdx++
		}

		if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
			return err
		}

		query += ` ORDER BY CASE severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END, received_date DESC`

		offset := (page - 1) * perPage
		query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
		args = append(args, perPage, offset)

		return r.db.SelectContext(ctx, &notices, query, args...)
	})

	if err != nil {
		return nil, 0, err
	}

	return notices, total, nil
}

// UpdateNotice updates a field safety notice
// TENANT-ISOLATED: Updates via RLS
func (r *RecallRepository) UpdateNotice(ctx context.Context, notice *FieldSafetyNotice) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE field_safety_notices SET
				notice_number = $2, notice_type = $3, severity = $4, title = $5,
				description = $6, manufacturer = $7, affected_product = $8,
				affected_batch_numbers = $9, affected_udi_dis = $10,
				affected_serial_numbers = $11, source = $12, source_url = $13,
				notice_date = $14, status = $15, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`

		result, err := r.db.ExecContext(ctx, query,
			notice.ID, notice.NoticeNumber, notice.NoticeType, notice.Severity,
			notice.Title, notice.Description, notice.Manufacturer, notice.AffectedProduct,
			notice.AffectedBatchNumbers, notice.AffectedUdiDIs, notice.AffectedSerialNumbers,
			notice.Source, notice.SourceURL, notice.NoticeDate, notice.Status,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("field_safety_notice")
		}

		return nil
	})
}

// CreateMatch creates a new recall match
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *RecallRepository) CreateMatch(ctx context.Context, match *RecallMatch) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if match.ID == "" {
		match.ID = uuid.New().String()
	}

	if match.Status == "" {
		match.Status = "pending"
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO recall_matches (
				id, tenant_id, notice_id, item_id, batch_id, match_type,
				matched_value, status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at, updated_at
		`

		return r.db.QueryRowxContext(ctx, query,
			match.ID, tenantID, match.NoticeID, match.ItemID, match.BatchID,
			match.MatchType, match.MatchedValue, match.Status,
		).Scan(&match.CreatedAt, &match.UpdatedAt)
	})
}

// ListMatchesByNotice lists recall matches for a specific notice
// TENANT-ISOLATED: Returns only matches via RLS
func (r *RecallRepository) ListMatchesByNotice(ctx context.Context, noticeID string) ([]*RecallMatch, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var matches []*RecallMatch

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, notice_id, item_id, batch_id, match_type, matched_value,
			       action_taken, action_date, action_by, status, created_at, updated_at
			FROM recall_matches
			WHERE notice_id = $1 AND deleted_at IS NULL
			ORDER BY created_at DESC
		`
		return r.db.SelectContext(ctx, &matches, query, noticeID)
	})

	if err != nil {
		return nil, err
	}

	return matches, nil
}

// ListMatchesByItem lists recall matches for a specific item
// TENANT-ISOLATED: Returns only matches via RLS
func (r *RecallRepository) ListMatchesByItem(ctx context.Context, itemID string) ([]*RecallMatch, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var matches []*RecallMatch

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, notice_id, item_id, batch_id, match_type, matched_value,
			       action_taken, action_date, action_by, status, created_at, updated_at
			FROM recall_matches
			WHERE item_id = $1 AND deleted_at IS NULL
			ORDER BY created_at DESC
		`
		return r.db.SelectContext(ctx, &matches, query, itemID)
	})

	if err != nil {
		return nil, err
	}

	return matches, nil
}

// UpdateMatchStatus updates the status and action for a recall match
// TENANT-ISOLATED: Updates via RLS
func (r *RecallRepository) UpdateMatchStatus(ctx context.Context, matchID, status, actionTaken, actionBy string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE recall_matches SET
				status = $2, action_taken = $3, action_by = $4, action_date = NOW(), updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`

		result, err := r.db.ExecContext(ctx, query, matchID, status, actionTaken, actionBy)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("recall_match")
		}

		return nil
	})
}

// ListPendingMatches lists all pending recall matches across items
// TENANT-ISOLATED: Returns only matches via RLS
func (r *RecallRepository) ListPendingMatches(ctx context.Context) ([]*RecallMatch, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var matches []*RecallMatch

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		countQuery := `SELECT COUNT(*) FROM recall_matches WHERE status = 'pending' AND deleted_at IS NULL`
		if err := r.db.GetContext(ctx, &total, countQuery); err != nil {
			return err
		}

		query := `
			SELECT id, notice_id, item_id, batch_id, match_type, matched_value,
			       action_taken, action_date, action_by, status, created_at, updated_at
			FROM recall_matches
			WHERE status = 'pending' AND deleted_at IS NULL
			ORDER BY created_at DESC
		`
		return r.db.SelectContext(ctx, &matches, query)
	})

	if err != nil {
		return nil, 0, err
	}

	return matches, total, nil
}
