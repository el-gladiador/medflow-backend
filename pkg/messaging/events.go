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
	EventEmployeeCreated            = "staff.employee.created"
	EventEmployeeUpdated            = "staff.employee.updated"
	EventEmployeeDeleted            = "staff.employee.deleted"
	EventEmployeeCredentialsAdded   = "staff.employee.credentials.added"
	EventEmployeeCredentialsRemoved = "staff.employee.credentials.removed"

	// Shift events
	EventShiftCreated = "staff.shift.created"
	EventShiftUpdated = "staff.shift.updated"
	EventShiftDeleted = "staff.shift.deleted"

	// Absence events
	EventAbsenceCreated  = "staff.absence.created"
	EventAbsenceUpdated  = "staff.absence.updated"
	EventAbsenceApproved = "staff.absence.approved"
	EventAbsenceRejected = "staff.absence.rejected"
	EventAbsenceDeleted  = "staff.absence.deleted"

	// Time tracking events
	EventTimeClockIn    = "staff.time.clock_in"
	EventTimeClockOut   = "staff.time.clock_out"
	EventTimeBreakStart = "staff.time.break_start"
	EventTimeBreakEnd   = "staff.time.break_end"

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
	UserID    string  `json:"user_id"`
	Email     string  `json:"email"`
	Username  *string `json:"username,omitempty"` // Optional username for subdomain login
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	RoleName  string  `json:"role_name"`

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

// EmployeeCredentialsAddedEvent is published when user credentials are added to an employee
type EmployeeCredentialsAddedEvent struct {
	EmployeeID   string `json:"employee_id"`
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
	RoleName     string `json:"role_name"`
	AddedBy      string `json:"added_by"` // Actor who added credentials
	TenantID     string `json:"tenant_id"`
	TenantSlug   string `json:"tenant_slug"`
	TenantSchema string `json:"tenant_schema"`
}

// EmployeeCredentialsRemovedEvent is published when user credentials are removed from an employee
type EmployeeCredentialsRemovedEvent struct {
	EmployeeID   string `json:"employee_id"`
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
	RemovedBy    string `json:"removed_by"` // Actor who removed credentials
	Reason       string `json:"reason,omitempty"`
	TenantID     string `json:"tenant_id"`
	TenantSlug   string `json:"tenant_slug"`
	TenantSchema string `json:"tenant_schema"`
}

// Shift Events

// ShiftCreatedEvent is published when a shift is created
type ShiftCreatedEvent struct {
	ShiftID    string    `json:"shift_id"`
	EmployeeID string    `json:"employee_id"`
	ShiftDate  time.Time `json:"shift_date"`
	StartTime  string    `json:"start_time"`
	EndTime    string    `json:"end_time"`
	ShiftType  string    `json:"shift_type"`
}

// ShiftUpdatedEvent is published when a shift is updated
type ShiftUpdatedEvent struct {
	ShiftID    string         `json:"shift_id"`
	EmployeeID string         `json:"employee_id"`
	Fields     map[string]any `json:"fields"`
}

// ShiftDeletedEvent is published when a shift is deleted
type ShiftDeletedEvent struct {
	ShiftID    string `json:"shift_id"`
	EmployeeID string `json:"employee_id"`
}

// Absence Events

// AbsenceCreatedEvent is published when an absence is created
type AbsenceCreatedEvent struct {
	AbsenceID   string    `json:"absence_id"`
	EmployeeID  string    `json:"employee_id"`
	AbsenceType string    `json:"absence_type"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	Status      string    `json:"status"`
}

// AbsenceUpdatedEvent is published when an absence is updated
type AbsenceUpdatedEvent struct {
	AbsenceID  string         `json:"absence_id"`
	EmployeeID string         `json:"employee_id"`
	Fields     map[string]any `json:"fields"`
}

// AbsenceApprovedEvent is published when an absence is approved
type AbsenceApprovedEvent struct {
	AbsenceID  string `json:"absence_id"`
	ReviewerID string `json:"reviewer_id"`
}

// AbsenceRejectedEvent is published when an absence is rejected
type AbsenceRejectedEvent struct {
	AbsenceID  string `json:"absence_id"`
	ReviewerID string `json:"reviewer_id"`
	Reason     string `json:"reason"`
}

// AbsenceDeletedEvent is published when an absence is deleted
type AbsenceDeletedEvent struct {
	AbsenceID string `json:"absence_id"`
}

// Time Tracking Events

// TimeClockInEvent is published when an employee clocks in
type TimeClockInEvent struct {
	TimeEntryID  string    `json:"time_entry_id"`
	EmployeeID   string    `json:"employee_id"`
	ClockIn      time.Time `json:"clock_in"`
	IsManual     bool      `json:"is_manual"`
}

// TimeClockOutEvent is published when an employee clocks out
type TimeClockOutEvent struct {
	TimeEntryID       string    `json:"time_entry_id"`
	EmployeeID        string    `json:"employee_id"`
	ClockIn           time.Time `json:"clock_in"`
	ClockOut          time.Time `json:"clock_out"`
	TotalWorkMinutes  int       `json:"total_work_minutes"`
	TotalBreakMinutes int       `json:"total_break_minutes"`
	IsManual          bool      `json:"is_manual"`
}

// TimeBreakStartEvent is published when an employee starts a break
type TimeBreakStartEvent struct {
	TimeBreakID string    `json:"time_break_id"`
	TimeEntryID string    `json:"time_entry_id"`
	EmployeeID  string    `json:"employee_id"`
	StartTime   time.Time `json:"start_time"`
}

// TimeBreakEndEvent is published when an employee ends a break
type TimeBreakEndEvent struct {
	TimeBreakID   string    `json:"time_break_id"`
	TimeEntryID   string    `json:"time_entry_id"`
	EmployeeID    string    `json:"employee_id"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	DurationMins  int       `json:"duration_mins"`
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
