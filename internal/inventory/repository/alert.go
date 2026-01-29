package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
)

// InventoryAlert represents an inventory alert
type InventoryAlert struct {
	ID              string     `db:"id" json:"id"`
	AlertType       string     `db:"alert_type" json:"alert_type"`
	ItemID          string     `db:"item_id" json:"item_id"`
	ItemName        string     `db:"item_name" json:"item_name"`
	BatchID         *string    `db:"batch_id" json:"batch_id,omitempty"`
	BatchNumber     *string    `db:"batch_number" json:"batch_number,omitempty"`
	Severity        string     `db:"severity" json:"severity"`
	Message         string     `db:"message" json:"message"`
	ExpiryDate      *time.Time `db:"expiry_date" json:"expiry_date,omitempty"`
	DaysUntilExpiry *int       `db:"days_until_expiry" json:"days_until_expiry,omitempty"`
	CurrentStock    *int       `db:"current_stock" json:"current_stock,omitempty"`
	MinStock        *int       `db:"min_stock" json:"min_stock,omitempty"`
	IsAcknowledged  bool       `db:"is_acknowledged" json:"is_acknowledged"`
	AcknowledgedBy  *string    `db:"acknowledged_by" json:"acknowledged_by,omitempty"`
	AcknowledgedAt  *time.Time `db:"acknowledged_at" json:"acknowledged_at,omitempty"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
}

// AlertRepository handles alert persistence
type AlertRepository struct {
	db *database.DB
}

// NewAlertRepository creates a new alert repository
func NewAlertRepository(db *database.DB) *AlertRepository {
	return &AlertRepository{db: db}
}

// Create creates a new alert
func (r *AlertRepository) Create(ctx context.Context, alert *InventoryAlert) error {
	if alert.ID == "" {
		alert.ID = uuid.New().String()
	}

	query := `
		INSERT INTO inventory_alerts (
			id, alert_type, item_id, item_name, batch_id, batch_number, severity, message,
			expiry_date, days_until_expiry, current_stock, min_stock
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at
	`

	return r.db.QueryRowxContext(ctx, query,
		alert.ID, alert.AlertType, alert.ItemID, alert.ItemName, alert.BatchID,
		alert.BatchNumber, alert.Severity, alert.Message, alert.ExpiryDate,
		alert.DaysUntilExpiry, alert.CurrentStock, alert.MinStock,
	).Scan(&alert.CreatedAt)
}

// GetByID gets an alert by ID
func (r *AlertRepository) GetByID(ctx context.Context, id string) (*InventoryAlert, error) {
	var alert InventoryAlert
	query := `SELECT * FROM inventory_alerts WHERE id = $1`
	if err := r.db.GetContext(ctx, &alert, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("alert")
		}
		return nil, err
	}
	return &alert, nil
}

// List lists alerts with filtering
func (r *AlertRepository) List(ctx context.Context, acknowledged *bool, alertType string, page, perPage int) ([]*InventoryAlert, int64, error) {
	var total int64
	args := []interface{}{}
	argIndex := 1

	countQuery := `SELECT COUNT(*) FROM inventory_alerts WHERE 1=1`
	query := `SELECT * FROM inventory_alerts WHERE 1=1`

	if acknowledged != nil {
		countQuery += ` AND is_acknowledged = $` + string(rune('0'+argIndex))
		query += ` AND is_acknowledged = $` + string(rune('0'+argIndex))
		args = append(args, *acknowledged)
		argIndex++
	}

	if alertType != "" {
		countQuery += ` AND alert_type = $` + string(rune('0'+argIndex))
		query += ` AND alert_type = $` + string(rune('0'+argIndex))
		args = append(args, alertType)
		argIndex++
	}

	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	query += ` ORDER BY CASE severity WHEN 'critical' THEN 0 ELSE 1 END, created_at DESC`

	offset := (page - 1) * perPage
	query += ` LIMIT $` + string(rune('0'+argIndex)) + ` OFFSET $` + string(rune('0'+argIndex+1))
	args = append(args, perPage, offset)

	var alerts []*InventoryAlert
	if err := r.db.SelectContext(ctx, &alerts, query, args...); err != nil {
		return nil, 0, err
	}

	return alerts, total, nil
}

// Acknowledge acknowledges an alert
func (r *AlertRepository) Acknowledge(ctx context.Context, id, userID string) error {
	query := `
		UPDATE inventory_alerts
		SET is_acknowledged = true, acknowledged_by = $2, acknowledged_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("alert")
	}

	return nil
}

// DeleteOld deletes old acknowledged alerts
func (r *AlertRepository) DeleteOld(ctx context.Context, olderThan time.Duration) error {
	query := `DELETE FROM inventory_alerts WHERE is_acknowledged = true AND acknowledged_at < $1`
	_, err := r.db.ExecContext(ctx, query, time.Now().Add(-olderThan))
	return err
}

// GetUnacknowledgedCount gets count of unacknowledged alerts
func (r *AlertRepository) GetUnacknowledgedCount(ctx context.Context) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM inventory_alerts WHERE is_acknowledged = false`
	if err := r.db.GetContext(ctx, &count, query); err != nil {
		return 0, err
	}
	return count, nil
}
