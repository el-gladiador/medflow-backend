package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/pkg/database"
)

// AuditRepository handles audit log persistence
type AuditRepository struct {
	db *database.DB
}

// NewAuditRepository creates a new audit repository
func NewAuditRepository(db *database.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Create creates a new audit log entry
func (r *AuditRepository) Create(ctx context.Context, log *domain.AuditLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}

	detailsJSON, err := json.Marshal(log.Details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	query := `
		INSERT INTO audit_logs (id, actor_id, actor_name, action, target_user_id, target_user_name,
		                        resource_type, resource_id, details, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at
	`

	return r.db.QueryRowxContext(ctx, query,
		log.ID,
		log.ActorID,
		log.ActorName,
		log.Action,
		log.TargetUserID,
		log.TargetUserName,
		log.ResourceType,
		log.ResourceID,
		detailsJSON,
		log.IPAddress,
		log.UserAgent,
	).Scan(&log.CreatedAt)
}

// ListFilter contains filter options for audit logs
type ListFilter struct {
	ActorID      string
	TargetUserID string
	Action       string
	FromDate     string
	ToDate       string
}

// List lists audit logs with pagination and filtering
func (r *AuditRepository) List(ctx context.Context, filter *ListFilter, page, perPage int) ([]*domain.AuditLog, int64, error) {
	args := []interface{}{}
	argIndex := 1

	countQuery := `SELECT COUNT(*) FROM audit_logs WHERE 1=1`
	query := `
		SELECT id, actor_id, actor_name, action, target_user_id, target_user_name,
		       resource_type, resource_id, details, ip_address, user_agent, created_at
		FROM audit_logs
		WHERE 1=1
	`

	if filter != nil {
		if filter.ActorID != "" {
			countQuery += ` AND actor_id = $` + string(rune('0'+argIndex))
			query += ` AND actor_id = $` + string(rune('0'+argIndex))
			args = append(args, filter.ActorID)
			argIndex++
		}
		if filter.TargetUserID != "" {
			countQuery += ` AND target_user_id = $` + string(rune('0'+argIndex))
			query += ` AND target_user_id = $` + string(rune('0'+argIndex))
			args = append(args, filter.TargetUserID)
			argIndex++
		}
		if filter.Action != "" {
			countQuery += ` AND action = $` + string(rune('0'+argIndex))
			query += ` AND action = $` + string(rune('0'+argIndex))
			args = append(args, filter.Action)
			argIndex++
		}
	}

	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	query += ` ORDER BY created_at DESC`

	offset := (page - 1) * perPage
	query += ` LIMIT $` + string(rune('0'+argIndex)) + ` OFFSET $` + string(rune('0'+argIndex+1))
	args = append(args, perPage, offset)

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*domain.AuditLog
	for rows.Next() {
		var log domain.AuditLog
		var detailsJSON []byte

		if err := rows.Scan(
			&log.ID, &log.ActorID, &log.ActorName, &log.Action,
			&log.TargetUserID, &log.TargetUserName, &log.ResourceType,
			&log.ResourceID, &detailsJSON, &log.IPAddress, &log.UserAgent, &log.CreatedAt,
		); err != nil {
			return nil, 0, err
		}

		if len(detailsJSON) > 0 {
			json.Unmarshal(detailsJSON, &log.Details)
		}

		logs = append(logs, &log)
	}

	return logs, total, nil
}
