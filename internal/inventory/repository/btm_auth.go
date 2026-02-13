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

// BtmAuthorizedPerson represents a person authorized to handle BtM controlled substances
type BtmAuthorizedPerson struct {
	ID                string     `db:"id" json:"id"`
	UserID            string     `db:"user_id" json:"user_id"`
	UserName          string     `db:"user_name" json:"user_name"`
	AuthorizationType string     `db:"authorization_type" json:"authorization_type"` // full, dispense_only, view_only
	AuthorizedBy      *string    `db:"authorized_by" json:"authorized_by,omitempty"`
	AuthorizedByName  *string    `db:"authorized_by_name" json:"authorized_by_name,omitempty"`
	AuthorizedAt      time.Time  `db:"authorized_at" json:"authorized_at"`
	RevokedAt         *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
	RevokedBy         *string    `db:"revoked_by" json:"revoked_by,omitempty"`
	RevokedByName     *string    `db:"revoked_by_name" json:"revoked_by_name,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

// BtmAuthRepository handles BtM authorized personnel persistence
type BtmAuthRepository struct {
	db *database.DB
}

// NewBtmAuthRepository creates a new BtM auth repository
func NewBtmAuthRepository(db *database.DB) *BtmAuthRepository {
	return &BtmAuthRepository{db: db}
}

// Create creates a new BtM authorized person record
// TENANT-ISOLATED: Inserts with tenant_id for RLS
func (r *BtmAuthRepository) Create(ctx context.Context, person *BtmAuthorizedPerson) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if person.ID == "" {
		person.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO btm_authorized_personnel (
				id, tenant_id, user_id, user_name, authorization_type,
				authorized_by, authorized_by_name, authorized_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at, updated_at
		`

		return r.db.QueryRowxContext(ctx, query,
			person.ID, tenantID, person.UserID, person.UserName, person.AuthorizationType,
			person.AuthorizedBy, person.AuthorizedByName, person.AuthorizedAt,
		).Scan(&person.CreatedAt, &person.UpdatedAt)
	})
}

// List lists all active (non-revoked) BtM authorized personnel
// TENANT-ISOLATED: Returns only records via RLS
func (r *BtmAuthRepository) List(ctx context.Context) ([]*BtmAuthorizedPerson, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var persons []*BtmAuthorizedPerson

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, user_id, user_name, authorization_type,
			       authorized_by, authorized_by_name, authorized_at,
			       revoked_at, revoked_by, revoked_by_name,
			       created_at, updated_at
			FROM btm_authorized_personnel
			WHERE revoked_at IS NULL
			ORDER BY user_name
		`
		return r.db.SelectContext(ctx, &persons, query)
	})

	if err != nil {
		return nil, err
	}

	return persons, nil
}

// IsAuthorized checks if a user has sufficient authorization for BtM operations.
// Authorization hierarchy: full > dispense_only > view_only
// TENANT-ISOLATED: Queries via RLS
func (r *BtmAuthRepository) IsAuthorized(ctx context.Context, userID string, requiredType string) (bool, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return false, err
	}

	// Map authorization levels to numeric values for comparison
	authLevel := map[string]int{
		"view_only":     1,
		"dispense_only": 2,
		"full":          3,
	}

	requiredLevel, ok := authLevel[requiredType]
	if !ok {
		return false, errors.BadRequest("invalid authorization type: " + requiredType)
	}

	var authType sql.NullString

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT authorization_type
			FROM btm_authorized_personnel
			WHERE user_id = $1 AND revoked_at IS NULL
			ORDER BY CASE authorization_type
				WHEN 'full' THEN 3
				WHEN 'dispense_only' THEN 2
				WHEN 'view_only' THEN 1
				ELSE 0
			END DESC
			LIMIT 1
		`
		return r.db.GetContext(ctx, &authType, query, userID)
	})

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if !authType.Valid {
		return false, nil
	}

	userLevel, ok := authLevel[authType.String]
	if !ok {
		return false, nil
	}

	return userLevel >= requiredLevel, nil
}

// Revoke revokes a BtM authorization
// TENANT-ISOLATED: Updates via RLS
func (r *BtmAuthRepository) Revoke(ctx context.Context, id, revokedBy, revokedByName string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			UPDATE btm_authorized_personnel
			SET revoked_at = NOW(), revoked_by = $2, revoked_by_name = $3, updated_at = NOW()
			WHERE id = $1 AND revoked_at IS NULL
		`

		result, err := r.db.ExecContext(ctx, query, id, revokedBy, revokedByName)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("btm_authorized_person")
		}

		return nil
	})
}
