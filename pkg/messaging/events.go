package messaging

import (
	"encoding/json"
	"fmt"
	"time"
)

// Event types
const (
	// User events
	EventUserCreated           = "user.created"
	EventUserUpdated           = "user.updated"
	EventUserDeleted           = "user.deleted"
	EventUserRoleChanged       = "user.role.changed"
	EventUserPermissionChanged = "user.permission.changed"

	// Staff events
	EventEmployeeCreated = "staff.employee.created"
	EventEmployeeUpdated = "staff.employee.updated"
	EventEmployeeDeleted = "staff.employee.deleted"

	// Inventory events
	EventStockAdjusted   = "inventory.stock.adjusted"
	EventBatchExpiring   = "inventory.batch.expiring"
	EventAlertGenerated  = "inventory.alert.generated"

	// Audit events
	EventAuditLogCreated = "audit.log.created"
)

// Exchange names
const (
	ExchangeUserEvents      = "user.events"
	ExchangeStaffEvents     = "staff.events"
	ExchangeInventoryEvents = "inventory.events"
	ExchangeAuditEvents     = "audit.events"
)

// Event is the base event structure
type Event struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Source        string          `json:"source"`
	Timestamp     time.Time       `json:"timestamp"`
	CorrelationID string          `json:"correlation_id"`
	Data          json.RawMessage `json:"data"`
}

// NewEvent creates a new event with the given type and data
func NewEvent(eventType, source, correlationID string, data interface{}) (*Event, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &Event{
		ID:            GenerateEventID(),
		Type:          eventType,
		Source:        source,
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		Data:          dataBytes,
	}, nil
}

// UnmarshalData unmarshals the event data into the provided struct
func (e *Event) UnmarshalData(v interface{}) error {
	return json.Unmarshal(e.Data, v)
}

// User Events

// UserCreatedEvent is published when a user is created
type UserCreatedEvent struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	RoleName  string `json:"role_name"`

	// Tenant context (required for user-tenant lookup table)
	TenantID     string `json:"tenant_id"`
	TenantSlug   string `json:"tenant_slug"`
	TenantSchema string `json:"tenant_schema"`
}

// FullName returns the user's full name
func (e *UserCreatedEvent) FullName() string {
	return e.FirstName + " " + e.LastName
}

// UserUpdatedEvent is published when a user is updated
type UserUpdatedEvent struct {
	UserID string         `json:"user_id"`
	Fields map[string]any `json:"fields"` // Changed fields

	// Email change tracking (for updating user-tenant lookup table)
	OldEmail *string `json:"old_email,omitempty"`
	NewEmail *string `json:"new_email,omitempty"`

	// Tenant context (required for user-tenant lookup table)
	TenantID     string `json:"tenant_id"`
	TenantSlug   string `json:"tenant_slug"`
	TenantSchema string `json:"tenant_schema"`
}

// UserDeletedEvent is published when a user is deleted
type UserDeletedEvent struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"` // Required for removing from user-tenant lookup table

	// Tenant context (required for user-tenant lookup table)
	TenantID     string `json:"tenant_id"`
	TenantSlug   string `json:"tenant_slug"`
	TenantSchema string `json:"tenant_schema"`
}

// UserRoleChangedEvent is published when a user's role changes
type UserRoleChangedEvent struct {
	UserID      string `json:"user_id"`
	OldRoleName string `json:"old_role_name"`
	NewRoleName string `json:"new_role_name"`
}

// UserPermissionChangedEvent is published when a user's permissions change
type UserPermissionChangedEvent struct {
	UserID             string   `json:"user_id"`
	GrantedPermissions []string `json:"granted_permissions,omitempty"`
	RevokedPermissions []string `json:"revoked_permissions,omitempty"`
}

// Staff Events

// EmployeeCreatedEvent is published when an employee is created
type EmployeeCreatedEvent struct {
	EmployeeID string  `json:"employee_id"`
	UserID     *string `json:"user_id,omitempty"`
	Name       string  `json:"name"`
}

// EmployeeUpdatedEvent is published when an employee is updated
type EmployeeUpdatedEvent struct {
	EmployeeID string         `json:"employee_id"`
	Fields     map[string]any `json:"fields"`
}

// EmployeeDeletedEvent is published when an employee is deleted
type EmployeeDeletedEvent struct {
	EmployeeID string `json:"employee_id"`
}

// Inventory Events

// StockAdjustedEvent is published when stock is adjusted
type StockAdjustedEvent struct {
	ItemID      string `json:"item_id"`
	BatchID     string `json:"batch_id"`
	Adjustment  int    `json:"adjustment"`
	NewQuantity int    `json:"new_quantity"`
	PerformedBy string `json:"performed_by"`
	Reason      string `json:"reason"`
}

// BatchExpiringEvent is published when a batch is nearing expiry
type BatchExpiringEvent struct {
	ItemID     string    `json:"item_id"`
	BatchID    string    `json:"batch_id"`
	ItemName   string    `json:"item_name"`
	BatchNo    string    `json:"batch_no"`
	ExpiryDate time.Time `json:"expiry_date"`
	DaysUntil  int       `json:"days_until"`
	Quantity   int       `json:"quantity"`
}

// AlertGeneratedEvent is published when an alert is generated
type AlertGeneratedEvent struct {
	AlertID   string `json:"alert_id"`
	AlertType string `json:"alert_type"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	ItemID    string `json:"item_id,omitempty"`
	BatchID   string `json:"batch_id,omitempty"`
}

// Audit Events

// AuditLogCreatedEvent is published when an audit log entry is created
type AuditLogCreatedEvent struct {
	LogID       string         `json:"log_id"`
	UserID      string         `json:"user_id"`
	Action      string         `json:"action"`
	Resource    string         `json:"resource"`
	ResourceID  string         `json:"resource_id"`
	Changes     map[string]any `json:"changes,omitempty"`
	IPAddress   string         `json:"ip_address,omitempty"`
}

// GenerateEventID generates a unique event ID
func GenerateEventID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Nanosecond()%10000)
}
