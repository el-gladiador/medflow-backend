package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// StorageRoom represents a storage room
// Actual DB schema: name, description, floor, building, is_active
type StorageRoom struct {
	ID          string    `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	Floor       *string   `db:"floor" json:"floor,omitempty"`
	Building    *string   `db:"building" json:"building,omitempty"`
	IsActive    bool      `db:"is_active" json:"is_active"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// StorageCabinet represents a storage cabinet
// Actual DB schema: room_id, name, description, temperature_controlled, target_temperature_celsius, requires_key, is_active
type StorageCabinet struct {
	ID                      string   `db:"id" json:"id"`
	RoomID                  string   `db:"room_id" json:"room_id"`
	Name                    string   `db:"name" json:"name"`
	Description             *string  `db:"description" json:"description,omitempty"`
	TemperatureControlled   bool     `db:"temperature_controlled" json:"temperature_controlled"`
	TargetTemperature       *float64 `db:"target_temperature_celsius" json:"target_temperature_celsius,omitempty"`
	RequiresKey             bool     `db:"requires_key" json:"requires_key"`
	IsActive                bool     `db:"is_active" json:"is_active"`
	CreatedAt               time.Time `db:"created_at" json:"created_at"`
	UpdatedAt               time.Time `db:"updated_at" json:"updated_at"`
}

// StorageShelf represents a storage shelf
// Actual DB schema: cabinet_id, name, position
type StorageShelf struct {
	ID        string    `db:"id" json:"id"`
	CabinetID string    `db:"cabinet_id" json:"cabinet_id"`
	Name      string    `db:"name" json:"name"`
	Position  int       `db:"position" json:"position"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// LocationTree represents the full location hierarchy
type LocationTree struct {
	Rooms    []*RoomWithCabinets `json:"rooms"`
}

// RoomWithCabinets includes cabinets
type RoomWithCabinets struct {
	StorageRoom
	Cabinets []*CabinetWithShelves `json:"cabinets"`
}

// CabinetWithShelves includes shelves
type CabinetWithShelves struct {
	StorageCabinet
	Shelves []*StorageShelf `json:"shelves"`
}

// LocationRepository handles location persistence
type LocationRepository struct {
	db *database.DB
}

// NewLocationRepository creates a new location repository
func NewLocationRepository(db *database.DB) *LocationRepository {
	return &LocationRepository{db: db}
}

// Room operations

// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *LocationRepository) CreateRoom(ctx context.Context, room *StorageRoom) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if room.ID == "" {
		room.ID = uuid.New().String()
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO storage_rooms (id, name, description, floor, building, is_active)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			room.ID, room.Name, room.Description, room.Floor, room.Building, room.IsActive,
		).Scan(&room.CreatedAt, &room.UpdatedAt)
	})
}

// TENANT-ISOLATED: Queries only the tenant's schema
func (r *LocationRepository) GetRoom(ctx context.Context, id string) (*StorageRoom, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var room StorageRoom
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, name, description, floor, building, is_active, created_at, updated_at
			FROM storage_rooms WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &room, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("room")
	}
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// TENANT-ISOLATED: Returns only rooms from the tenant's schema
func (r *LocationRepository) ListRooms(ctx context.Context) ([]*StorageRoom, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var rooms []*StorageRoom
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, name, description, floor, building, is_active, created_at, updated_at
			FROM storage_rooms WHERE is_active = true AND deleted_at IS NULL ORDER BY name
		`
		return r.db.SelectContext(ctx, &rooms, query)
	})

	if err != nil {
		return nil, err
	}
	return rooms, nil
}

// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *LocationRepository) UpdateRoom(ctx context.Context, room *StorageRoom) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE storage_rooms SET name = $2, description = $3, floor = $4, building = $5, is_active = $6, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			room.ID, room.Name, room.Description, room.Floor, room.Building, room.IsActive,
		)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("room")
		}
		return nil
	})
}

// TENANT-ISOLATED: Deletes only from the tenant's schema
func (r *LocationRepository) DeleteRoom(ctx context.Context, id string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `DELETE FROM storage_rooms WHERE id = $1`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("room")
		}
		return nil
	})
}

// Cabinet operations

// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *LocationRepository) CreateCabinet(ctx context.Context, cabinet *StorageCabinet) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if cabinet.ID == "" {
		cabinet.ID = uuid.New().String()
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO storage_cabinets (id, room_id, name, description, temperature_controlled,
			       target_temperature_celsius, requires_key, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			cabinet.ID, cabinet.RoomID, cabinet.Name, cabinet.Description,
			cabinet.TemperatureControlled, cabinet.TargetTemperature, cabinet.RequiresKey, cabinet.IsActive,
		).Scan(&cabinet.CreatedAt, &cabinet.UpdatedAt)
	})
}

// TENANT-ISOLATED: Queries only the tenant's schema
func (r *LocationRepository) GetCabinet(ctx context.Context, id string) (*StorageCabinet, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var cabinet StorageCabinet
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, room_id, name, description, temperature_controlled,
			       target_temperature_celsius, requires_key, is_active, created_at, updated_at
			FROM storage_cabinets WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &cabinet, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("cabinet")
	}
	if err != nil {
		return nil, err
	}
	return &cabinet, nil
}

// TENANT-ISOLATED: Returns only cabinets from the tenant's schema
func (r *LocationRepository) ListCabinets(ctx context.Context, roomID string) ([]*StorageCabinet, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var cabinets []*StorageCabinet
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, room_id, name, description, temperature_controlled,
			       target_temperature_celsius, requires_key, is_active, created_at, updated_at
			FROM storage_cabinets WHERE room_id = $1 AND is_active = true AND deleted_at IS NULL ORDER BY name
		`
		return r.db.SelectContext(ctx, &cabinets, query, roomID)
	})

	if err != nil {
		return nil, err
	}
	return cabinets, nil
}

// TENANT-ISOLATED: Returns all active cabinets from the tenant's schema
func (r *LocationRepository) ListAllCabinets(ctx context.Context) ([]*StorageCabinet, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var cabinets []*StorageCabinet
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, room_id, name, description, temperature_controlled,
			       target_temperature_celsius, requires_key, is_active, created_at, updated_at
			FROM storage_cabinets WHERE is_active = true AND deleted_at IS NULL ORDER BY name
		`
		return r.db.SelectContext(ctx, &cabinets, query)
	})

	if err != nil {
		return nil, err
	}
	return cabinets, nil
}

// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *LocationRepository) UpdateCabinet(ctx context.Context, cabinet *StorageCabinet) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE storage_cabinets SET room_id = $2, name = $3, description = $4, temperature_controlled = $5,
			target_temperature_celsius = $6, requires_key = $7, is_active = $8, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			cabinet.ID, cabinet.RoomID, cabinet.Name, cabinet.Description,
			cabinet.TemperatureControlled, cabinet.TargetTemperature, cabinet.RequiresKey, cabinet.IsActive,
		)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("cabinet")
		}
		return nil
	})
}

// TENANT-ISOLATED: Deletes only from the tenant's schema
func (r *LocationRepository) DeleteCabinet(ctx context.Context, id string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `DELETE FROM storage_cabinets WHERE id = $1`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("cabinet")
		}
		return nil
	})
}

// Shelf operations

// TENANT-ISOLATED: Inserts into the tenant's schema
func (r *LocationRepository) CreateShelf(ctx context.Context, shelf *StorageShelf) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	if shelf.ID == "" {
		shelf.ID = uuid.New().String()
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			INSERT INTO storage_shelves (id, cabinet_id, name, position)
			VALUES ($1, $2, $3, $4)
			RETURNING created_at, updated_at
		`
		return r.db.QueryRowxContext(ctx, query,
			shelf.ID, shelf.CabinetID, shelf.Name, shelf.Position,
		).Scan(&shelf.CreatedAt, &shelf.UpdatedAt)
	})
}

// TENANT-ISOLATED: Queries only the tenant's schema
func (r *LocationRepository) GetShelf(ctx context.Context, id string) (*StorageShelf, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var shelf StorageShelf
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, cabinet_id, name, position, created_at, updated_at
			FROM storage_shelves WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &shelf, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("shelf")
	}
	if err != nil {
		return nil, err
	}
	return &shelf, nil
}

// TENANT-ISOLATED: Returns only shelves from the tenant's schema
func (r *LocationRepository) ListShelves(ctx context.Context, cabinetID string) ([]*StorageShelf, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var shelves []*StorageShelf
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, cabinet_id, name, position, created_at, updated_at
			FROM storage_shelves WHERE cabinet_id = $1 AND deleted_at IS NULL ORDER BY position
		`
		return r.db.SelectContext(ctx, &shelves, query, cabinetID)
	})

	if err != nil {
		return nil, err
	}
	return shelves, nil
}

// TENANT-ISOLATED: Returns all active shelves from the tenant's schema
func (r *LocationRepository) ListAllShelves(ctx context.Context) ([]*StorageShelf, error) {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return nil, err
	}

	var shelves []*StorageShelf
	err = r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			SELECT id, cabinet_id, name, position, created_at, updated_at
			FROM storage_shelves WHERE deleted_at IS NULL ORDER BY position
		`
		return r.db.SelectContext(ctx, &shelves, query)
	})

	if err != nil {
		return nil, err
	}
	return shelves, nil
}

// TENANT-ISOLATED: Updates only in the tenant's schema
func (r *LocationRepository) UpdateShelf(ctx context.Context, shelf *StorageShelf) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `
			UPDATE storage_shelves SET cabinet_id = $2, name = $3, position = $4, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		result, err := r.db.ExecContext(ctx, query,
			shelf.ID, shelf.CabinetID, shelf.Name, shelf.Position,
		)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("shelf")
		}
		return nil
	})
}

// TENANT-ISOLATED: Deletes only from the tenant's schema
func (r *LocationRepository) DeleteShelf(ctx context.Context, id string) error {
	tenantSchema, err := tenant.TenantSchema(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantSchema(ctx, tenantSchema, func(ctx context.Context) error {
		query := `DELETE FROM storage_shelves WHERE id = $1`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("shelf")
		}
		return nil
	})
}

// GetTree returns the full location hierarchy
// TENANT-ISOLATED: Returns tree built from tenant's schema (via tenant-scoped list methods)
func (r *LocationRepository) GetTree(ctx context.Context) (*LocationTree, error) {
	rooms, err := r.ListRooms(ctx)
	if err != nil {
		return nil, err
	}

	cabinets, err := r.ListAllCabinets(ctx)
	if err != nil {
		return nil, err
	}

	shelves, err := r.ListAllShelves(ctx)
	if err != nil {
		return nil, err
	}

	// Build cabinet map
	cabinetMap := make(map[string]*CabinetWithShelves)
	for _, c := range cabinets {
		cabinetMap[c.ID] = &CabinetWithShelves{
			StorageCabinet: *c,
			Shelves:        []*StorageShelf{},
		}
	}

	// Assign shelves to cabinets
	for _, s := range shelves {
		if cab, ok := cabinetMap[s.CabinetID]; ok {
			cab.Shelves = append(cab.Shelves, s)
		}
	}

	// Build room map
	roomsWithCabinets := make([]*RoomWithCabinets, len(rooms))
	for i, room := range rooms {
		roomsWithCabinets[i] = &RoomWithCabinets{
			StorageRoom: *room,
			Cabinets:    []*CabinetWithShelves{},
		}

		for _, cab := range cabinetMap {
			if cab.RoomID == room.ID {
				roomsWithCabinets[i].Cabinets = append(roomsWithCabinets[i].Cabinets, cab)
			}
		}
	}

	return &LocationTree{Rooms: roomsWithCabinets}, nil
}
