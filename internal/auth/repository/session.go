package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
)

// Session represents a user session
type Session struct {
	ID               string     `db:"id"`
	UserID           string     `db:"user_id"`
	RefreshTokenHash string     `db:"refresh_token_hash"`
	UserAgent        *string    `db:"user_agent"`
	IPAddress        *string    `db:"ip_address"`
	ExpiresAt        time.Time  `db:"expires_at"`
	CreatedAt        time.Time  `db:"created_at"`
	LastUsedAt       time.Time  `db:"last_used_at"`
	RevokedAt        *time.Time `db:"revoked_at"`
}

// SessionRepository handles session persistence
type SessionRepository struct {
	db *database.DB
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *database.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create creates a new session
func (r *SessionRepository) Create(ctx context.Context, userID string, refreshToken string, expiresAt time.Time, userAgent, ipAddress string) (*Session, error) {
	return r.CreateWithID(ctx, uuid.New().String(), userID, refreshToken, expiresAt, userAgent, ipAddress)
}

// CreateWithID creates a new session with a specific ID
func (r *SessionRepository) CreateWithID(ctx context.Context, id, userID string, refreshToken string, expiresAt time.Time, userAgent, ipAddress string) (*Session, error) {
	session := &Session{
		ID:               id,
		UserID:           userID,
		RefreshTokenHash: hashToken(refreshToken),
		UserAgent:        &userAgent,
		IPAddress:        &ipAddress,
		ExpiresAt:        expiresAt,
		CreatedAt:        time.Now(),
		LastUsedAt:       time.Now(),
	}

	query := `
		INSERT INTO sessions (id, user_id, refresh_token_hash, user_agent, ip_address, expires_at, created_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		session.ID,
		session.UserID,
		session.RefreshTokenHash,
		session.UserAgent,
		session.IPAddress,
		session.ExpiresAt,
		session.CreatedAt,
		session.LastUsedAt,
	)

	if err != nil {
		return nil, err
	}

	return session, nil
}

// GetByRefreshToken gets a session by refresh token
func (r *SessionRepository) GetByRefreshToken(ctx context.Context, refreshToken string) (*Session, error) {
	hash := hashToken(refreshToken)

	var session Session
	query := `
		SELECT id, user_id, refresh_token_hash, user_agent, ip_address, expires_at, created_at, last_used_at, revoked_at
		FROM sessions
		WHERE refresh_token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()
	`

	if err := r.db.GetContext(ctx, &session, query, hash); err != nil {
		return nil, err
	}

	return &session, nil
}

// GetByID gets a session by ID
func (r *SessionRepository) GetByID(ctx context.Context, id string) (*Session, error) {
	var session Session
	query := `
		SELECT id, user_id, refresh_token_hash, user_agent, ip_address, expires_at, created_at, last_used_at, revoked_at
		FROM sessions
		WHERE id = $1
	`

	if err := r.db.GetContext(ctx, &session, query, id); err != nil {
		return nil, err
	}

	return &session, nil
}

// UpdateLastUsed updates the last_used_at timestamp
func (r *SessionRepository) UpdateLastUsed(ctx context.Context, id string) error {
	query := `UPDATE sessions SET last_used_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// UpdateRefreshTokenHash updates the refresh token hash for a session (used during token rotation)
func (r *SessionRepository) UpdateRefreshTokenHash(ctx context.Context, id string, newRefreshToken string) error {
	newHash := hashToken(newRefreshToken)
	query := `UPDATE sessions SET refresh_token_hash = $1, last_used_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, newHash, id)
	return err
}

// Revoke revokes a session
func (r *SessionRepository) Revoke(ctx context.Context, id string) error {
	query := `UPDATE sessions SET revoked_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// RevokeByRefreshToken revokes a session by refresh token
func (r *SessionRepository) RevokeByRefreshToken(ctx context.Context, refreshToken string) error {
	hash := hashToken(refreshToken)
	query := `UPDATE sessions SET revoked_at = NOW() WHERE refresh_token_hash = $1`
	_, err := r.db.ExecContext(ctx, query, hash)
	return err
}

// RevokeAllForUser revokes all sessions for a user
func (r *SessionRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	query := `UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

// CleanExpired removes expired sessions
func (r *SessionRepository) CleanExpired(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < NOW() OR revoked_at IS NOT NULL`
	_, err := r.db.ExecContext(ctx, query)
	return err
}

// BlacklistToken adds a token to the blacklist
func (r *SessionRepository) BlacklistToken(ctx context.Context, jti, userID string, expiresAt time.Time) error {
	query := `
		INSERT INTO token_blacklist (token_jti, user_id, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (token_jti) DO NOTHING
	`
	_, err := r.db.ExecContext(ctx, query, jti, userID, expiresAt)
	return err
}

// IsTokenBlacklisted checks if a token is blacklisted
func (r *SessionRepository) IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM token_blacklist WHERE token_jti = $1 AND expires_at > NOW()`
	if err := r.db.GetContext(ctx, &count, query); err != nil {
		return false, err
	}
	return count > 0, nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
