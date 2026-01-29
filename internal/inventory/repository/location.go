package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
)

// StorageRoom represents a storage room
type StorageRoom struct {
	ID          string    `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Code        string    `db:"code" json:"code"`
	Description *string   `db:"description" json:"description,omitempty"`
	IsActive    bool      `db:"is_active" json:"is_active"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// StorageCabinet represents a storage cabinet
type StorageCabinet struct {
	ID                      string   `db:"id" json:"id"`
	RoomID                  string   `db:"room_id" json:"room_id"`
	Name                    string   `db:"name" json:"name"`
	Code                    *string  `db:"code" json:"code,omitempty"`
	IsTemperatureControlled bool     `db:"is_temperature_controlled" json:"is_temperature_controlled"`
	TemperatureMin          *float64 `db:"temperature_min" json:"temperature_min,omitempty"`
	TemperatureMax          *float64 `db:"temperature_max" json:"temperature_max,omitempty"`
	Description             *string  `db:"description" json:"description,omitempty"`
	IsActive                bool     `db:"is_active" json:"is_active"`
	CreatedAt               time.Time `db:"created_at" json:"created_at"`
	UpdatedAt               time.Time `db:"updated_at" json:"updated_at"`
}

// StorageShelf represents a storage shelf
type StorageShelf struct {
	ID            string    `db:"id" json:"id"`
	CabinetID     string    `db:"cabinet_id" json:"cabinet_id"`
	ParentShelfID *string   `db:"parent_shelf_id" json:"parent_shelf_id,omitempty"`
	Name          string    `db:"name" json:"name"`
	Code          *string   `db:"code" json:"code,omitempty"`
	Position      int       `db:"position" json:"position"`
	Capacity      *string   `db:"capacity" json:"capacity,omitempty"`
	IsActive      bool      `db:"is_active" json:"is_active"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
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

func (r *LocationRepository) CreateRoom(ctx context.Context, room *StorageRoom) error {
	if room.ID == "" {
		room.ID = uuid.New().String()
	}

	query := `
		INSERT INTO storage_rooms (id, name, code, description, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at
	`

	return r.db.QueryRowxContext(ctx, query,
		room.ID, room.Name, room.Code, room.Description, room.IsActive,
	).Scan(&room.CreatedAt, &room.UpdatedAt)
}

func (r *LocationRepository) GetRoom(ctx context.Context, id string) (*StorageRoom, error) {
	var room StorageRoom
	query := `SELECT * FROM storage_rooms WHERE id = $1`
	if err := r.db.GetContext(ctx, &room, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("room")
		}
		return nil, err
	}
	return &room, nil
}

func (r *LocationRepository) ListRooms(ctx context.Context) ([]*StorageRoom, error) {
	var rooms []*StorageRoom
	query := `SELECT * FROM storage_rooms WHERE is_active = true ORDER BY name`
	if err := r.db.SelectContext(ctx, &rooms, query); err != nil {
		return nil, err
	}
	return rooms, nil
}

func (r *LocationRepository) UpdateRoom(ctx context.Context, room *StorageRoom) error {
	query := `
		UPDATE storage_rooms SET name = $2, code = $3, description = $4, is_active = $5
		WHERE id = $1
	`
	result, err := r.db.ExecContext(ctx, query,
		room.ID, room.Name, room.Code, room.Description, room.IsActive,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("room")
	}
	return nil
}

func (r *LocationRepository) DeleteRoom(ctx context.Context, id string) error {
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
}

// Cabinet operations

func (r *LocationRepository) CreateCabinet(ctx context.Context, cabinet *StorageCabinet) error {
	if cabinet.ID == "" {
		cabinet.ID = uuid.New().String()
	}

	query := `
		INSERT INTO storage_cabinets (id, room_id, name, code, is_temperature_controlled, temperature_min, temperature_max, description, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`

	return r.db.QueryRowxContext(ctx, query,
		cabinet.ID, cabinet.RoomID, cabinet.Name, cabinet.Code,
		cabinet.IsTemperatureControlled, cabinet.TemperatureMin, cabinet.TemperatureMax,
		cabinet.Description, cabinet.IsActive,
	).Scan(&cabinet.CreatedAt, &cabinet.UpdatedAt)
}

func (r *LocationRepository) GetCabinet(ctx context.Context, id string) (*StorageCabinet, error) {
	var cabinet StorageCabinet
	query := `SELECT * FROM storage_cabinets WHERE id = $1`
	if err := r.db.GetContext(ctx, &cabinet, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("cabinet")
		}
		return nil, err
	}
	return &cabinet, nil
}

func (r *LocationRepository) ListCabinets(ctx context.Context, roomID string) ([]*StorageCabinet, error) {
	var cabinets []*StorageCabinet
	query := `SELECT * FROM storage_cabinets WHERE room_id = $1 AND is_active = true ORDER BY name`
	if err := r.db.SelectContext(ctx, &cabinets, query, roomID); err != nil {
		return nil, err
	}
	return cabinets, nil
}

func (r *LocationRepository) ListAllCabinets(ctx context.Context) ([]*StorageCabinet, error) {
	var cabinets []*StorageCabinet
	query := `SELECT * FROM storage_cabinets WHERE is_active = true ORDER BY name`
	if err := r.db.SelectContext(ctx, &cabinets, query); err != nil {
		return nil, err
	}
	return cabinets, nil
}

func (r *LocationRepository) UpdateCabinet(ctx context.Context, cabinet *StorageCabinet) error {
	query := `
		UPDATE storage_cabinets SET room_id = $2, name = $3, code = $4, is_temperature_controlled = $5,
		temperature_min = $6, temperature_max = $7, description = $8, is_active = $9
		WHERE id = $1
	`
	result, err := r.db.ExecContext(ctx, query,
		cabinet.ID, cabinet.RoomID, cabinet.Name, cabinet.Code,
		cabinet.IsTemperatureControlled, cabinet.TemperatureMin, cabinet.TemperatureMax,
		cabinet.Description, cabinet.IsActive,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("cabinet")
	}
	return nil
}

func (r *LocationRepository) DeleteCabinet(ctx context.Context, id string) error {
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
}

// Shelf operations

func (r *LocationRepository) CreateShelf(ctx context.Context, shelf *StorageShelf) error {
	if shelf.ID == "" {
		shelf.ID = uuid.New().String()
	}

	query := `
		INSERT INTO storage_shelves (id, cabinet_id, parent_shelf_id, name, code, position, capacity, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`

	return r.db.QueryRowxContext(ctx, query,
		shelf.ID, shelf.CabinetID, shelf.ParentShelfID, shelf.Name,
		shelf.Code, shelf.Position, shelf.Capacity, shelf.IsActive,
	).Scan(&shelf.CreatedAt, &shelf.UpdatedAt)
}

func (r *LocationRepository) GetShelf(ctx context.Context, id string) (*StorageShelf, error) {
	var shelf StorageShelf
	query := `SELECT * FROM storage_shelves WHERE id = $1`
	if err := r.db.GetContext(ctx, &shelf, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("shelf")
		}
		return nil, err
	}
	return &shelf, nil
}

func (r *LocationRepository) ListShelves(ctx context.Context, cabinetID string) ([]*StorageShelf, error) {
	var shelves []*StorageShelf
	query := `SELECT * FROM storage_shelves WHERE cabinet_id = $1 AND is_active = true ORDER BY position`
	if err := r.db.SelectContext(ctx, &shelves, query, cabinetID); err != nil {
		return nil, err
	}
	return shelves, nil
}

func (r *LocationRepository) ListAllShelves(ctx context.Context) ([]*StorageShelf, error) {
	var shelves []*StorageShelf
	query := `SELECT * FROM storage_shelves WHERE is_active = true ORDER BY position`
	if err := r.db.SelectContext(ctx, &shelves, query); err != nil {
		return nil, err
	}
	return shelves, nil
}

func (r *LocationRepository) UpdateShelf(ctx context.Context, shelf *StorageShelf) error {
	query := `
		UPDATE storage_shelves SET cabinet_id = $2, parent_shelf_id = $3, name = $4,
		code = $5, position = $6, capacity = $7, is_active = $8
		WHERE id = $1
	`
	result, err := r.db.ExecContext(ctx, query,
		shelf.ID, shelf.CabinetID, shelf.ParentShelfID, shelf.Name,
		shelf.Code, shelf.Position, shelf.Capacity, shelf.IsActive,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("shelf")
	}
	return nil
}

func (r *LocationRepository) DeleteShelf(ctx context.Context, id string) error {
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
}

// GetTree returns the full location hierarchy
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
