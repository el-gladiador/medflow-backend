package domain

import (
	"time"
)

// InvitationStatus represents the status of an invitation
type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusExpired  InvitationStatus = "expired"
	InvitationStatusRevoked  InvitationStatus = "revoked"
)

// Invitation represents a user invitation
type Invitation struct {
	ID             string                  `json:"id" db:"id"`
	Email          string                  `json:"email" db:"email"`
	Token          string                  `json:"-" db:"token"`
	TokenHash      string                  `json:"-" db:"token_hash"`
	RoleID         string                  `json:"-" db:"role_id"`
	Role           *Role                   `json:"role,omitempty" db:"-"`
	Status         InvitationStatus        `json:"status" db:"status"`
	ExpiresAt      time.Time               `json:"expires_at" db:"expires_at"`
	AcceptedAt     *time.Time              `json:"accepted_at,omitempty" db:"accepted_at"`
	AcceptedUserID *string                 `json:"accepted_user_id,omitempty" db:"accepted_user_id"`
	CreatedBy      *string                 `json:"created_by,omitempty" db:"created_by"`
	CreatedByUser  *User                   `json:"created_by_user,omitempty" db:"-"`
	CreatedAt      time.Time               `json:"created_at" db:"created_at"`
	RevokedAt      *time.Time              `json:"revoked_at,omitempty" db:"revoked_at"`
	RevokedBy      *string                 `json:"revoked_by,omitempty" db:"revoked_by"`
	Metadata       map[string]interface{}  `json:"metadata,omitempty" db:"metadata"`
}

// IsExpired checks if the invitation has expired
func (i *Invitation) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// IsPending checks if the invitation is still pending
func (i *Invitation) IsPending() bool {
	return i.Status == InvitationStatusPending && !i.IsExpired()
}

// InvitationResponse is the response returned to the frontend (without sensitive data)
type InvitationResponse struct {
	ID        string           `json:"id"`
	Email     string           `json:"email"`
	Role      string           `json:"role"`
	RoleDE    string           `json:"role_de"`
	Status    InvitationStatus `json:"status"`
	ExpiresAt time.Time        `json:"expires_at"`
	CreatedAt time.Time        `json:"created_at"`
}

// ToResponse converts an invitation to a response
func (i *Invitation) ToResponse() *InvitationResponse {
	response := &InvitationResponse{
		ID:        i.ID,
		Email:     i.Email,
		Status:    i.Status,
		ExpiresAt: i.ExpiresAt,
		CreatedAt: i.CreatedAt,
	}

	if i.Role != nil {
		response.Role = i.Role.DisplayName
		response.RoleDE = i.Role.DisplayNameDE
	}

	return response
}

// InvitationPublicInfo is the minimal info shown on the acceptance page
type InvitationPublicInfo struct {
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	RoleDE    string    `json:"role_de"`
	ExpiresAt time.Time `json:"expires_at"`
	IsValid   bool      `json:"is_valid"`
}

// ToPublicInfo returns minimal info for the acceptance page
func (i *Invitation) ToPublicInfo() *InvitationPublicInfo {
	info := &InvitationPublicInfo{
		Email:     i.Email,
		ExpiresAt: i.ExpiresAt,
		IsValid:   i.IsPending(),
	}

	if i.Role != nil {
		info.Role = i.Role.DisplayName
		info.RoleDE = i.Role.DisplayNameDE
	}

	return info
}
