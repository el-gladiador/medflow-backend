package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/tenant"
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
// TENANT-ISOLATED: Inserts with tenant_id for RLS filtering
func (r *AuditRepository) Create(ctx context.Context, log *domain.AuditLog) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if log.ID == "" {
		log.ID = uuid.New().String()
	}

	detailsJSON, err := json.Marshal(log.Details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	oldValuesJSON, err := json.Marshal(log.OldValues)
	if err != nil {
		oldValuesJSON = nil
	}

	newValuesJSON, err := json.Marshal(log.NewValues)
	if err != nil {
		newValuesJSON = nil
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO audit_logs (id, tenant_id, actor_id, actor_name, action, resource_type, resource_id,
			                        target_user_id, target_user_name, old_values, new_values,
			                        details, ip_address, user_agent)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			RETURNING created_at
		`

		return r.db.QueryRowxContext(ctx, query,
			log.ID,
			tenantID,
			log.ActorID,
			log.ActorName,
			log.Action,
			log.ResourceType,
			log.ResourceID,
			log.TargetUserID,
			log.TargetUserName,
			oldValuesJSON,
			newValuesJSON,
			detailsJSON,
			log.IPAddress,
			log.UserAgent,
		).Scan(&log.CreatedAt)
	})
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
// TENANT-ISOLATED: Returns only audit logs visible to the tenant via RLS
func (r *AuditRepository) List(ctx context.Context, filter *ListFilter, page, perPage int) ([]*domain.AuditLog, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var logs []*domain.AuditLog

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		args := []interface{}{}
		argIndex := 1

		countQuery := `SELECT COUNT(*) FROM audit_logs WHERE 1=1`
		query := `
			SELECT id, actor_id, actor_name, action, resource_type, resource_id,
			       target_user_id, target_user_name, old_values, new_values,
			       details, ip_address, user_agent, created_at
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

		if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
			return err
		}

		query += ` ORDER BY created_at DESC`

		offset := (page - 1) * perPage
		query += ` LIMIT $` + string(rune('0'+argIndex)) + ` OFFSET $` + string(rune('0'+argIndex+1))
		args = append(args, perPage, offset)

		rows, err := r.db.QueryxContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var log domain.AuditLog
			var oldValuesJSON, newValuesJSON, detailsJSON []byte

			if err := rows.Scan(
				&log.ID, &log.ActorID, &log.ActorName, &log.Action,
				&log.ResourceType, &log.ResourceID,
				&log.TargetUserID, &log.TargetUserName,
				&oldValuesJSON, &newValuesJSON,
				&detailsJSON, &log.IPAddress, &log.UserAgent, &log.CreatedAt,
			); err != nil {
				return err
			}

			if len(oldValuesJSON) > 0 {
				json.Unmarshal(oldValuesJSON, &log.OldValues)
			}
			if len(newValuesJSON) > 0 {
				json.Unmarshal(newValuesJSON, &log.NewValues)
			}
			if len(detailsJSON) > 0 {
				json.Unmarshal(detailsJSON, &log.Details)
			}

			logs = append(logs, &log)
		}

		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
