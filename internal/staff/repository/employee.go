package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/errors"
)

// Employee represents an employee
type Employee struct {
	ID              string     `db:"id" json:"id"`
	UserID          *string    `db:"user_id" json:"user_id,omitempty"`
	Personalnummer  string     `db:"personalnummer" json:"personalnummer"`
	Vorname         string     `db:"vorname" json:"vorname"`
	Nachname        string     `db:"nachname" json:"nachname"`
	Profilbild      *string    `db:"profilbild" json:"profilbild,omitempty"`
	Geburtsdatum    time.Time  `db:"geburtsdatum" json:"geburtsdatum"`
	Geburtsort      *string    `db:"geburtsort" json:"geburtsort,omitempty"`
	Geschlecht      string     `db:"geschlecht" json:"geschlecht"`
	Nationalitaet   string     `db:"nationalitaet" json:"nationalitaet"`
	Familienstand   *string    `db:"familienstand" json:"familienstand,omitempty"`
	Rolle           string     `db:"rolle" json:"rolle"`
	Abteilung       *string    `db:"abteilung" json:"abteilung,omitempty"`
	Anstellungsart  string     `db:"anstellungsart" json:"anstellungsart"`
	Vertragsart     string     `db:"vertragsart" json:"vertragsart"`
	Eintrittsdatum  time.Time  `db:"eintrittsdatum" json:"eintrittsdatum"`
	Probezeitende   *time.Time `db:"probezeitende" json:"probezeitende,omitempty"`
	Befristungsende *time.Time `db:"befristungsende" json:"befristungsende,omitempty"`
	Wochenstunden   float64    `db:"wochenstunden" json:"wochenstunden"`
	Urlaubstage     int        `db:"urlaubstage" json:"urlaubstage"`
	Arbeitszeitmodell string   `db:"arbeitszeitmodell" json:"arbeitszeitmodell"`
	IsActive        bool       `db:"is_active" json:"is_active"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt       *time.Time `db:"deleted_at" json:"-"`
}

// EmployeeAddress represents an employee's address
type EmployeeAddress struct {
	ID         string    `db:"id" json:"id"`
	EmployeeID string    `db:"employee_id" json:"employee_id"`
	Strasse    string    `db:"strasse" json:"strasse"`
	Hausnummer string    `db:"hausnummer" json:"hausnummer"`
	PLZ        string    `db:"plz" json:"plz"`
	Ort        string    `db:"ort" json:"ort"`
	Land       string    `db:"land" json:"land"`
	Zusatz     *string   `db:"zusatz" json:"zusatz,omitempty"`
	IsPrimary  bool      `db:"is_primary" json:"is_primary"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// EmployeeContact represents employee contact information
type EmployeeContact struct {
	ID                       string  `db:"id" json:"id"`
	EmployeeID               string  `db:"employee_id" json:"employee_id"`
	EmailGeschaeftlich       string  `db:"email_geschaeftlich" json:"email_geschaeftlich"`
	EmailPrivat              *string `db:"email_privat" json:"email_privat,omitempty"`
	TelefonMobil             *string `db:"telefon_mobil" json:"telefon_mobil,omitempty"`
	TelefonFestnetz          *string `db:"telefon_festnetz" json:"telefon_festnetz,omitempty"`
	NotfallkontaktName       *string `db:"notfallkontakt_name" json:"notfallkontakt_name,omitempty"`
	NotfallkontaktBeziehung  *string `db:"notfallkontakt_beziehung" json:"notfallkontakt_beziehung,omitempty"`
	NotfallkontaktTelefon    *string `db:"notfallkontakt_telefon" json:"notfallkontakt_telefon,omitempty"`
}

// EmployeeFinancials represents employee financial data
type EmployeeFinancials struct {
	ID           string  `db:"id" json:"id"`
	EmployeeID   string  `db:"employee_id" json:"employee_id"`
	Kontoinhaber string  `db:"kontoinhaber" json:"kontoinhaber"`
	IBAN         string  `db:"iban" json:"iban"`
	BIC          *string `db:"bic" json:"bic,omitempty"`
	Bankname     *string `db:"bankname" json:"bankname,omitempty"`
	SteuerID     string  `db:"steuer_id" json:"steuer_id"`
	Steuerklasse string  `db:"steuerklasse" json:"steuerklasse"`
	Konfession   *string `db:"konfession" json:"konfession,omitempty"`
}

// EmployeeFile represents an uploaded file
type EmployeeFile struct {
	ID          string    `db:"id" json:"id"`
	EmployeeID  string    `db:"employee_id" json:"employee_id"`
	Name        string    `db:"name" json:"name"`
	FileType    string    `db:"file_type" json:"file_type"`
	FilePath    string    `db:"file_path" json:"file_path"`
	FileSize    *int      `db:"file_size" json:"file_size,omitempty"`
	MimeType    *string   `db:"mime_type" json:"mime_type,omitempty"`
	Category    *string   `db:"category" json:"category,omitempty"`
	UploadedAt  time.Time `db:"uploaded_at" json:"uploaded_at"`
	UploadedBy  *string   `db:"uploaded_by" json:"uploaded_by,omitempty"`
}

// EmployeeRepository handles employee persistence
type EmployeeRepository struct {
	db *database.DB
}

// NewEmployeeRepository creates a new employee repository
func NewEmployeeRepository(db *database.DB) *EmployeeRepository {
	return &EmployeeRepository{db: db}
}

// Create creates a new employee
func (r *EmployeeRepository) Create(ctx context.Context, emp *Employee) error {
	if emp.ID == "" {
		emp.ID = uuid.New().String()
	}

	query := `
		INSERT INTO employees (
			id, user_id, personalnummer, vorname, nachname, profilbild,
			geburtsdatum, geburtsort, geschlecht, nationalitaet, familienstand,
			rolle, abteilung, anstellungsart, vertragsart, eintrittsdatum,
			probezeitende, befristungsende, wochenstunden, urlaubstage, arbeitszeitmodell, is_active
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		) RETURNING created_at, updated_at
	`

	return r.db.QueryRowxContext(ctx, query,
		emp.ID, emp.UserID, emp.Personalnummer, emp.Vorname, emp.Nachname, emp.Profilbild,
		emp.Geburtsdatum, emp.Geburtsort, emp.Geschlecht, emp.Nationalitaet, emp.Familienstand,
		emp.Rolle, emp.Abteilung, emp.Anstellungsart, emp.Vertragsart, emp.Eintrittsdatum,
		emp.Probezeitende, emp.Befristungsende, emp.Wochenstunden, emp.Urlaubstage, emp.Arbeitszeitmodell, emp.IsActive,
	).Scan(&emp.CreatedAt, &emp.UpdatedAt)
}

// GetByID gets an employee by ID
func (r *EmployeeRepository) GetByID(ctx context.Context, id string) (*Employee, error) {
	var emp Employee
	query := `
		SELECT id, user_id, personalnummer, vorname, nachname, profilbild,
		       geburtsdatum, geburtsort, geschlecht, nationalitaet, familienstand,
		       rolle, abteilung, anstellungsart, vertragsart, eintrittsdatum,
		       probezeitende, befristungsende, wochenstunden, urlaubstage, arbeitszeitmodell,
		       is_active, created_at, updated_at
		FROM employees
		WHERE id = $1 AND deleted_at IS NULL
	`

	if err := r.db.GetContext(ctx, &emp, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("employee")
		}
		return nil, err
	}

	return &emp, nil
}

// List lists employees with pagination
func (r *EmployeeRepository) List(ctx context.Context, page, perPage int) ([]*Employee, int64, error) {
	var total int64
	countQuery := `SELECT COUNT(*) FROM employees WHERE deleted_at IS NULL`
	if err := r.db.GetContext(ctx, &total, countQuery); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	query := `
		SELECT id, user_id, personalnummer, vorname, nachname, profilbild,
		       geburtsdatum, geburtsort, geschlecht, nationalitaet, familienstand,
		       rolle, abteilung, anstellungsart, vertragsart, eintrittsdatum,
		       probezeitende, befristungsende, wochenstunden, urlaubstage, arbeitszeitmodell,
		       is_active, created_at, updated_at
		FROM employees
		WHERE deleted_at IS NULL
		ORDER BY nachname, vorname
		LIMIT $1 OFFSET $2
	`

	var employees []*Employee
	if err := r.db.SelectContext(ctx, &employees, query, perPage, offset); err != nil {
		return nil, 0, err
	}

	return employees, total, nil
}

// Update updates an employee
func (r *EmployeeRepository) Update(ctx context.Context, emp *Employee) error {
	query := `
		UPDATE employees SET
			user_id = $2, personalnummer = $3, vorname = $4, nachname = $5, profilbild = $6,
			geburtsdatum = $7, geburtsort = $8, geschlecht = $9, nationalitaet = $10, familienstand = $11,
			rolle = $12, abteilung = $13, anstellungsart = $14, vertragsart = $15, eintrittsdatum = $16,
			probezeitende = $17, befristungsende = $18, wochenstunden = $19, urlaubstage = $20, arbeitszeitmodell = $21,
			is_active = $22
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query,
		emp.ID, emp.UserID, emp.Personalnummer, emp.Vorname, emp.Nachname, emp.Profilbild,
		emp.Geburtsdatum, emp.Geburtsort, emp.Geschlecht, emp.Nationalitaet, emp.Familienstand,
		emp.Rolle, emp.Abteilung, emp.Anstellungsart, emp.Vertragsart, emp.Eintrittsdatum,
		emp.Probezeitende, emp.Befristungsende, emp.Wochenstunden, emp.Urlaubstage, emp.Arbeitszeitmodell,
		emp.IsActive,
	)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("employee")
	}

	return nil
}

// SoftDelete soft deletes an employee
func (r *EmployeeRepository) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE employees SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("employee")
	}

	return nil
}

// GetAddress gets an employee's address
func (r *EmployeeRepository) GetAddress(ctx context.Context, employeeID string) (*EmployeeAddress, error) {
	var addr EmployeeAddress
	query := `SELECT * FROM employee_addresses WHERE employee_id = $1 AND is_primary = true LIMIT 1`
	if err := r.db.GetContext(ctx, &addr, query, employeeID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &addr, nil
}

// SaveAddress saves an employee's address
func (r *EmployeeRepository) SaveAddress(ctx context.Context, addr *EmployeeAddress) error {
	if addr.ID == "" {
		addr.ID = uuid.New().String()
	}

	query := `
		INSERT INTO employee_addresses (id, employee_id, strasse, hausnummer, plz, ort, land, zusatz, is_primary)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (employee_id) WHERE is_primary = true
		DO UPDATE SET strasse = $3, hausnummer = $4, plz = $5, ort = $6, land = $7, zusatz = $8
	`

	_, err := r.db.ExecContext(ctx, query,
		addr.ID, addr.EmployeeID, addr.Strasse, addr.Hausnummer,
		addr.PLZ, addr.Ort, addr.Land, addr.Zusatz, addr.IsPrimary,
	)
	return err
}

// GetContact gets an employee's contact info
func (r *EmployeeRepository) GetContact(ctx context.Context, employeeID string) (*EmployeeContact, error) {
	var contact EmployeeContact
	query := `SELECT * FROM employee_contacts WHERE employee_id = $1 LIMIT 1`
	if err := r.db.GetContext(ctx, &contact, query, employeeID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &contact, nil
}

// SaveContact saves an employee's contact info
func (r *EmployeeRepository) SaveContact(ctx context.Context, contact *EmployeeContact) error {
	if contact.ID == "" {
		contact.ID = uuid.New().String()
	}

	query := `
		INSERT INTO employee_contacts (
			id, employee_id, email_geschaeftlich, email_privat, telefon_mobil, telefon_festnetz,
			notfallkontakt_name, notfallkontakt_beziehung, notfallkontakt_telefon
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (employee_id)
		DO UPDATE SET
			email_geschaeftlich = $3, email_privat = $4, telefon_mobil = $5, telefon_festnetz = $6,
			notfallkontakt_name = $7, notfallkontakt_beziehung = $8, notfallkontakt_telefon = $9
	`

	_, err := r.db.ExecContext(ctx, query,
		contact.ID, contact.EmployeeID, contact.EmailGeschaeftlich, contact.EmailPrivat,
		contact.TelefonMobil, contact.TelefonFestnetz, contact.NotfallkontaktName,
		contact.NotfallkontaktBeziehung, contact.NotfallkontaktTelefon,
	)
	return err
}

// GetFinancials gets an employee's financial data
func (r *EmployeeRepository) GetFinancials(ctx context.Context, employeeID string) (*EmployeeFinancials, error) {
	var fin EmployeeFinancials
	query := `SELECT * FROM employee_financials WHERE employee_id = $1 LIMIT 1`
	if err := r.db.GetContext(ctx, &fin, query, employeeID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &fin, nil
}

// SaveFinancials saves an employee's financial data
func (r *EmployeeRepository) SaveFinancials(ctx context.Context, fin *EmployeeFinancials) error {
	if fin.ID == "" {
		fin.ID = uuid.New().String()
	}

	query := `
		INSERT INTO employee_financials (
			id, employee_id, kontoinhaber, iban, bic, bankname, steuer_id, steuerklasse, konfession
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (employee_id)
		DO UPDATE SET
			kontoinhaber = $3, iban = $4, bic = $5, bankname = $6,
			steuer_id = $7, steuerklasse = $8, konfession = $9
	`

	_, err := r.db.ExecContext(ctx, query,
		fin.ID, fin.EmployeeID, fin.Kontoinhaber, fin.IBAN, fin.BIC,
		fin.Bankname, fin.SteuerID, fin.Steuerklasse, fin.Konfession,
	)
	return err
}

// ListFiles lists files for an employee
func (r *EmployeeRepository) ListFiles(ctx context.Context, employeeID string) ([]*EmployeeFile, error) {
	var files []*EmployeeFile
	query := `SELECT * FROM employee_files WHERE employee_id = $1 ORDER BY uploaded_at DESC`
	if err := r.db.SelectContext(ctx, &files, query, employeeID); err != nil {
		return nil, err
	}
	return files, nil
}

// CreateFile creates a file record
func (r *EmployeeRepository) CreateFile(ctx context.Context, file *EmployeeFile) error {
	if file.ID == "" {
		file.ID = uuid.New().String()
	}

	query := `
		INSERT INTO employee_files (id, employee_id, name, file_type, file_path, file_size, mime_type, category, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING uploaded_at
	`

	return r.db.QueryRowxContext(ctx, query,
		file.ID, file.EmployeeID, file.Name, file.FileType, file.FilePath,
		file.FileSize, file.MimeType, file.Category, file.UploadedBy,
	).Scan(&file.UploadedAt)
}

// DeleteFile deletes a file record
func (r *EmployeeRepository) DeleteFile(ctx context.Context, id string) error {
	query := `DELETE FROM employee_files WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.NotFound("file")
	}

	return nil
}

// NullifyUserReferences nullifies user_id for all employees referencing a deleted user
func (r *EmployeeRepository) NullifyUserReferences(ctx context.Context, userID string) error {
	query := `UPDATE employees SET user_id = NULL WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}
