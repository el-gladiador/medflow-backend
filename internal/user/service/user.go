package service

import (
	"context"

	"golang.org/x/crypto/bcrypt"

	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/internal/user/events"
	"github.com/medflow/medflow-backend/internal/user/repository"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// UserService handles user business logic
type UserService struct {
	userRepo  *repository.UserRepository
	roleRepo  *repository.RoleRepository
	auditRepo *repository.AuditRepository
	publisher *events.UserEventPublisher
	logger    *logger.Logger
}

// NewUserService creates a new user service
func NewUserService(
	userRepo *repository.UserRepository,
	roleRepo *repository.RoleRepository,
	auditRepo *repository.AuditRepository,
	publisher *events.UserEventPublisher,
	log *logger.Logger,
) *UserService {
	return &UserService{
		userRepo:  userRepo,
		roleRepo:  roleRepo,
		auditRepo: auditRepo,
		publisher: publisher,
		logger:    log,
	}
}

// CreateUserRequest represents a create user request
type CreateUserRequest struct {
	Email    string  `json:"email" validate:"required,email"`
	Password string  `json:"password" validate:"required,min=6"`
	Name     string  `json:"name" validate:"required"`
	Avatar   *string `json:"avatar"`
	RoleName string  `json:"role" validate:"required"`
}

// UpdateUserRequest represents an update user request
type UpdateUserRequest struct {
	Email    *string `json:"email" validate:"omitempty,email"`
	Name     *string `json:"name"`
	Avatar   *string `json:"avatar"`
	IsActive *bool   `json:"is_active"`
}

// Create creates a new user
func (s *UserService) Create(ctx context.Context, req *CreateUserRequest, actorID, actorName string) (*domain.User, error) {
	// Check if email already exists
	existing, _ := s.userRepo.GetByEmail(ctx, req.Email)
	if existing != nil {
		return nil, errors.Conflict("email already in use")
	}

	// Get role
	role, err := s.roleRepo.GetByName(ctx, req.RoleName)
	if err != nil {
		return nil, errors.BadRequest("invalid role")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Internal("failed to hash password")
	}

	user := &domain.User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Name:         req.Name,
		Avatar:       req.Avatar,
		RoleID:       role.ID,
		IsActive:     true,
		CreatedBy:    &actorID,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, errors.Internal("failed to create user")
	}

	// Get full user with role
	user, err = s.userRepo.GetWithRole(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// Publish event
	s.publisher.PublishUserCreated(ctx, user)

	// Create audit log
	s.auditRepo.Create(ctx, &domain.AuditLog{
		ActorID:        &actorID,
		ActorName:      actorName,
		Action:         "create_user",
		TargetUserID:   &user.ID,
		TargetUserName: &user.Name,
		Details: map[string]interface{}{
			"email": user.Email,
			"role":  role.Name,
		},
	})

	return user, nil
}

// GetByID gets a user by ID
func (s *UserService) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return s.userRepo.GetWithRole(ctx, id)
}

// GetByEmail gets a user by email
func (s *UserService) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return s.userRepo.GetWithRole(ctx, user.ID)
}

// List lists users with pagination
func (s *UserService) List(ctx context.Context, page, perPage int) ([]*domain.User, int64, error) {
	return s.userRepo.List(ctx, page, perPage)
}

// Update updates a user
func (s *UserService) Update(ctx context.Context, id string, req *UpdateUserRequest, actorID, actorName string) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	changes := make(map[string]interface{})

	if req.Email != nil && *req.Email != user.Email {
		// Check if email already exists
		existing, _ := s.userRepo.GetByEmail(ctx, *req.Email)
		if existing != nil && existing.ID != id {
			return nil, errors.Conflict("email already in use")
		}
		changes["email"] = map[string]string{"from": user.Email, "to": *req.Email}
		user.Email = *req.Email
	}

	if req.Name != nil && *req.Name != user.Name {
		changes["name"] = map[string]string{"from": user.Name, "to": *req.Name}
		user.Name = *req.Name
	}

	if req.Avatar != nil {
		user.Avatar = req.Avatar
	}

	if req.IsActive != nil && *req.IsActive != user.IsActive {
		changes["is_active"] = map[string]bool{"from": user.IsActive, "to": *req.IsActive}
		user.IsActive = *req.IsActive
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	// Get updated user with role
	user, err = s.userRepo.GetWithRole(ctx, id)
	if err != nil {
		return nil, err
	}

	// Publish event
	s.publisher.PublishUserUpdated(ctx, user, changes)

	// Create audit log
	if len(changes) > 0 {
		s.auditRepo.Create(ctx, &domain.AuditLog{
			ActorID:        &actorID,
			ActorName:      actorName,
			Action:         "update_user",
			TargetUserID:   &user.ID,
			TargetUserName: &user.Name,
			Details:        changes,
		})
	}

	return user, nil
}

// Delete soft deletes a user
func (s *UserService) Delete(ctx context.Context, id, actorID, actorName string) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.userRepo.SoftDelete(ctx, id); err != nil {
		return err
	}

	// Publish event
	s.publisher.PublishUserDeleted(ctx, id)

	// Create audit log
	s.auditRepo.Create(ctx, &domain.AuditLog{
		ActorID:        &actorID,
		ActorName:      actorName,
		Action:         "delete_user",
		TargetUserID:   &id,
		TargetUserName: &user.Name,
	})

	return nil
}

// ChangeRole changes a user's role
func (s *UserService) ChangeRole(ctx context.Context, userID, roleName, actorID, actorName string) (*domain.User, error) {
	user, err := s.userRepo.GetWithRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	oldRoleName := user.Role.Name

	// Get new role
	newRole, err := s.roleRepo.GetByName(ctx, roleName)
	if err != nil {
		return nil, errors.BadRequest("invalid role")
	}

	user.RoleID = newRole.ID
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	// Get updated user with role
	user, err = s.userRepo.GetWithRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Publish event
	s.publisher.PublishUserRoleChanged(ctx, userID, oldRoleName, roleName)

	// Create audit log
	s.auditRepo.Create(ctx, &domain.AuditLog{
		ActorID:        &actorID,
		ActorName:      actorName,
		Action:         "change_role",
		TargetUserID:   &userID,
		TargetUserName: &user.Name,
		Details: map[string]interface{}{
			"old_role": oldRoleName,
			"new_role": roleName,
		},
	})

	return user, nil
}

// GrantPermission grants a permission override to a user
func (s *UserService) GrantPermission(ctx context.Context, userID, permission, reason, actorID, actorName string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Validate permission exists
	_, err = s.roleRepo.GetPermissionByName(ctx, permission)
	if err != nil {
		return errors.BadRequest("invalid permission")
	}

	override := &domain.PermissionOverride{
		UserID:     userID,
		Permission: permission,
		Granted:    true,
		GrantedBy:  actorID,
		Reason:     &reason,
	}

	if err := s.userRepo.AddPermissionOverride(ctx, override); err != nil {
		return err
	}

	// Create audit log
	s.auditRepo.Create(ctx, &domain.AuditLog{
		ActorID:        &actorID,
		ActorName:      actorName,
		Action:         "grant_permission",
		TargetUserID:   &userID,
		TargetUserName: &user.Name,
		Details: map[string]interface{}{
			"permission": permission,
			"reason":     reason,
		},
	})

	return nil
}

// RevokePermission revokes a permission override from a user
func (s *UserService) RevokePermission(ctx context.Context, userID, permission, reason, actorID, actorName string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := s.userRepo.RemovePermissionOverride(ctx, userID, permission); err != nil {
		return err
	}

	// Create audit log
	s.auditRepo.Create(ctx, &domain.AuditLog{
		ActorID:        &actorID,
		ActorName:      actorName,
		Action:         "revoke_permission",
		TargetUserID:   &userID,
		TargetUserName: &user.Name,
		Details: map[string]interface{}{
			"permission": permission,
			"reason":     reason,
		},
	})

	return nil
}

// GrantAccessGiver grants access giver status to a user
func (s *UserService) GrantAccessGiver(ctx context.Context, userID string, scope []string, actorID, actorName string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	user.IsAccessGiver = true
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	if err := s.userRepo.SetAccessGiverScope(ctx, userID, scope); err != nil {
		return err
	}

	// Create audit log
	s.auditRepo.Create(ctx, &domain.AuditLog{
		ActorID:        &actorID,
		ActorName:      actorName,
		Action:         "grant_access_giver",
		TargetUserID:   &userID,
		TargetUserName: &user.Name,
		Details: map[string]interface{}{
			"scope": scope,
		},
	})

	return nil
}

// RevokeAccessGiver revokes access giver status from a user
func (s *UserService) RevokeAccessGiver(ctx context.Context, userID, actorID, actorName string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	user.IsAccessGiver = false
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	if err := s.userRepo.ClearAccessGiverScope(ctx, userID); err != nil {
		return err
	}

	// Create audit log
	s.auditRepo.Create(ctx, &domain.AuditLog{
		ActorID:        &actorID,
		ActorName:      actorName,
		Action:         "revoke_access_giver",
		TargetUserID:   &userID,
		TargetUserName: &user.Name,
	})

	return nil
}

// ValidateCredentials validates user credentials
func (s *UserService) ValidateCredentials(ctx context.Context, email, password string) (*domain.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, errors.InvalidCredentials()
	}

	if !user.IsActive {
		return nil, errors.InvalidCredentials()
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.InvalidCredentials()
	}

	// Update last login
	s.userRepo.UpdateLastLogin(ctx, user.ID)

	return s.userRepo.GetWithRole(ctx, user.ID)
}
