package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// UserClient provides HTTP client for calling user service from staff service
type UserClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *logger.Logger
}

// NewUserClient creates a new user service client
func NewUserClient(baseURL string, log *logger.Logger) *UserClient {
	return &UserClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     log,
	}
}

// CreateUserRequest is the request structure for creating a user
type CreateUserRequest struct {
	Email     string  `json:"email"`
	Password  string  `json:"password"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Username  *string `json:"username,omitempty"` // Optional username for subdomain login
	RoleName  string  `json:"role"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

// User is the response structure from user service
type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Status    string `json:"status"`
	RoleName  string `json:"role_name,omitempty"`
	RoleLevel int    `json:"role_level,omitempty"`
}

// Role represents a role from the user service
type Role struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"` // Higher level = more permissions (admin=100, manager=50, staff=10)
}

// CreateUser creates a new user account in the user service
// CRITICAL: This must forward tenant headers to ensure user is created in correct tenant
func (c *UserClient) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	// Marshal request payload
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/users", bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// CRITICAL: Forward tenant headers - user must be created in same tenant as employee
	tenantID, _ := tenant.TenantID(ctx)
	tenantSlug, _ := tenant.TenantSlug(ctx)
	tenantSchema, _ := tenant.TenantSchema(ctx)

	if tenantID != "" {
		httpReq.Header.Set("X-Tenant-ID", tenantID)
	}
	if tenantSlug != "" {
		httpReq.Header.Set("X-Tenant-Slug", tenantSlug)
	}
	if tenantSchema != "" {
		httpReq.Header.Set("X-Tenant-Schema", tenantSchema)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.Info().
		Str("email", req.Email).
		Str("role", req.RoleName).
		Str("tenant_id", tenantID).
		Msg("creating user account via user service")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to call user service")
		return nil, fmt.Errorf("failed to call user service: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusCreated {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		c.logger.Error().
			Int("status", resp.StatusCode).
			Interface("error", errResp).
			Msg("user creation failed")
		return nil, fmt.Errorf("user creation failed with status %d: %v", resp.StatusCode, errResp)
	}

	// Decode response - user service wraps responses in {"success": true, "data": ...}
	var response struct {
		Success bool `json:"success"`
		Data    User `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Info().
		Str("user_id", response.Data.ID).
		Str("email", response.Data.Email).
		Msg("user account created successfully")

	return &response.Data, nil
}

// GetUserByID fetches a user by their ID (for role level validation)
// CRITICAL: This must forward tenant headers
func (c *UserClient) GetUserByID(ctx context.Context, userID string) (*User, error) {
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/users/"+userID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// CRITICAL: Forward tenant headers
	tenantID, _ := tenant.TenantID(ctx)
	tenantSlug, _ := tenant.TenantSlug(ctx)
	tenantSchema, _ := tenant.TenantSchema(ctx)

	if tenantID != "" {
		httpReq.Header.Set("X-Tenant-ID", tenantID)
	}
	if tenantSlug != "" {
		httpReq.Header.Set("X-Tenant-Slug", tenantSlug)
	}
	if tenantSchema != "" {
		httpReq.Header.Set("X-Tenant-Schema", tenantSchema)
	}

	c.logger.Debug().
		Str("user_id", userID).
		Str("tenant_id", tenantID).
		Msg("fetching user by ID")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call user service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found")
	}

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("get user failed with status %d: %v", resp.StatusCode, errResp)
	}

	// Decode response - user service wraps responses in {"success": true, "data": ...}
	var response struct {
		Success bool `json:"success"`
		Data    User `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response.Data, nil
}

// GetRole fetches role information by name (for role hierarchy validation)
// CRITICAL: This must forward tenant headers
func (c *UserClient) GetRole(ctx context.Context, roleName string) (*Role, error) {
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/roles/"+roleName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// CRITICAL: Forward tenant headers
	tenantID, _ := tenant.TenantID(ctx)
	tenantSlug, _ := tenant.TenantSlug(ctx)
	tenantSchema, _ := tenant.TenantSchema(ctx)

	if tenantID != "" {
		httpReq.Header.Set("X-Tenant-ID", tenantID)
	}
	if tenantSlug != "" {
		httpReq.Header.Set("X-Tenant-Slug", tenantSlug)
	}
	if tenantSchema != "" {
		httpReq.Header.Set("X-Tenant-Schema", tenantSchema)
	}

	c.logger.Debug().
		Str("role_name", roleName).
		Str("tenant_id", tenantID).
		Msg("fetching role by name")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call user service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("role not found: %s", roleName)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("get role failed with status %d: %v", resp.StatusCode, errResp)
	}

	// Decode response - user service wraps responses in {"success": true, "data": ...}
	var response struct {
		Success bool `json:"success"`
		Data    Role `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response.Data, nil
}

// SoftDeleteUser soft-deletes a user account (disables login, preserves audit trail)
// CRITICAL: This must forward tenant headers
func (c *UserClient) SoftDeleteUser(ctx context.Context, userID string) error {
	// Create HTTP request - using PATCH to soft-delete (set status to deleted)
	payload := []byte(`{"status":"deleted"}`)
	httpReq, err := http.NewRequestWithContext(ctx, "PATCH", c.baseURL+"/api/v1/users/"+userID+"/status", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// CRITICAL: Forward tenant headers
	tenantID, _ := tenant.TenantID(ctx)
	tenantSlug, _ := tenant.TenantSlug(ctx)
	tenantSchema, _ := tenant.TenantSchema(ctx)

	if tenantID != "" {
		httpReq.Header.Set("X-Tenant-ID", tenantID)
	}
	if tenantSlug != "" {
		httpReq.Header.Set("X-Tenant-Slug", tenantSlug)
	}
	if tenantSchema != "" {
		httpReq.Header.Set("X-Tenant-Schema", tenantSchema)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.Info().
		Str("user_id", userID).
		Str("tenant_id", tenantID).
		Msg("soft-deleting user account")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to soft-delete user")
		return fmt.Errorf("failed to soft-delete user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		c.logger.Error().Int("status", resp.StatusCode).Msg("user soft-delete failed")
		return fmt.Errorf("user soft-delete failed with status %d", resp.StatusCode)
	}

	c.logger.Info().Str("user_id", userID).Msg("user account soft-deleted")
	return nil
}

// DeleteUser deletes a user account (for rollback if employee creation fails)
func (c *UserClient) DeleteUser(ctx context.Context, userID string) error {
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/api/v1/users/"+userID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// CRITICAL: Forward tenant headers
	tenantID, _ := tenant.TenantID(ctx)
	tenantSlug, _ := tenant.TenantSlug(ctx)
	tenantSchema, _ := tenant.TenantSchema(ctx)

	if tenantID != "" {
		httpReq.Header.Set("X-Tenant-ID", tenantID)
	}
	if tenantSlug != "" {
		httpReq.Header.Set("X-Tenant-Slug", tenantSlug)
	}
	if tenantSchema != "" {
		httpReq.Header.Set("X-Tenant-Schema", tenantSchema)
	}

	c.logger.Warn().
		Str("user_id", userID).
		Str("tenant_id", tenantID).
		Msg("rolling back user account creation")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to delete user")
		return fmt.Errorf("failed to delete user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		c.logger.Error().Int("status", resp.StatusCode).Msg("user deletion failed")
		return fmt.Errorf("user deletion failed with status %d", resp.StatusCode)
	}

	c.logger.Info().Str("user_id", userID).Msg("user account deleted (rollback)")
	return nil
}
