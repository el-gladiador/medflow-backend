package testutil

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// UserFixture represents test user data
type UserFixture struct {
	ID           string
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
	Status       string
	RoleID       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RoleFixture represents test role data
type RoleFixture struct {
	ID          string
	Name        string
	DisplayName string
	Level       int
	IsManager   bool
	Permissions []string
}

// InventoryItemFixture represents test inventory item data
type InventoryItemFixture struct {
	ID            string
	Name          string
	SKU           string
	Category      string
	Description   string
	Unit          string
	MinQuantity   int
	ReorderPoint  int
	StorageRoomID *string
	CreatedAt     time.Time
}

// StorageRoomFixture represents test storage room data
type StorageRoomFixture struct {
	ID          string
	Name        string
	Description string
	Location    string
	IsActive    bool
	CreatedAt   time.Time
}

// EmployeeFixture represents test employee data
type EmployeeFixture struct {
	ID             string
	EmployeeNumber string
	FirstName      string
	LastName       string
	Email          string
	Position       string
	Department     string
	HireDate       time.Time
	Status         string
	CreatedAt      time.Time
}

// FixtureFactory creates test fixtures with sensible defaults
type FixtureFactory struct {
	sequence int
}

// NewFixtureFactory creates a new fixture factory
func NewFixtureFactory() *FixtureFactory {
	return &FixtureFactory{sequence: 0}
}

// nextSeq returns the next sequence number for unique values
func (f *FixtureFactory) nextSeq() int {
	f.sequence++
	return f.sequence
}

// User creates a user fixture with defaults
func (f *FixtureFactory) User(opts ...func(*UserFixture)) UserFixture {
	seq := f.nextSeq()
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)

	user := UserFixture{
		ID:           uuid.New().String(),
		Email:        fmt.Sprintf("user%d@test.medflow.de", seq),
		PasswordHash: string(hash),
		FirstName:    fmt.Sprintf("Test%d", seq),
		LastName:     "User",
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	for _, opt := range opts {
		opt(&user)
	}

	return user
}

// WithEmail sets the user email
func WithEmail(email string) func(*UserFixture) {
	return func(u *UserFixture) {
		u.Email = email
	}
}

// WithName sets the user's first and last name
func WithName(first, last string) func(*UserFixture) {
	return func(u *UserFixture) {
		u.FirstName = first
		u.LastName = last
	}
}

// WithStatus sets the user status
func WithStatus(status string) func(*UserFixture) {
	return func(u *UserFixture) {
		u.Status = status
	}
}

// WithPassword sets the user password (hashed)
func WithPassword(password string) func(*UserFixture) {
	return func(u *UserFixture) {
		hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		u.PasswordHash = string(hash)
	}
}

// WithRoleID sets the user's role ID
func WithRoleID(roleID string) func(*UserFixture) {
	return func(u *UserFixture) {
		u.RoleID = roleID
	}
}

// Role creates a role fixture with defaults
func (f *FixtureFactory) Role(opts ...func(*RoleFixture)) RoleFixture {
	seq := f.nextSeq()

	role := RoleFixture{
		ID:          uuid.New().String(),
		Name:        fmt.Sprintf("role_%d", seq),
		DisplayName: fmt.Sprintf("Role %d", seq),
		Level:       50,
		IsManager:   false,
		Permissions: []string{"read"},
	}

	for _, opt := range opts {
		opt(&role)
	}

	return role
}

// AdminRole creates an admin role fixture
func (f *FixtureFactory) AdminRole() RoleFixture {
	return RoleFixture{
		ID:          uuid.New().String(),
		Name:        "admin",
		DisplayName: "Administrator",
		Level:       100,
		IsManager:   true,
		Permissions: []string{"*"},
	}
}

// StaffRole creates a staff role fixture
func (f *FixtureFactory) StaffRole() RoleFixture {
	return RoleFixture{
		ID:          uuid.New().String(),
		Name:        "staff",
		DisplayName: "Staff",
		Level:       50,
		IsManager:   false,
		Permissions: []string{"inventory:read", "inventory:write"},
	}
}

// InventoryItem creates an inventory item fixture with defaults
func (f *FixtureFactory) InventoryItem(opts ...func(*InventoryItemFixture)) InventoryItemFixture {
	seq := f.nextSeq()

	item := InventoryItemFixture{
		ID:           uuid.New().String(),
		Name:         fmt.Sprintf("Test Item %d", seq),
		SKU:          fmt.Sprintf("SKU-%04d", seq),
		Category:     "Medical Supplies",
		Description:  "Test inventory item",
		Unit:         "piece",
		MinQuantity:  10,
		ReorderPoint: 20,
		CreatedAt:    time.Now(),
	}

	for _, opt := range opts {
		opt(&item)
	}

	return item
}

// WithItemName sets the inventory item name
func WithItemName(name string) func(*InventoryItemFixture) {
	return func(i *InventoryItemFixture) {
		i.Name = name
	}
}

// WithSKU sets the inventory item SKU
func WithSKU(sku string) func(*InventoryItemFixture) {
	return func(i *InventoryItemFixture) {
		i.SKU = sku
	}
}

// WithCategory sets the inventory item category
func WithCategory(category string) func(*InventoryItemFixture) {
	return func(i *InventoryItemFixture) {
		i.Category = category
	}
}

// WithStorageRoom sets the inventory item storage room
func WithStorageRoom(roomID string) func(*InventoryItemFixture) {
	return func(i *InventoryItemFixture) {
		i.StorageRoomID = &roomID
	}
}

// StorageRoom creates a storage room fixture with defaults
func (f *FixtureFactory) StorageRoom(opts ...func(*StorageRoomFixture)) StorageRoomFixture {
	seq := f.nextSeq()

	room := StorageRoomFixture{
		ID:          uuid.New().String(),
		Name:        fmt.Sprintf("Storage Room %d", seq),
		Description: "Test storage room",
		Location:    "Building A",
		IsActive:    true,
		CreatedAt:   time.Now(),
	}

	for _, opt := range opts {
		opt(&room)
	}

	return room
}

// WithRoomName sets the storage room name
func WithRoomName(name string) func(*StorageRoomFixture) {
	return func(r *StorageRoomFixture) {
		r.Name = name
	}
}

// WithLocation sets the storage room location
func WithLocation(location string) func(*StorageRoomFixture) {
	return func(r *StorageRoomFixture) {
		r.Location = location
	}
}

// Employee creates an employee fixture with defaults
func (f *FixtureFactory) Employee(opts ...func(*EmployeeFixture)) EmployeeFixture {
	seq := f.nextSeq()

	emp := EmployeeFixture{
		ID:             uuid.New().String(),
		EmployeeNumber: fmt.Sprintf("EMP-%04d", seq),
		FirstName:      fmt.Sprintf("Employee%d", seq),
		LastName:       "Test",
		Email:          fmt.Sprintf("employee%d@test.medflow.de", seq),
		Position:       "Staff",
		Department:     "General",
		HireDate:       time.Now().AddDate(-1, 0, 0),
		Status:         "active",
		CreatedAt:      time.Now(),
	}

	for _, opt := range opts {
		opt(&emp)
	}

	return emp
}

// WithEmployeeName sets the employee's first and last name
func WithEmployeeName(first, last string) func(*EmployeeFixture) {
	return func(e *EmployeeFixture) {
		e.FirstName = first
		e.LastName = last
	}
}

// WithPosition sets the employee's position
func WithPosition(position string) func(*EmployeeFixture) {
	return func(e *EmployeeFixture) {
		e.Position = position
	}
}

// WithDepartment sets the employee's department
func WithDepartment(department string) func(*EmployeeFixture) {
	return func(e *EmployeeFixture) {
		e.Department = department
	}
}

// WithEmployeeStatus sets the employee's status
func WithEmployeeStatus(status string) func(*EmployeeFixture) {
	return func(e *EmployeeFixture) {
		e.Status = status
	}
}

// DefaultTestUsers returns a set of standard test users
func DefaultTestUsers(factory *FixtureFactory) []UserFixture {
	return []UserFixture{
		factory.User(WithEmail("admin@praxis-mueller.de"), WithName("Max", "Mueller")),
		factory.User(WithEmail("staff@praxis-mueller.de"), WithName("Anna", "Schmidt")),
		factory.User(WithEmail("viewer@praxis-mueller.de"), WithName("Hans", "Weber")),
		factory.User(WithEmail("inactive@praxis-mueller.de"), WithName("Lisa", "Fischer"), WithStatus("inactive")),
	}
}

// DefaultTestRoles returns standard test roles
func DefaultTestRoles() []RoleFixture {
	return []RoleFixture{
		{ID: uuid.New().String(), Name: "admin", DisplayName: "Administrator", Level: 100, IsManager: true, Permissions: []string{"*"}},
		{ID: uuid.New().String(), Name: "manager", DisplayName: "Manager", Level: 80, IsManager: true, Permissions: []string{"users:read", "users:write", "inventory:read", "inventory:write"}},
		{ID: uuid.New().String(), Name: "staff", DisplayName: "Staff", Level: 50, IsManager: false, Permissions: []string{"inventory:read", "inventory:write"}},
		{ID: uuid.New().String(), Name: "viewer", DisplayName: "Viewer", Level: 10, IsManager: false, Permissions: []string{"inventory:read"}},
	}
}
