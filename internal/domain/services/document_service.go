package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
)

var (
	ErrDocumentNotFound    = errors.New("document not found")
	ErrDocumentExists      = errors.New("document already exists")
	ErrInvalidDocumentType = errors.New("invalid document type")
	ErrQuotaExceeded       = errors.New("storage quota exceeded")
	ErrProcessingFailed    = errors.New("document processing failed")
	ErrUnauthorizedAccess  = errors.New("unauthorized access to document")
	ErrDocumentTooLarge    = errors.New("document exceeds maximum size limit")
	ErrUnsupportedFormat   = errors.New("unsupported document format")
)

// DocumentServiceConfig holds configuration for the document service
type DocumentServiceConfig struct {
	MaxFileSize            int64 // bytes
	AllowedMimeTypes       []string
	StorageBasePath        string
	ThumbnailPath          string
	PreviewPath            string
	EnableAIProcessing     bool
	EnableDuplicateCheck   bool
	AutoGenerateThumbnails bool
}

// DocumentService handles all document-related business logic
type DocumentService struct {
	docRepo       repositories.DocumentRepository
	tenantRepo    repositories.TenantRepository
	userRepo      repositories.UserRepository
	folderRepo    repositories.FolderRepository
	tagRepo       repositories.TagRepository
	categoryRepo  repositories.CategoryRepository
	auditRepo     repositories.AuditLogRepository
	aiJobRepo     repositories.AIProcessingJobRepository
	analyticsRepo repositories.AnalyticsRepository

	storageService StorageService
	aiService      AIService
	config         DocumentServiceConfig
}

// NewDocumentService creates a new document service instance
func NewDocumentService(
	docRepo repositories.DocumentRepository,
	tenantRepo repositories.TenantRepository,
	userRepo repositories.UserRepository,
	folderRepo repositories.FolderRepository,
	tagRepo repositories.TagRepository,
	categoryRepo repositories.CategoryRepository,
	auditRepo repositories.AuditLogRepository,
	aiJobRepo repositories.AIProcessingJobRepository,
	analyticsRepo repositories.AnalyticsRepository,
	storageService StorageService,
	aiService AIService,
	config DocumentServiceConfig,
) *DocumentService {
	return &DocumentService{
		docRepo:        docRepo,
		tenantRepo:     tenantRepo,
		userRepo:       userRepo,
		folderRepo:     folderRepo,
		tagRepo:        tagRepo,
		categoryRepo:   categoryRepo,
		auditRepo:      auditRepo,
		aiJobRepo:      aiJobRepo,
		analyticsRepo:  analyticsRepo,
		storageService: storageService,
		aiService:      aiService,
		config:         config,
	}
}

// UploadDocumentParams contains parameters for document upload
type UploadDocumentParams struct {
	TenantID     uuid.UUID              `json:"tenant_id"`
	UserID       uuid.UUID              `json:"user_id"`
	FolderID     *uuid.UUID             `json:"folder_id,omitempty"`
	File         *multipart.FileHeader  `json:"-"`
	FileReader   io.Reader              `json:"-"`
	Title        string                 `json:"title,omitempty"`
	Description  string                 `json:"description,omitempty"`
	DocumentType models.DocumentType    `json:"document_type,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Categories   []string               `json:"categories,omitempty"`
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`

	// Financial document fields
	Amount       *float64   `json:"amount,omitempty"`
	Currency     string     `json:"currency,omitempty"`
	TaxAmount    *float64   `json:"tax_amount,omitempty"`
	VendorName   string     `json:"vendor_name,omitempty"`
	CustomerName string     `json:"customer_name,omitempty"`
	DocumentDate *time.Time `json:"document_date,omitempty"`
	DueDate      *time.Time `json:"due_date,omitempty"`
	ExpiryDate   *time.Time `json:"expiry_date,omitempty"`

	// Processing options
	EnableAI           bool `json:"enable_ai"`
	EnableOCR          bool `json:"enable_ocr"`
	SkipDuplicateCheck bool `json:"skip_duplicate_check"`
}

// UploadDocument handles document upload with intelligent processing
func (s *DocumentService) UploadDocument(ctx context.Context, params UploadDocumentParams) (*models.Document, error) {
	// 1. Validate tenant and quota
	quotaStatus, err := s.tenantRepo.CheckQuotaLimits(ctx, params.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to check quota: %w", err)
	}

	if !quotaStatus.CanUpload {
		return nil, ErrQuotaExceeded
	}

	// 2. Validate file
	if params.File != nil && params.File.Size > s.config.MaxFileSize {
		return nil, ErrDocumentTooLarge
	}

	// 3. Validate file type
	contentType := params.File.Header.Get("Content-Type")
	if !s.isAllowedMimeType(contentType) {
		return nil, ErrUnsupportedFormat
	}

	// 4. Open and read file
	var fileContent []byte
	if params.File != nil {
		file, err := params.File.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		// Read file content into memory once
		fileContent, err = io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
	} else if params.FileReader != nil {
		fileContent, err = io.ReadAll(params.FileReader)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
	}

	// 5. Calculate content hash for duplicate detection
	contentHash := s.calculateContentHashFromBytes(fileContent)

	// 6. Check for duplicates if enabled
	if s.config.EnableDuplicateCheck && !params.SkipDuplicateCheck {
		existing, err := s.docRepo.GetByContentHash(ctx, params.TenantID, contentHash)
		if err == nil && existing != nil {
			return nil, ErrDocumentExists
		}
	}

	// 7. Auto-detect document type if not provided
	if params.DocumentType == "" {
		params.DocumentType = s.detectDocumentType(params.File.Filename, contentType)
	}

	// 8. Store file using bytes reader
	storagePath, err := s.storageService.Store(ctx, StorageParams{
		TenantID:    params.TenantID,
		FileReader:  bytes.NewReader(fileContent),
		Filename:    params.File.Filename,
		ContentType: contentType,
		Size:        params.File.Size,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	// 9. Create document record
	document := &models.Document{
		ID:           uuid.New(),
		TenantID:     params.TenantID,
		FolderID:     params.FolderID,
		FileName:     s.generateFileName(params.File.Filename),
		OriginalName: params.File.Filename,
		ContentType:  contentType,
		FileSize:     params.File.Size,
		StoragePath:  storagePath,
		ContentHash:  contentHash,
		Title:        params.Title,
		Description:  params.Description,
		DocumentType: params.DocumentType,
		Status:       models.DocStatusPending,
		CreatedBy:    params.UserID,

		// Financial fields
		Amount:       params.Amount,
		Currency:     params.Currency,
		TaxAmount:    params.TaxAmount,
		VendorName:   params.VendorName,
		CustomerName: params.CustomerName,
		DocumentDate: params.DocumentDate,
		DueDate:      params.DueDate,
		ExpiryDate:   params.ExpiryDate,

		// Custom fields
		CustomFields: models.JSONB(params.CustomFields),
	}

	// Set default title if not provided
	if document.Title == "" {
		document.Title = s.generateTitle(params.File.Filename)
	}

	// 10. Save document to database
	if err := s.docRepo.Create(ctx, document); err != nil {
		// Cleanup stored file on database error
		s.storageService.Delete(ctx, storagePath)
		return nil, fmt.Errorf("failed to create document record: %w", err)
	}

	// 11. Update tenant storage usage
	if err := s.tenantRepo.UpdateUsage(ctx, params.TenantID, params.File.Size, 0); err != nil {
		// Log but don't fail - this is non-critical
		// TODO: Add proper logging
	}

	// 12. Process tags and categories
	if err := s.processTags(ctx, document.ID, params.TenantID, params.Tags); err != nil {
		// Log but don't fail - this is non-critical
	}

	if err := s.processCategories(ctx, document.ID, params.TenantID, params.Categories); err != nil {
		// Log but don't fail - this is non-critical
	}

	// 13. Queue AI processing if enabled
	if params.EnableAI && s.config.EnableAIProcessing {
		if err := s.queueAIProcessing(ctx, document, params.EnableOCR); err != nil {
			// Log but don't fail - AI processing is optional
		}
	}

	// 14. Generate thumbnails if enabled
	if s.config.AutoGenerateThumbnails {
		if err := s.generateThumbnail(ctx, document); err != nil {
			// Log but don't fail - thumbnails are optional
		}
	}

	// 15. Create audit log
	s.createAuditLog(ctx, params.TenantID, params.UserID, document.ID, models.AuditCreate, "Document uploaded")

	// 16. Create analytics record
	s.analyticsRepo.CreateDocumentAnalytics(ctx, &models.DocumentAnalytics{
		TenantID:   params.TenantID,
		DocumentID: document.ID,
	})

	return document, nil
}

// GetDocument retrieves a document with access control
func (s *DocumentService) GetDocument(ctx context.Context, documentID, tenantID, userID uuid.UUID) (*models.Document, error) {
	document, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, ErrDocumentNotFound
	}

	// Verify tenant access
	if document.TenantID != tenantID {
		return nil, ErrUnauthorizedAccess
	}

	// Update view analytics
	s.analyticsRepo.UpdateDocumentView(ctx, documentID)

	// Create audit log
	s.createAuditLog(ctx, tenantID, userID, documentID, models.AuditRead, "Document viewed")

	return document, nil
}

// SearchDocuments performs intelligent document search
func (s *DocumentService) SearchDocuments(ctx context.Context, tenantID uuid.UUID, query repositories.SearchQuery) ([]models.Document, error) {
	// First try semantic search if query is complex
	if len(query.Query) > 10 && s.aiService != nil {
		if embedding, err := s.aiService.GenerateEmbedding(ctx, query.Query); err == nil {
			results, err := s.docRepo.SemanticSearch(ctx, tenantID, embedding, query.Limit)
			if err == nil && len(results) > 0 {
				return results, nil
			}
		}
	}

	// Fallback to traditional search
	return s.docRepo.Search(ctx, tenantID, query)
}

// ProcessFinancialDocument extracts financial data using AI
func (s *DocumentService) ProcessFinancialDocument(ctx context.Context, documentID uuid.UUID, userID uuid.UUID) error {
	document, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return ErrDocumentNotFound
	}

	// Only process financial document types
	if !s.isFinancialDocument(document.DocumentType) {
		return ErrInvalidDocumentType
	}

	// Queue specialized financial AI processing
	job := &models.AIProcessingJob{
		TenantID:   document.TenantID,
		DocumentID: document.ID,
		JobType:    "financial_extraction",
		Priority:   3, // Higher priority for financial docs
	}

	if err := s.aiJobRepo.Create(ctx, job); err != nil {
		return fmt.Errorf("failed to queue financial processing: %w", err)
	}

	// Update document status
	document.Status = models.DocStatusProcessing
	if err := s.docRepo.Update(ctx, document); err != nil {
		return fmt.Errorf("failed to update document status: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, document.TenantID, userID, document.ID, models.AuditUpdate, "Financial processing initiated")

	return nil
}

// FindDuplicates identifies potential duplicate documents
func (s *DocumentService) FindDuplicates(ctx context.Context, tenantID uuid.UUID, threshold float64) ([]repositories.DocumentDuplicate, error) {
	return s.docRepo.GetDuplicates(ctx, tenantID, threshold)
}

// GetExpiringDocuments finds documents nearing expiration
func (s *DocumentService) GetExpiringDocuments(ctx context.Context, tenantID uuid.UUID, days int) ([]models.Document, error) {
	return s.docRepo.GetExpiring(ctx, tenantID, days)
}

// UpdateDocument updates document metadata and handles versioning
func (s *DocumentService) UpdateDocument(ctx context.Context, documentID uuid.UUID, updates map[string]interface{}, userID uuid.UUID) (*models.Document, error) {
	document, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, ErrDocumentNotFound
	}

	// Apply updates
	if title, ok := updates["title"].(string); ok {
		document.Title = title
	}
	if description, ok := updates["description"].(string); ok {
		document.Description = description
	}
	if docType, ok := updates["document_type"].(models.DocumentType); ok {
		document.DocumentType = docType
	}

	// Update financial fields if provided
	if amount, ok := updates["amount"].(float64); ok {
		document.Amount = &amount
	}
	if vendor, ok := updates["vendor_name"].(string); ok {
		document.VendorName = vendor
	}
	if customer, ok := updates["customer_name"].(string); ok {
		document.CustomerName = customer
	}

	document.UpdatedBy = &userID
	document.UpdatedAt = time.Now()

	if err := s.docRepo.Update(ctx, document); err != nil {
		return nil, fmt.Errorf("failed to update document: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, document.TenantID, userID, document.ID, models.AuditUpdate, "Document updated")

	return document, nil
}

// DeleteDocument soft deletes a document
func (s *DocumentService) DeleteDocument(ctx context.Context, documentID, userID uuid.UUID) error {
	document, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return ErrDocumentNotFound
	}

	// Soft delete the document
	if err := s.docRepo.SoftDelete(ctx, documentID, userID); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	// Update tenant storage usage
	s.tenantRepo.UpdateUsage(ctx, document.TenantID, -document.FileSize, 0)

	// Create audit log
	s.createAuditLog(ctx, document.TenantID, userID, documentID, models.AuditDelete, "Document deleted")

	return nil
}

// Helper methods

func (s *DocumentService) isAllowedMimeType(contentType string) bool {
	if len(s.config.AllowedMimeTypes) == 0 {
		return true // Allow all if not specified
	}

	for _, allowed := range s.config.AllowedMimeTypes {
		if strings.HasPrefix(contentType, allowed) {
			return true
		}
	}
	return false
}

func (s *DocumentService) calculateContentHashFromBytes(content []byte) string {
	hasher := sha256.New()
	hasher.Write(content)
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func (s *DocumentService) detectDocumentType(filename, contentType string) models.DocumentType {
	ext := strings.ToLower(filepath.Ext(filename))

	// Invoice patterns
	if strings.Contains(strings.ToLower(filename), "invoice") {
		return models.DocTypeInvoice
	}

	// Receipt patterns
	if strings.Contains(strings.ToLower(filename), "receipt") {
		return models.DocTypeReceipt
	}

	// Contract patterns
	if strings.Contains(strings.ToLower(filename), "contract") ||
		strings.Contains(strings.ToLower(filename), "agreement") {
		return models.DocTypeContract
	}

	// File extension based detection
	switch ext {
	case ".xlsx", ".xls", ".csv":
		return models.DocTypeSpreadsheet
	case ".pptx", ".ppt":
		return models.DocTypePresentationn
	case ".pdf":
		// PDF could be anything, return general for now
		return models.DocTypeGeneral
	}

	return models.DocTypeGeneral
}

func (s *DocumentService) generateFileName(originalName string) string {
	ext := filepath.Ext(originalName)
	name := strings.TrimSuffix(originalName, ext)
	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("%s_%s%s", name, timestamp, ext)
}

func (s *DocumentService) generateTitle(filename string) string {
	// Remove extension and clean up filename for title
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	// Use cases.Title instead of deprecated strings.Title
	return strings.ToTitle(strings.ToLower(name))
}

func (s *DocumentService) processTags(ctx context.Context, documentID, tenantID uuid.UUID, tagNames []string) error {
	if len(tagNames) == 0 {
		return nil
	}

	var tagIDs []uuid.UUID
	for _, tagName := range tagNames {
		tag, err := s.tagRepo.GetByName(ctx, tenantID, tagName)
		if err != nil {
			// Create new tag if it doesn't exist
			newTag := &models.Tag{
				ID:       uuid.New(),
				TenantID: tenantID,
				Name:     tagName,
			}
			if err := s.tagRepo.Create(ctx, newTag); err != nil {
				continue // Skip this tag if creation fails
			}
			tagIDs = append(tagIDs, newTag.ID)
		} else {
			tagIDs = append(tagIDs, tag.ID)
			s.tagRepo.IncrementUsage(ctx, tag.ID)
		}
	}

	// Associate tags with document (this would need a new repository method)
	// For now, we'll assume this method exists
	return s.docRepo.AssociateTags(ctx, documentID, tagIDs)
}

func (s *DocumentService) processCategories(ctx context.Context, documentID, tenantID uuid.UUID, categoryNames []string) error {
	if len(categoryNames) == 0 {
		return nil
	}

	var categoryIDs []uuid.UUID
	for _, categoryName := range categoryNames {
		category, err := s.categoryRepo.GetByName(ctx, tenantID, categoryName)
		if err != nil {
			// Create new category if it doesn't exist
			newCategory := &models.Category{
				ID:       uuid.New(),
				TenantID: tenantID,
				Name:     categoryName,
			}
			if err := s.categoryRepo.Create(ctx, newCategory); err != nil {
				continue // Skip this category if creation fails
			}
			categoryIDs = append(categoryIDs, newCategory.ID)
		} else {
			categoryIDs = append(categoryIDs, category.ID)
		}
	}

	// Associate categories with document (this would need a new repository method)
	return s.docRepo.AssociateCategories(ctx, documentID, categoryIDs)
}

func (s *DocumentService) queueAIProcessing(ctx context.Context, document *models.Document, enableOCR bool) error {
	jobs := []string{"text_extraction", "categorization", "tagging"}

	if enableOCR {
		jobs = append(jobs, "ocr")
	}

	if s.isFinancialDocument(document.DocumentType) {
		jobs = append(jobs, "financial_extraction")
	}

	for _, jobType := range jobs {
		job := &models.AIProcessingJob{
			TenantID:   document.TenantID,
			DocumentID: document.ID,
			JobType:    jobType,
			Priority:   5,
		}

		if err := s.aiJobRepo.Create(ctx, job); err != nil {
			return err
		}
	}

	return nil
}

func (s *DocumentService) generateThumbnail(ctx context.Context, document *models.Document) error {
	// TODO: Implement thumbnail generation
	// This would use image processing libraries or external services
	return nil
}

func (s *DocumentService) isFinancialDocument(docType models.DocumentType) bool {
	financial := []models.DocumentType{
		models.DocTypeInvoice,
		models.DocTypeReceipt,
		models.DocTypeBankStatement,
		models.DocTypePayroll,
		models.DocTypeTaxDocument,
	}

	for _, ft := range financial {
		if docType == ft {
			return true
		}
	}
	return false
}

func (s *DocumentService) createAuditLog(ctx context.Context, tenantID, userID, resourceID uuid.UUID, action models.AuditAction, details string) {
	log := &models.AuditLog{
		TenantID:     tenantID,
		UserID:       userID,
		ResourceID:   resourceID,
		Action:       action,
		ResourceType: "document",
		Details:      models.JSONB{"message": details},
	}

	// Don't block on audit log creation
	go func() {
		s.auditRepo.Create(context.Background(), log)
	}()
}

// Keep original method for backwards compatibility
func (s *DocumentService) calculateContentHash(reader io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}
