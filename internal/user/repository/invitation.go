package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
)

// InvitationRepository handles invitation persistence
type InvitationRepository struct {
	db *database.DB
}

// NewInvitationRepository creates a new invitation repository
func NewInvitationRepository(db *database.DB) *InvitationRepository {
	return &InvitationRepository{db: db}
}

// generateSecureToken generates a cryptographically secure token
func generateSecureToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	token := base64.URLEncoding.EncodeToString(bytes)

	// Create hash of token for storage
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	return token, tokenHash, nil
}

// hashToken hashes a token for comparison
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Create creates a new invitation
func (r *InvitationRepository) Create(ctx context.Context, inv *domain.Invitation) (string, error) {
	if inv.ID == "" {
		inv.ID = uuid.New().String()
	}

	// Generate secure token
	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return "", errors.Internal("failed to generate token")
	}
	inv.Token = token
	inv.TokenHash = tokenHash

	query := `
		INSERT INTO user_invitations (id, email, token, token_hash, role_id, status, expires_at, created_by, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at
	`

	err = r.db.QueryRowxContext(ctx, query,
		inv.ID,
		inv.Email,
		token,
		tokenHash,
		inv.RoleID,
		inv.Status,
		inv.ExpiresAt,
		inv.CreatedBy,
		inv.Metadata,
	).Scan(&inv.CreatedAt)

	if err != nil {
		return "", err
	}

	return token, nil
}

// GetByID gets an invitation by ID
func (r *InvitationRepository) GetByID(ctx context.Context, id string) (*domain.Invitation, error) {
	var inv domain.Invitation
	query := `
		SELECT i.id, i.email, i.token, i.token_hash, i.role_id, i.status,
		       i.expires_at, i.accepted_at, i.accepted_user_id,
		       i.created_by, i.created_at, i.revoked_at, i.revoked_by, i.metadata
		FROM user_invitations i
		WHERE i.id = $1
	`

	if err := r.db.GetContext(ctx, &inv, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("invitation")
		}
		return nil, err
	}

	return &inv, nil
}

// GetByToken gets an invitation by token
func (r *InvitationRepository) GetByToken(ctx context.Context, token string) (*domain.Invitation, error) {
	tokenHash := hashToken(token)

	var inv domain.Invitation
	query := `
		SELECT i.id, i.email, i.token, i.token_hash, i.role_id, i.status,
		       i.expires_at, i.accepted_at, i.accepted_user_id,
		       i.created_by, i.created_at, i.revoked_at, i.revoked_by, i.metadata
		FROM user_invitations i
		WHERE i.token_hash = $1
	`

	if err := r.db.GetContext(ctx, &inv, query, tokenHash); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("invitation")
		}
		return nil, err
	}

	return &inv, nil
}

// GetByEmail gets a pending invitation by email
func (r *InvitationRepository) GetPendingByEmail(ctx context.Context, email string) (*domain.Invitation, error) {
	var inv domain.Invitation
	query := `
		SELECT i.id, i.email, i.token, i.token_hash, i.role_id, i.status,
		       i.expires_at, i.accepted_at, i.accepted_user_id,
		       i.created_by, i.created_at, i.revoked_at, i.revoked_by, i.metadata
		FROM user_invitations i
		WHERE i.email = $1 AND i.status = 'pending' AND i.expires_at > NOW()
	`

	if err := r.db.GetContext(ctx, &inv, query, email); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("invitation")
		}
		return nil, err
	}

	return &inv, nil
}

// GetWithRole gets an invitation with role information
func (r *InvitationRepository) GetWithRole(ctx context.Context, id string) (*domain.Invitation, error) {
	inv, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return r.loadRole(ctx, inv)
}

// GetByTokenWithRole gets an invitation by token with role information
func (r *InvitationRepository) GetByTokenWithRole(ctx context.Context, token string) (*domain.Invitation, error) {
	inv, err := r.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	return r.loadRole(ctx, inv)
}

// loadRole loads the role for an invitation
func (r *InvitationRepository) loadRole(ctx context.Context, inv *domain.Invitation) (*domain.Invitation, error) {
	var role domain.Role
	roleQuery := `
		SELECT id, name, display_name, display_name_de, description,
		       level::text::int as level, is_manager, can_receive_delegation, created_at, updated_at
		FROM roles
		WHERE id = $1
	`
	if err := r.db.GetContext(ctx, &role, roleQuery, inv.RoleID); err != nil {
		return nil, err
	}
	inv.Role = &role

	return inv, nil
}

// List lists invitations with pagination
func (r *InvitationRepository) List(ctx context.Context, page, perPage int, status string) ([]*domain.Invitation, int64, error) {
	var total int64
	countArgs := []interface{}{}
	countQuery := `SELECT COUNT(*) FROM user_invitations WHERE 1=1`

	if status != "" {
		countQuery += ` AND status = $1`
		countArgs = append(countArgs, status)
	}

	if err := r.db.GetContext(ctx, &total, countQuery, countArgs...); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	query := `
		SELECT i.id, i.email, i.role_id, i.status,
		       i.expires_at, i.accepted_at, i.created_by, i.created_at,
		       r.name as "role.name", r.display_name as "role.display_name",
		       r.display_name_de as "role.display_name_de"
		FROM user_invitations i
		JOIN roles r ON r.id = i.role_id
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 0

	if status != "" {
		argCount++
		query += ` AND i.status = $` + string(rune('0'+argCount))
		args = append(args, status)
	}

	argCount++
	query += ` ORDER BY i.created_at DESC LIMIT $` + string(rune('0'+argCount))
	args = append(args, perPage)

	argCount++
	query += ` OFFSET $` + string(rune('0'+argCount))
	args = append(args, offset)

	var invitations []*domain.Invitation
	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var inv domain.Invitation
		inv.Role = &domain.Role{}
		if err := rows.Scan(
			&inv.ID, &inv.Email, &inv.RoleID, &inv.Status,
			&inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedBy, &inv.CreatedAt,
			&inv.Role.Name, &inv.Role.DisplayName, &inv.Role.DisplayNameDE,
		); err != nil {
			return nil, 0, err
		}
		invitations = append(invitations, &inv)
	}

	return invitations, total, nil
}

// MarkAccepted marks an invitation as accepted
func (r *InvitationRepository) MarkAccepted(ctx context.Context, id, userID string) error {
	query := `
		UPDATE user_invitations
		SET status = 'accepted', accepted_at = NOW(), accepted_user_id = $2
		WHERE id = $1 AND status = 'pending'
	`

	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("invitation")
	}

	return nil
}

// MarkRevoked marks an invitation as revoked
func (r *InvitationRepository) MarkRevoked(ctx context.Context, id, revokedBy string) error {
	query := `
		UPDATE user_invitations
		SET status = 'revoked', revoked_at = NOW(), revoked_by = $2
		WHERE id = $1 AND status = 'pending'
	`

	result, err := r.db.ExecContext(ctx, query, id, revokedBy)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("invitation")
	}

	return nil
}

// MarkExpired marks expired invitations
func (r *InvitationRepository) MarkExpired(ctx context.Context) (int64, error) {
	query := `
		UPDATE user_invitations
		SET status = 'expired'
		WHERE status = 'pending' AND expires_at < NOW()
	`

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// RegenerateToken regenerates the token for a pending invitation
func (r *InvitationRepository) RegenerateToken(ctx context.Context, id string) (string, error) {
	// Generate new token
	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return "", errors.Internal("failed to generate token")
	}

	query := `
		UPDATE user_invitations
		SET token = $2, token_hash = $3
		WHERE id = $1 AND status = 'pending'
	`

	result, err := r.db.ExecContext(ctx, query, id, token, tokenHash)
	if err != nil {
		return "", err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return "", errors.NotFound("invitation")
	}

	return token, nil
}

// Delete permanently deletes an invitation
func (r *InvitationRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM user_invitations WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
