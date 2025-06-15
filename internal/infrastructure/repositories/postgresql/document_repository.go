package postgresql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DocumentRepository struct {
	db *database.DB
}

func NewDocumentRepository(db *database.DB) repositories.DocumentRepository {
	return &DocumentRepository{db: db}
}

func (r *DocumentRepository) Create(ctx context.Context, document *models.Document) error {
	if err := r.db.WithContext(ctx).Create(document).Error; err != nil {
		return fmt.Errorf("failed to create document: %w", err)
	}
	return nil
}

func (r *DocumentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Document, error) {
	var document models.Document
	err := r.db.WithContext(ctx).
		Preload("Tenant").
		Preload("Folder").
		Preload("Creator").
		Preload("Tags").
		Preload("Categories").
		Where("id = ?", id).First(&document).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("document not found")
		}
		return nil, fmt.Errorf("failed to get document: %w", err)
	}
	return &document, nil
}

func (r *DocumentRepository) GetByContentHash(ctx context.Context, tenantID uuid.UUID, hash string) (*models.Document, error) {
	var document models.Document
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND content_hash = ?", tenantID, hash).
		First(&document).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("document not found")
		}
		return nil, fmt.Errorf("failed to get document by hash: %w", err)
	}
	return &document, nil
}

func (r *DocumentRepository) Update(ctx context.Context, document *models.Document) error {
	result := r.db.WithContext(ctx).Save(document)
	if result.Error != nil {
		return fmt.Errorf("failed to update document: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

func (r *DocumentRepository) List(ctx context.Context, tenantID uuid.UUID, filters repositories.DocumentFilters) ([]models.Document, int64, error) {
	var documents []models.Document
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Document{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	if filters.FolderID != nil {
		query = query.Where("folder_id = ?", *filters.FolderID)
	}

	if len(filters.Status) > 0 {
		query = query.Where("status IN ?", filters.Status)
	}

	if len(filters.DocumentType) > 0 {
		query = query.Where("document_type IN ?", filters.DocumentType)
	}

	if len(filters.CreatedBy) > 0 {
		query = query.Where("created_by IN ?", filters.CreatedBy)
	}

	if filters.DateFrom != nil {
		query = query.Where("created_at >= ?", *filters.DateFrom)
	}

	if filters.DateTo != nil {
		query = query.Where("created_at <= ?", *filters.DateTo)
	}

	if filters.MinSize != nil {
		query = query.Where("file_size >= ?", *filters.MinSize)
	}

	if filters.MaxSize != nil {
		query = query.Where("file_size <= ?", *filters.MaxSize)
	}

	if filters.Search != "" {
		searchTerm := "%" + filters.Search + "%"
		query = query.Where("title ILIKE ? OR file_name ILIKE ? OR extracted_text ILIKE ?",
			searchTerm, searchTerm, searchTerm)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	// Apply pagination and sorting
	offset := (filters.Page - 1) * filters.PageSize
	orderBy := "created_at DESC"
	if filters.SortBy != "" {
		direction := "ASC"
		if filters.SortDesc {
			direction = "DESC"
		}
		orderBy = fmt.Sprintf("%s %s", filters.SortBy, direction)
	}

	err := query.Preload("Creator").Preload("Folder").Preload("Tags").Preload("Categories").
		Order(orderBy).Offset(offset).Limit(filters.PageSize).Find(&documents).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list documents: %w", err)
	}

	return documents, total, nil
}

func (r *DocumentRepository) Search(ctx context.Context, tenantID uuid.UUID, query repositories.SearchQuery) ([]models.Document, error) {
	var documents []models.Document

	db := r.db.WithContext(ctx).Model(&models.Document{}).Where("tenant_id = ?", tenantID)

	if query.Query != "" {
		if query.Fuzzy {
			// Use PostgreSQL full-text search
			searchVector := "to_tsvector('english', coalesce(title, '') || ' ' || coalesce(extracted_text, '') || ' ' || coalesce(ocr_text, ''))"
			searchQuery := "plainto_tsquery('english', ?)"
			db = db.Where(fmt.Sprintf("%s @@ %s", searchVector, searchQuery), query.Query)
		} else {
			// Exact search
			searchTerm := "%" + query.Query + "%"
			db = db.Where("title ILIKE ? OR extracted_text ILIKE ? OR ocr_text ILIKE ?",
				searchTerm, searchTerm, searchTerm)
		}
	}

	if len(query.DocumentTypes) > 0 {
		db = db.Where("document_type IN ?", query.DocumentTypes)
	}

	if len(query.FolderIDs) > 0 {
		db = db.Where("folder_id IN ?", query.FolderIDs)
	}

	if query.DateFrom != nil {
		db = db.Where("created_at >= ?", *query.DateFrom)
	}

	if query.DateTo != nil {
		db = db.Where("created_at <= ?", *query.DateTo)
	}

	limit := query.Limit
	if limit == 0 {
		limit = 50
	}

	err := db.Preload("Creator").Preload("Folder").Preload("Tags").Preload("Categories").
		Order("created_at DESC").Limit(limit).Find(&documents).Error
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}

	return documents, nil
}

func (r *DocumentRepository) SemanticSearch(ctx context.Context, tenantID uuid.UUID, embedding []float32, limit int) ([]models.Document, error) {
	// This would use pgvector for semantic search
	// For now, fallback to regular search
	query := repositories.SearchQuery{
		Query: "",
		Limit: limit,
	}

	return r.Search(ctx, tenantID, query)
}

func (r *DocumentRepository) GetByFolder(ctx context.Context, folderID uuid.UUID, params repositories.ListParams) ([]models.Document, int64, error) {
	var documents []models.Document
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Document{}).Where("folder_id = ?", folderID)

	if params.Search != "" {
		searchTerm := "%" + params.Search + "%"
		query = query.Where("title ILIKE ? OR file_name ILIKE ?", searchTerm, searchTerm)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	// Apply pagination and sorting
	offset := (params.Page - 1) * params.PageSize
	orderBy := "created_at DESC"
	if params.SortBy != "" {
		direction := "ASC"
		if params.SortDesc {
			direction = "DESC"
		}
		orderBy = fmt.Sprintf("%s %s", params.SortBy, direction)
	}

	err := query.Preload("Creator").Preload("Tags").Preload("Categories").
		Order(orderBy).Offset(offset).Limit(params.PageSize).Find(&documents).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get documents by folder: %w", err)
	}

	return documents, total, nil
}

func (r *DocumentRepository) GetByTags(ctx context.Context, tenantID uuid.UUID, tagIDs []uuid.UUID) ([]models.Document, error) {
	var documents []models.Document

	err := r.db.WithContext(ctx).
		Joins("JOIN document_tags ON documents.id = document_tags.document_id").
		Where("documents.tenant_id = ? AND document_tags.tag_id IN ?", tenantID, tagIDs).
		Preload("Creator").Preload("Folder").Preload("Tags").Preload("Categories").
		Find(&documents).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get documents by tags: %w", err)
	}

	return documents, nil
}

func (r *DocumentRepository) GetByCategories(ctx context.Context, tenantID uuid.UUID, categoryIDs []uuid.UUID) ([]models.Document, error) {
	var documents []models.Document

	err := r.db.WithContext(ctx).
		Joins("JOIN document_categories ON documents.id = document_categories.document_id").
		Where("documents.tenant_id = ? AND document_categories.category_id IN ?", tenantID, categoryIDs).
		Preload("Creator").Preload("Folder").Preload("Tags").Preload("Categories").
		Find(&documents).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get documents by categories: %w", err)
	}

	return documents, nil
}

func (r *DocumentRepository) GetDuplicates(ctx context.Context, tenantID uuid.UUID, threshold float64) ([]repositories.DocumentDuplicate, error) {
	var duplicates []repositories.DocumentDuplicate

	// Simple implementation based on content hash
	// In production, you might want more sophisticated duplicate detection
	var results []struct {
		ContentHash string
		Documents   []uuid.UUID
	}

	err := r.db.WithContext(ctx).Raw(`
		SELECT content_hash, array_agg(id) as documents 
		FROM documents 
		WHERE tenant_id = ? 
		GROUP BY content_hash 
		HAVING count(*) > 1
	`, tenantID).Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to find duplicates: %w", err)
	}

	for _, result := range results {
		if len(result.Documents) > 1 {
			for i := 1; i < len(result.Documents); i++ {
				duplicates = append(duplicates, repositories.DocumentDuplicate{
					OriginalID:   result.Documents[0],
					DuplicateID:  result.Documents[i],
					Similarity:   1.0, // 100% for exact hash match
					ContentMatch: true,
				})
			}
		}
	}

	return duplicates, nil
}

func (r *DocumentRepository) GetExpiring(ctx context.Context, tenantID uuid.UUID, days int) ([]models.Document, error) {
	var documents []models.Document

	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND expiry_date IS NOT NULL AND expiry_date <= NOW() + INTERVAL '%d days'", tenantID, days).
		Preload("Creator").Preload("Folder").
		Order("expiry_date ASC").
		Find(&documents).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get expiring documents: %w", err)
	}

	return documents, nil
}

func (r *DocumentRepository) GetFinancialDocuments(ctx context.Context, tenantID uuid.UUID, filters repositories.FinancialFilters) ([]models.Document, error) {
	var documents []models.Document

	query := r.db.WithContext(ctx).Model(&models.Document{}).
		Where("tenant_id = ? AND document_type IN ?", tenantID, []models.DocumentType{
			models.DocTypeInvoice,
			models.DocTypeReceipt,
			models.DocTypeBankStatement,
			models.DocTypePayroll,
		})

	if filters.MinAmount != nil {
		query = query.Where("amount >= ?", *filters.MinAmount)
	}

	if filters.MaxAmount != nil {
		query = query.Where("amount <= ?", *filters.MaxAmount)
	}

	if len(filters.VendorNames) > 0 {
		query = query.Where("vendor_name IN ?", filters.VendorNames)
	}

	if len(filters.CustomerNames) > 0 {
		query = query.Where("customer_name IN ?", filters.CustomerNames)
	}

	if filters.Currency != "" {
		query = query.Where("currency = ?", strings.ToUpper(filters.Currency))
	}

	if filters.DateFrom != nil {
		query = query.Where("document_date >= ?", *filters.DateFrom)
	}

	if filters.DateTo != nil {
		query = query.Where("document_date <= ?", *filters.DateTo)
	}

	// Apply pagination
	offset := (filters.Page - 1) * filters.PageSize
	orderBy := "document_date DESC"
	if filters.SortBy != "" {
		direction := "ASC"
		if filters.SortDesc {
			direction = "DESC"
		}
		orderBy = fmt.Sprintf("%s %s", filters.SortBy, direction)
	}

	err := query.Preload("Creator").Preload("Folder").
		Order(orderBy).Offset(offset).Limit(filters.PageSize).Find(&documents).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get financial documents: %w", err)
	}

	return documents, nil
}

func (r *DocumentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.DocStatus) error {
	result := r.db.WithContext(ctx).Model(&models.Document{}).
		Where("id = ?", id).Update("status", status)

	if result.Error != nil {
		return fmt.Errorf("failed to update document status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

func (r *DocumentRepository) AssociateTags(ctx context.Context, documentID uuid.UUID, tagIDs []uuid.UUID) error {
	var document models.Document
	if err := r.db.WithContext(ctx).First(&document, documentID).Error; err != nil {
		return fmt.Errorf("document not found: %w", err)
	}

	var tags []models.Tag
	if err := r.db.WithContext(ctx).Find(&tags, tagIDs).Error; err != nil {
		return fmt.Errorf("failed to find tags: %w", err)
	}

	if err := r.db.WithContext(ctx).Model(&document).Association("Tags").Replace(tags); err != nil {
		return fmt.Errorf("failed to associate tags: %w", err)
	}

	return nil
}

func (r *DocumentRepository) AssociateCategories(ctx context.Context, documentID uuid.UUID, categoryIDs []uuid.UUID) error {
	var document models.Document
	if err := r.db.WithContext(ctx).First(&document, documentID).Error; err != nil {
		return fmt.Errorf("document not found: %w", err)
	}

	var categories []models.Category
	if err := r.db.WithContext(ctx).Find(&categories, categoryIDs).Error; err != nil {
		return fmt.Errorf("failed to find categories: %w", err)
	}

	if err := r.db.WithContext(ctx).Model(&document).Association("Categories").Replace(categories); err != nil {
		return fmt.Errorf("failed to associate categories: %w", err)
	}

	return nil
}

func (r *DocumentRepository) SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&models.Document{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     models.DocStatusArchived,
			"updated_by": deletedBy,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to soft delete document: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

func (r *DocumentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&models.Document{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete document: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}
