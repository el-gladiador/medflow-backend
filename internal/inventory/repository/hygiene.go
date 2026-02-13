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

// HygienePlan represents a hygiene plan (IfSG compliance)
type HygienePlan struct {
	ID             string     `db:"id" json:"id"`
	Title          string     `db:"title" json:"title"`
	Version        int        `db:"version" json:"version"`
	Category       string     `db:"category" json:"category"`
	Content        *string    `db:"content" json:"content,omitempty"`
	EffectiveFrom  *time.Time `db:"effective_from" json:"effective_from,omitempty"`
	EffectiveUntil *time.Time `db:"effective_until" json:"effective_until,omitempty"`
	ApprovedBy     *string    `db:"approved_by" json:"approved_by,omitempty"`
	ApprovedByName *string    `db:"approved_by_name" json:"approved_by_name,omitempty"`
	ApprovedAt     *time.Time `db:"approved_at" json:"approved_at,omitempty"`
	Status         string     `db:"status" json:"status"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at" json:"-"`
}

// HygieneInspection represents a hygiene inspection record
type HygieneInspection struct {
	ID                string     `db:"id" json:"id"`
	PlanID            *string    `db:"plan_id" json:"plan_id,omitempty"`
	InspectionDate    time.Time  `db:"inspection_date" json:"inspection_date"`
	InspectorName     string     `db:"inspector_name" json:"inspector_name"`
	AreaInspected     *string    `db:"area_inspected" json:"area_inspected,omitempty"`
	ChecklistResults  *string    `db:"checklist_results" json:"checklist_results,omitempty"`
	OverallResult     string     `db:"overall_result" json:"overall_result"`
	CorrectiveActions *string    `db:"corrective_actions" json:"corrective_actions,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt         *time.Time `db:"deleted_at" json:"-"`
}

// HygieneRepository handles hygiene plan and inspection persistence
type HygieneRepository struct {
	db *database.DB
}

// NewHygieneRepository creates a new hygiene repository
func NewHygieneRepository(db *database.DB) *HygieneRepository {
	return &HygieneRepository{db: db}
}

// --- Hygiene Plans ---

// CreatePlan creates a new hygiene plan
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *HygieneRepository) CreatePlan(ctx context.Context, plan *HygienePlan) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if plan.ID == "" {
		plan.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO hygiene_plans (
				id, tenant_id, title, version, category, content,
				effective_from, effective_until, approved_by, approved_by_name,
				approved_at, status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			plan.ID, tenantID, plan.Title, plan.Version, plan.Category, plan.Content,
			plan.EffectiveFrom, plan.EffectiveUntil, plan.ApprovedBy, plan.ApprovedByName,
			plan.ApprovedAt, plan.Status,
		).Scan(&plan.CreatedAt, &plan.UpdatedAt)
	})
}

// GetPlan gets a hygiene plan by ID
// TENANT-ISOLATED: Queries via RLS
func (r *HygieneRepository) GetPlan(ctx context.Context, id string) (*HygienePlan, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var plan HygienePlan
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, title, version, category, content,
			       effective_from, effective_until, approved_by, approved_by_name,
			       approved_at, status, created_at, updated_at
			FROM hygiene_plans WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &plan, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("hygiene_plan")
	}
	if err != nil {
		return nil, err
	}

	return &plan, nil
}

// ListPlans lists hygiene plans with pagination and optional filters
// TENANT-ISOLATED: Returns only plans via RLS
func (r *HygieneRepository) ListPlans(ctx context.Context, status, category string, page, perPage int) ([]*HygienePlan, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var plans []*HygienePlan

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		// Build dynamic WHERE clause
		where := "deleted_at IS NULL"
		args := []interface{}{}
		argIdx := 1

		if status != "" {
			where += " AND status = $" + itoa(argIdx)
			args = append(args, status)
			argIdx++
		}
		if category != "" {
			where += " AND category = $" + itoa(argIdx)
			args = append(args, category)
			argIdx++
		}

		countQuery := "SELECT COUNT(*) FROM hygiene_plans WHERE " + where
		if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
			return err
		}

		query := `
			SELECT id, title, version, category, content,
			       effective_from, effective_until, approved_by, approved_by_name,
			       approved_at, status, created_at, updated_at
			FROM hygiene_plans WHERE ` + where + `
			ORDER BY created_at DESC
			LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)

		offset := (page - 1) * perPage
		args = append(args, perPage, offset)
		return r.db.SelectContext(ctx, &plans, query, args...)
	})

	if err != nil {
		return nil, 0, err
	}

	return plans, total, nil
}

// UpdatePlan updates a hygiene plan
// TENANT-ISOLATED: Updates via RLS
func (r *HygieneRepository) UpdatePlan(ctx context.Context, plan *HygienePlan) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE hygiene_plans SET
				title = $2, version = $3, category = $4, content = $5,
				effective_from = $6, effective_until = $7, approved_by = $8,
				approved_by_name = $9, approved_at = $10, status = $11, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			plan.ID, plan.Title, plan.Version, plan.Category, plan.Content,
			plan.EffectiveFrom, plan.EffectiveUntil, plan.ApprovedBy, plan.ApprovedByName,
			plan.ApprovedAt, plan.Status,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("hygiene_plan")
		}

		return nil
	})
}

// DeletePlan soft-deletes a hygiene plan
// TENANT-ISOLATED: Deletes via RLS
func (r *HygieneRepository) DeletePlan(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE hygiene_plans SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("hygiene_plan")
		}

		return nil
	})
}

// --- Hygiene Inspections ---

// CreateInspection creates a new hygiene inspection
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *HygieneRepository) CreateInspection(ctx context.Context, inspection *HygieneInspection) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if inspection.ID == "" {
		inspection.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO hygiene_inspections (
				id, tenant_id, plan_id, inspection_date, inspector_name,
				area_inspected, checklist_results, overall_result, corrective_actions
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			inspection.ID, tenantID, inspection.PlanID, inspection.InspectionDate,
			inspection.InspectorName, inspection.AreaInspected, inspection.ChecklistResults,
			inspection.OverallResult, inspection.CorrectiveActions,
		).Scan(&inspection.CreatedAt, &inspection.UpdatedAt)
	})
}

// GetInspection gets a hygiene inspection by ID
// TENANT-ISOLATED: Queries via RLS
func (r *HygieneRepository) GetInspection(ctx context.Context, id string) (*HygieneInspection, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var inspection HygieneInspection
	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, plan_id, inspection_date, inspector_name, area_inspected,
			       checklist_results, overall_result, corrective_actions, created_at, updated_at
			FROM hygiene_inspections WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &inspection, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("hygiene_inspection")
	}
	if err != nil {
		return nil, err
	}

	return &inspection, nil
}

// ListInspections lists hygiene inspections with pagination and optional plan_id filter
// TENANT-ISOLATED: Returns only inspections via RLS
func (r *HygieneRepository) ListInspections(ctx context.Context, planID string, page, perPage int) ([]*HygieneInspection, int64, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	var inspections []*HygieneInspection

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		where := "deleted_at IS NULL"
		args := []interface{}{}
		argIdx := 1

		if planID != "" {
			where += " AND plan_id = $" + itoa(argIdx)
			args = append(args, planID)
			argIdx++
		}

		countQuery := "SELECT COUNT(*) FROM hygiene_inspections WHERE " + where
		if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
			return err
		}

		query := `
			SELECT id, plan_id, inspection_date, inspector_name, area_inspected,
			       checklist_results, overall_result, corrective_actions, created_at, updated_at
			FROM hygiene_inspections WHERE ` + where + `
			ORDER BY inspection_date DESC
			LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)

		offset := (page - 1) * perPage
		args = append(args, perPage, offset)
		return r.db.SelectContext(ctx, &inspections, query, args...)
	})

	if err != nil {
		return nil, 0, err
	}

	return inspections, total, nil
}

// UpdateInspection updates a hygiene inspection
// TENANT-ISOLATED: Updates via RLS
func (r *HygieneRepository) UpdateInspection(ctx context.Context, inspection *HygieneInspection) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE hygiene_inspections SET
				plan_id = $2, inspection_date = $3, inspector_name = $4,
				area_inspected = $5, checklist_results = $6, overall_result = $7,
				corrective_actions = $8, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			inspection.ID, inspection.PlanID, inspection.InspectionDate,
			inspection.InspectorName, inspection.AreaInspected, inspection.ChecklistResults,
			inspection.OverallResult, inspection.CorrectiveActions,
		)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("hygiene_inspection")
		}

		return nil
	})
}

// DeleteInspection soft-deletes a hygiene inspection
// TENANT-ISOLATED: Deletes via RLS
func (r *HygieneRepository) DeleteInspection(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE hygiene_inspections SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("hygiene_inspection")
		}

		return nil
	})
}

// itoa is a simple int-to-string helper for building query parameter indices
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
