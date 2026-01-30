package service

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/medflow/medflow-backend/internal/user/domain"
	"github.com/medflow/medflow-backend/internal/user/events"
	"github.com/medflow/medflow-backend/internal/user/repository"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/tenant"
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
	// Use GetUserWithRoleFromJunction to match actual database schema
	return s.userRepo.GetUserWithRoleFromJunction(ctx, id)
}

// GetByEmail gets a user by email
func (s *UserService) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	// Use GetUserWithRoleFromJunction to match actual database schema
	return s.userRepo.GetUserWithRoleFromJunction(ctx, user.ID)
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
	// HIERARCHY CHECK: Can manage current user?
	if err := s.canActorManageTarget(ctx, actorID, userID); err != nil {
		return nil, err
	}

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

	// NEW ROLE CHECK: Can assign this role?
	actor, err := s.userRepo.GetWithRole(ctx, actorID)
	if err != nil {
		return nil, errors.Internal("failed to get actor")
	}

	// Can only assign roles lower than your own (unless admin)
	if actor.Role != nil && actor.Role.Name != "admin" {
		if newRole.Level >= actor.Role.Level {
			return nil, errors.Forbidden("cannot assign role equal or higher than your own")
		}
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
	// HIERARCHY CHECK: Can actor manage target?
	if err := s.canActorManageTarget(ctx, actorID, userID); err != nil {
		return err
	}

	// ADMIN-ONLY CHECK: Is this a restricted permission?
	if isAdminOnlyPermission(permission) {
		actor, err := s.userRepo.GetWithRole(ctx, actorID)
		if err != nil {
			return errors.Internal("failed to get actor")
		}
		if actor.Role == nil || actor.Role.Name != "admin" {
			return errors.Forbidden("only admin can grant this permission")
		}
	}

	// PERMISSION CHECK: Can only grant permissions you have
	hasPermission, err := s.actorHasPermission(ctx, actorID, permission)
	if err != nil {
		return errors.Internal("failed to check actor permissions")
	}
	if !hasPermission {
		return errors.Forbidden("cannot grant permission you do not have")
	}

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
	// HIERARCHY CHECK: Can actor manage target?
	if err := s.canActorManageTarget(ctx, actorID, userID); err != nil {
		return err
	}

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
	// ADMIN ONLY: Only admin can grant access giver status
	actor, err := s.userRepo.GetWithRole(ctx, actorID)
	if err != nil {
		return errors.Internal("failed to get actor")
	}
	if actor.Role == nil || actor.Role.Name != "admin" {
		return errors.Forbidden("only admin can grant access giver status")
	}

	// Validate target is a manager (only managers can be access givers)
	target, err := s.userRepo.GetWithRole(ctx, userID)
	if err != nil {
		return err
	}
	if target.Role == nil {
		return errors.Internal("target role not found")
	}
	if !target.Role.IsManager || target.Role.Name == "admin" {
		return errors.BadRequest("only managers can be access givers")
	}

	// Validate scope only includes roles below the target's role
	for _, roleName := range scope {
		role, err := s.roleRepo.GetByName(ctx, roleName)
		if err != nil {
			return errors.BadRequest("invalid role in scope: " + roleName)
		}
		if role.Level >= target.Role.Level {
			return errors.BadRequest("invalid scope: can only manage roles lower than own role")
		}
	}

	target.IsAccessGiver = true
	if err := s.userRepo.Update(ctx, target); err != nil {
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
		TargetUserName: &target.Name,
		Details: map[string]interface{}{
			"scope": scope,
		},
	})

	return nil
}

// RevokeAccessGiver revokes access giver status from a user
func (s *UserService) RevokeAccessGiver(ctx context.Context, userID, actorID, actorName string) error {
	// ADMIN ONLY: Only admin can revoke access giver status
	actor, err := s.userRepo.GetWithRole(ctx, actorID)
	if err != nil {
		return errors.Internal("failed to get actor")
	}
	if actor.Role == nil || actor.Role.Name != "admin" {
		return errors.Forbidden("only admin can revoke access giver status")
	}

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

// ============================================================================
// HIERARCHY VALIDATION HELPERS
// ============================================================================

// adminOnlyPermissions lists permissions that only admin can grant
var adminOnlyPermissions = []string{
	"roles:manage",
	"users:delete",
	"access:delegate",
}

// contains checks if a string slice contains a value
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

// isAdminOnlyPermission checks if a permission requires admin
func isAdminOnlyPermission(permission string) bool {
	return contains(adminOnlyPermissions, permission)
}

// canActorManageTarget checks if actor can manage target user based on role hierarchy
func (s *UserService) canActorManageTarget(ctx context.Context, actorID, targetID string) error {
	// Cannot manage self
	if actorID == targetID {
		return errors.Forbidden("cannot modify own permissions")
	}

	actor, err := s.userRepo.GetWithRole(ctx, actorID)
	if err != nil {
		return errors.Internal("failed to get actor")
	}

	target, err := s.userRepo.GetWithRole(ctx, targetID)
	if err != nil {
		return errors.NotFound("target user not found")
	}

	// Admin can manage anyone
	if actor.Role != nil && actor.Role.Name == "admin" {
		return nil
	}

	// Check role hierarchy (actor level must be > target level)
	if actor.Role == nil || target.Role == nil {
		return errors.Internal("role information not available")
	}

	if actor.Role.Level <= target.Role.Level {
		return errors.Forbidden("insufficient privileges to manage this user")
	}

	// If actor is access giver, check scope
	if actor.IsAccessGiver {
		if !contains(actor.AccessGiverScope, target.Role.Name) {
			return errors.Forbidden("user not in access giver scope")
		}
		return nil
	}

	// Non-access givers need to be managers to manage users
	if !actor.Role.IsManager {
		return errors.Forbidden("not authorized to manage users")
	}

	return nil
}

// actorHasPermission checks if actor has a specific permission
func (s *UserService) actorHasPermission(ctx context.Context, actorID, permission string) (bool, error) {
	actor, err := s.userRepo.GetWithRole(ctx, actorID)
	if err != nil {
		return false, err
	}

	effectivePerms := actor.GetEffectivePermissions()
	return contains(effectivePerms, permission), nil
}

// ============================================================================
// AUTHENTICATION
// ============================================================================

// ValidateCredentials validates user credentials
func (s *UserService) ValidateCredentials(ctx context.Context, email, password string) (*domain.User, *repository.TenantInfo, error) {
	// Search across all tenant schemas to find which tenant owns this email
	user, tenantInfo, err := s.userRepo.FindUserAcrossTenants(ctx, email)
	if err != nil {
		return nil, nil, errors.InvalidCredentials()
	}

	if !user.IsActive {
		return nil, nil, errors.InvalidCredentials()
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, errors.InvalidCredentials()
	}

	// Now that we know the tenant, create tenant context and fetch role/permissions
	tenantCtx := tenant.WithTenantContext(ctx, tenantInfo.ID, tenantInfo.Slug, tenantInfo.SchemaName)

	// Fetch user with role and permissions from tenant's schema using user_roles junction table
	fullUser, err := s.userRepo.GetUserWithRoleFromJunction(tenantCtx, user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch user role: %w", err)
	}

	return fullUser, tenantInfo, nil
}
