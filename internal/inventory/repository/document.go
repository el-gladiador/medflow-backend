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

// ItemDocument represents a document attached to an inventory item
type ItemDocument struct {
	ID            string    `db:"id" json:"id"`
	ItemID        string    `db:"item_id" json:"item_id"`
	DocumentType  string    `db:"document_type" json:"document_type"`
	FileName      string    `db:"file_name" json:"file_name"`
	FilePath      string    `db:"file_path" json:"-"`
	FileSizeBytes *int      `db:"file_size_bytes" json:"file_size_bytes,omitempty"`
	MimeType      *string   `db:"mime_type" json:"mime_type,omitempty"`
	UploadedAt    time.Time `db:"uploaded_at" json:"uploaded_at"`
	UploadedBy    *string   `db:"uploaded_by" json:"uploaded_by,omitempty"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
	DeletedAt     *time.Time `db:"deleted_at" json:"-"`
}

// DocumentRepository handles item document persistence
type DocumentRepository struct {
	db *database.DB
}

// NewDocumentRepository creates a new document repository
func NewDocumentRepository(db *database.DB) *DocumentRepository {
	return &DocumentRepository{db: db}
}

// Create creates a new item document record
func (r *DocumentRepository) Create(ctx context.Context, doc *ItemDocument) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			INSERT INTO item_documents (
				id, tenant_id, item_id, document_type, file_name, file_path,
				file_size_bytes, mime_type, uploaded_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING uploaded_at, created_at, updated_at
		`

		return r.db.QueryRowxContext(ctx, query,
			doc.ID, tenantID, doc.ItemID, doc.DocumentType, doc.FileName,
			doc.FilePath, doc.FileSizeBytes, doc.MimeType, doc.UploadedBy,
		).Scan(&doc.UploadedAt, &doc.CreatedAt, &doc.UpdatedAt)
	})
}

// GetByID gets a document by ID
func (r *DocumentRepository) GetByID(ctx context.Context, id string) (*ItemDocument, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var doc ItemDocument

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, document_type, file_name, file_path, file_size_bytes,
			       mime_type, uploaded_at, uploaded_by, created_at, updated_at
			FROM item_documents
			WHERE id = $1 AND deleted_at IS NULL
		`
		return r.db.GetContext(ctx, &doc, query, id)
	})

	if err == sql.ErrNoRows {
		return nil, errors.NotFound("document")
	}
	if err != nil {
		return nil, err
	}

	return &doc, nil
}

// ListByItem lists documents for an item
func (r *DocumentRepository) ListByItem(ctx context.Context, itemID string) ([]*ItemDocument, error) {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	var docs []*ItemDocument

	err = r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `
			SELECT id, item_id, document_type, file_name, file_path, file_size_bytes,
			       mime_type, uploaded_at, uploaded_by, created_at, updated_at
			FROM item_documents
			WHERE item_id = $1 AND deleted_at IS NULL
			ORDER BY uploaded_at DESC
		`
		return r.db.SelectContext(ctx, &docs, query, itemID)
	})

	if err != nil {
		return nil, err
	}

	return docs, nil
}

// Delete soft-deletes a document
func (r *DocumentRepository) Delete(ctx context.Context, id string) error {
	tenantID, err := tenant.TenantID(ctx)
	if err != nil {
		return err
	}

	return r.db.WithTenantRLS(ctx, tenantID, func(ctx context.Context) error {
		query := `UPDATE item_documents SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
		result, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return errors.NotFound("document")
		}

		return nil
	})
}
