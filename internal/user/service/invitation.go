package service

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/internal/user/events"
	"github.com/medflow/medflow-backend/internal/user/repository"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// Default invitation expiry
const DefaultInvitationExpiry = 72 * time.Hour // 3 days

// InvitationService handles invitation business logic
type InvitationService struct {
	inviteRepo *repository.InvitationRepository
	userRepo   *repository.UserRepository
	roleRepo   *repository.RoleRepository
	auditRepo  *repository.AuditRepository
	publisher  *events.UserEventPublisher
	logger     *logger.Logger
	baseURL    string
}

// NewInvitationService creates a new invitation service
func NewInvitationService(
	inviteRepo *repository.InvitationRepository,
	userRepo *repository.UserRepository,
	roleRepo *repository.RoleRepository,
	auditRepo *repository.AuditRepository,
	publisher *events.UserEventPublisher,
	log *logger.Logger,
	baseURL string,
) *InvitationService {
	return &InvitationService{
		inviteRepo: inviteRepo,
		userRepo:   userRepo,
		roleRepo:   roleRepo,
		auditRepo:  auditRepo,
		publisher:  publisher,
		logger:     log,
		baseURL:    baseURL,
	}
}

// CreateInvitationRequest represents a create invitation request
type CreateInvitationRequest struct {
	Email      string                 `json:"email" validate:"required,email"`
	RoleName   string                 `json:"role" validate:"required"`
	ExpiresIn  *time.Duration         `json:"expires_in,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// CreateInvitationResponse includes the invitation details and token
type CreateInvitationResponse struct {
	Invitation *domain.Invitation `json:"invitation"`
	InviteURL  string             `json:"invite_url"`
}

// Create creates a new invitation
func (s *InvitationService) Create(ctx context.Context, req *CreateInvitationRequest, actorID, actorName string) (*CreateInvitationResponse, error) {
	// Check if user already exists
	existingUser, _ := s.userRepo.GetByEmail(ctx, req.Email)
	if existingUser != nil {
		return nil, errors.Conflict("user with this email already exists")
	}

	// Check if there's already a pending invitation
	existingInvite, _ := s.inviteRepo.GetPendingByEmail(ctx, req.Email)
	if existingInvite != nil {
		return nil, errors.Conflict("pending invitation already exists for this email")
	}

	// Get role
	role, err := s.roleRepo.GetByName(ctx, req.RoleName)
	if err != nil {
		return nil, errors.BadRequest("invalid role")
	}

	// Calculate expiry
	expiry := DefaultInvitationExpiry
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		expiry = *req.ExpiresIn
	}

	inv := &domain.Invitation{
		Email:     req.Email,
		RoleID:    role.ID,
		Status:    domain.InvitationStatusPending,
		ExpiresAt: time.Now().Add(expiry),
		CreatedBy: &actorID,
		Metadata:  req.Metadata,
	}

	token, err := s.inviteRepo.Create(ctx, inv)
	if err != nil {
		return nil, errors.Internal("failed to create invitation")
	}

	// Load role for response
	inv.Role = role

	// Generate invite URL
	inviteURL := fmt.Sprintf("%s/invite/%s", s.baseURL, token)

	// Create audit log
	s.auditRepo.Create(ctx, &domain.AuditLog{
		ActorID:      &actorID,
		ActorName:    actorName,
		Action:       "create_invitation",
		ResourceType: stringPtr("invitation"),
		ResourceID:   &inv.ID,
		Details: map[string]interface{}{
			"email":      req.Email,
			"role":       req.RoleName,
			"expires_at": inv.ExpiresAt,
		},
	})

	s.logger.Info().
		Str("invitation_id", inv.ID).
		Str("email", req.Email).
		Str("role", req.RoleName).
		Msg("invitation created")

	return &CreateInvitationResponse{
		Invitation: inv,
		InviteURL:  inviteURL,
	}, nil
}

// GetByID gets an invitation by ID
func (s *InvitationService) GetByID(ctx context.Context, id string) (*domain.Invitation, error) {
	return s.inviteRepo.GetWithRole(ctx, id)
}

// GetByToken gets an invitation by token (public endpoint)
func (s *InvitationService) GetByToken(ctx context.Context, token string) (*domain.InvitationPublicInfo, error) {
	inv, err := s.inviteRepo.GetByTokenWithRole(ctx, token)
	if err != nil {
		return nil, err
	}

	// Update status if expired
	if inv.Status == domain.InvitationStatusPending && inv.IsExpired() {
		s.inviteRepo.MarkExpired(ctx)
		inv.Status = domain.InvitationStatusExpired
	}

	return inv.ToPublicInfo(), nil
}

// List lists invitations
func (s *InvitationService) List(ctx context.Context, page, perPage int, status string) ([]*domain.Invitation, int64, error) {
	// Mark expired invitations first
	s.inviteRepo.MarkExpired(ctx)

	return s.inviteRepo.List(ctx, page, perPage, status)
}

// AcceptInvitationRequest represents an accept invitation request
type AcceptInvitationRequest struct {
	Token    string `json:"token" validate:"required"`
	Name     string `json:"name" validate:"required"`
	Password string `json:"password" validate:"required,min=8"`
}

// AcceptInvitationResponse returns the created user
type AcceptInvitationResponse struct {
	User *domain.User `json:"user"`
}

// Accept accepts an invitation and creates the user
func (s *InvitationService) Accept(ctx context.Context, req *AcceptInvitationRequest) (*AcceptInvitationResponse, error) {
	// Get invitation
	inv, err := s.inviteRepo.GetByTokenWithRole(ctx, req.Token)
	if err != nil {
		return nil, errors.NotFound("invitation")
	}

	// Validate invitation status
	if !inv.IsPending() {
		if inv.Status == domain.InvitationStatusAccepted {
			return nil, errors.BadRequest("invitation has already been used")
		}
		if inv.Status == domain.InvitationStatusRevoked {
			return nil, errors.BadRequest("invitation has been revoked")
		}
		if inv.IsExpired() || inv.Status == domain.InvitationStatusExpired {
			return nil, errors.BadRequest("invitation has expired")
		}
		return nil, errors.BadRequest("invitation is no longer valid")
	}

	// Check if user already exists (in case of race condition)
	existingUser, _ := s.userRepo.GetByEmail(ctx, inv.Email)
	if existingUser != nil {
		// Mark invitation as accepted
		s.inviteRepo.MarkAccepted(ctx, inv.ID, existingUser.ID)
		return nil, errors.Conflict("user with this email already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Internal("failed to hash password")
	}

	// Create user
	user := &domain.User{
		Email:        inv.Email,
		PasswordHash: string(hashedPassword),
		Name:         req.Name,
		RoleID:       inv.RoleID,
		IsActive:     true,
		CreatedBy:    inv.CreatedBy,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, errors.Internal("failed to create user")
	}

	// Mark invitation as accepted
	if err := s.inviteRepo.MarkAccepted(ctx, inv.ID, user.ID); err != nil {
		s.logger.Warn().Err(err).Str("invitation_id", inv.ID).Msg("failed to mark invitation as accepted")
	}

	// Get full user with role
	user, err = s.userRepo.GetWithRole(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// Publish user created event
	s.publisher.PublishUserCreated(ctx, user)

	// Create audit log
	s.auditRepo.Create(ctx, &domain.AuditLog{
		ActorID:        &user.ID,
		ActorName:      user.Email,
		Action:         "accept_invitation",
		TargetUserID:   &user.ID,
		TargetUserName: &user.Name,
		ResourceType:   stringPtr("invitation"),
		ResourceID:     &inv.ID,
		Details: map[string]interface{}{
			"email":       inv.Email,
			"role":        inv.Role.Name,
			"invited_by":  inv.CreatedBy,
		},
	})

	s.logger.Info().
		Str("user_id", user.ID).
		Str("invitation_id", inv.ID).
		Str("email", inv.Email).
		Msg("invitation accepted, user created")

	return &AcceptInvitationResponse{
		User: user,
	}, nil
}

// Revoke revokes a pending invitation
func (s *InvitationService) Revoke(ctx context.Context, id, actorID, actorName string) error {
	inv, err := s.inviteRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if inv.Status != domain.InvitationStatusPending {
		return errors.BadRequest("can only revoke pending invitations")
	}

	if err := s.inviteRepo.MarkRevoked(ctx, id, actorID); err != nil {
		return err
	}

	// Create audit log
	s.auditRepo.Create(ctx, &domain.AuditLog{
		ActorID:      &actorID,
		ActorName:    actorName,
		Action:       "revoke_invitation",
		ResourceType: stringPtr("invitation"),
		ResourceID:   &id,
		Details: map[string]interface{}{
			"email": inv.Email,
		},
	})

	s.logger.Info().
		Str("invitation_id", id).
		Str("email", inv.Email).
		Str("revoked_by", actorID).
		Msg("invitation revoked")

	return nil
}

// ResendResponse includes the new invite URL
type ResendResponse struct {
	InviteURL string `json:"invite_url"`
}

// Resend regenerates the token and returns a new invite URL
func (s *InvitationService) Resend(ctx context.Context, id, actorID, actorName string) (*ResendResponse, error) {
	inv, err := s.inviteRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if inv.Status != domain.InvitationStatusPending {
		return nil, errors.BadRequest("can only resend pending invitations")
	}

	token, err := s.inviteRepo.RegenerateToken(ctx, id)
	if err != nil {
		return nil, err
	}

	inviteURL := fmt.Sprintf("%s/invite/%s", s.baseURL, token)

	// Create audit log
	s.auditRepo.Create(ctx, &domain.AuditLog{
		ActorID:      &actorID,
		ActorName:    actorName,
		Action:       "resend_invitation",
		ResourceType: stringPtr("invitation"),
		ResourceID:   &id,
		Details: map[string]interface{}{
			"email": inv.Email,
		},
	})

	s.logger.Info().
		Str("invitation_id", id).
		Str("email", inv.Email).
		Msg("invitation resent")

	return &ResendResponse{
		InviteURL: inviteURL,
	}, nil
}

// CleanupExpired marks all expired invitations
func (s *InvitationService) CleanupExpired(ctx context.Context) (int64, error) {
	return s.inviteRepo.MarkExpired(ctx)
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
