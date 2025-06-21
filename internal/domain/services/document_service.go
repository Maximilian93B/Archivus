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

	// Initialize embedding to prevent PostgreSQL vector dimension errors
	// When no AI service is available, we leave embedding as nil (NULL in database)
	// This prevents the "vector must have at least 1 dimension" error

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

// ListDocuments lists documents with filtering and pagination
func (s *DocumentService) ListDocuments(ctx context.Context, tenantID uuid.UUID, filters repositories.DocumentFilters) ([]models.Document, int64, error) {
	return s.docRepo.List(ctx, tenantID, filters)
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
	// Phase 3 AI job types - Core document intelligence processing
	jobs := []string{
		JobTypeDocumentSummarization,  // Generate intelligent summaries
		JobTypeEntityExtraction,       // Extract people, organizations, dates, amounts
		JobTypeDocumentClassification, // Classify document type with confidence
		JobTypeSemanticAnalysis,       // Generate tags and semantic understanding
	}

	// Add embedding generation for semantic search (requires OpenAI or similar)
	// Note: Claude doesn't provide embeddings directly
	if s.aiService != nil {
		jobs = append(jobs, JobTypeEmbeddingGeneration)
	}

	// Add OCR processing if requested and document is image-based
	if enableOCR && s.isImageDocument(document.ContentType) {
		jobs = append(jobs, "ocr_extraction")
	}

	// Add specialized financial processing for financial documents
	if s.isFinancialDocument(document.DocumentType) {
		jobs = append(jobs, "financial_extraction")
	}

	// Queue AI jobs with appropriate priorities
	for i, jobType := range jobs {
		job := &models.AIProcessingJob{
			TenantID:   document.TenantID,
			DocumentID: document.ID,
			JobType:    jobType,
			Priority:   5 - i, // Higher priority for earlier jobs (summarization first)
			Status:     models.ProcessingQueued,
		}

		if err := s.aiJobRepo.Create(ctx, job); err != nil {
			return fmt.Errorf("failed to queue AI job %s: %w", jobType, err)
		}
	}

	return nil
}

// isImageDocument checks if the document is image-based and would benefit from OCR
func (s *DocumentService) isImageDocument(contentType string) bool {
	imageTypes := []string{
		"image/jpeg",
		"image/png",
		"image/tiff",
		"image/bmp",
		"application/pdf", // PDFs might contain scanned images
	}

	for _, imageType := range imageTypes {
		if strings.HasPrefix(contentType, imageType) {
			return true
		}
	}
	return false
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
	_, err := io.Copy(hasher, reader)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// DownloadDocument retrieves a document file from storage
func (s *DocumentService) DownloadDocument(ctx context.Context, documentID, tenantID, userID uuid.UUID) (io.ReadCloser, error) {
	// Get document with access control
	document, err := s.GetDocument(ctx, documentID, tenantID, userID)
	if err != nil {
		return nil, err
	}

	// Get file from storage service
	fileReader, err := s.storageService.Get(ctx, document.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve file from storage: %w", err)
	}

	// Create audit log for download
	s.createAuditLog(ctx, tenantID, userID, documentID, models.AuditRead, "Document downloaded")

	// Update analytics
	s.analyticsRepo.UpdateDocumentDownload(ctx, documentID)

	return fileReader, nil
}

// GetDocumentPreview retrieves a preview/thumbnail of a document
func (s *DocumentService) GetDocumentPreview(ctx context.Context, documentID, tenantID, userID uuid.UUID) (io.ReadCloser, string, error) {
	// Get document with access control
	document, err := s.GetDocument(ctx, documentID, tenantID, userID)
	if err != nil {
		return nil, "", err
	}

	// Try to get generated preview/thumbnail first
	previewPath := s.getPreviewPath(document.StoragePath)
	thumbnailPath := s.getThumbnailPath(document.StoragePath)

	// Check for existing preview
	if previewReader, err := s.storageService.Get(ctx, previewPath); err == nil {
		s.createAuditLog(ctx, tenantID, userID, documentID, models.AuditRead, "Document preview viewed")
		s.analyticsRepo.UpdateDocumentView(ctx, documentID)
		return previewReader, "text/plain", nil // Placeholder content type
	}

	// Check for existing thumbnail
	if thumbnailReader, err := s.storageService.Get(ctx, thumbnailPath); err == nil {
		s.createAuditLog(ctx, tenantID, userID, documentID, models.AuditRead, "Document thumbnail viewed")
		s.analyticsRepo.UpdateDocumentView(ctx, documentID)
		return thumbnailReader, "image/jpeg", nil // Placeholder content type
	}

	// If no preview/thumbnail exists, generate one on-demand
	if err := s.generatePreviewOnDemand(ctx, document); err != nil {
		// If generation fails, return a placeholder
		placeholder := fmt.Sprintf("Preview not available for %s\nDocument Type: %s\nFile Size: %d bytes",
			document.OriginalName, document.ContentType, document.FileSize)

		return io.NopCloser(strings.NewReader(placeholder)), "text/plain", nil
	}

	// Try to get the newly generated preview
	if previewReader, err := s.storageService.Get(ctx, previewPath); err == nil {
		s.createAuditLog(ctx, tenantID, userID, documentID, models.AuditRead, "Document preview generated and viewed")
		s.analyticsRepo.UpdateDocumentView(ctx, documentID)
		return previewReader, "text/plain", nil
	}

	// Fallback to placeholder
	placeholder := fmt.Sprintf("Preview not available for %s\nDocument Type: %s\nFile Size: %d bytes",
		document.OriginalName, document.ContentType, document.FileSize)

	return io.NopCloser(strings.NewReader(placeholder)), "text/plain", nil
}

// Helper methods for preview/thumbnail paths
func (s *DocumentService) getPreviewPath(originalPath string) string {
	ext := filepath.Ext(originalPath)
	baseName := strings.TrimSuffix(originalPath, ext)
	return fmt.Sprintf("previews/%s_preview.txt", filepath.Base(baseName))
}

func (s *DocumentService) getThumbnailPath(originalPath string) string {
	ext := filepath.Ext(originalPath)
	baseName := strings.TrimSuffix(originalPath, ext)
	return fmt.Sprintf("thumbnails/%s_thumb.jpg", filepath.Base(baseName))
}

// generatePreviewOnDemand generates a preview for a document on-demand
func (s *DocumentService) generatePreviewOnDemand(ctx context.Context, document *models.Document) error {
	// Create background job for preview generation
	job := &models.AIProcessingJob{
		TenantID:   document.TenantID,
		DocumentID: document.ID,
		JobType:    "preview_generation",
		Priority:   1, // High priority for on-demand generation
		Status:     models.ProcessingQueued,
	}

	return s.aiJobRepo.Create(ctx, job)
}

// FOLDER MANAGEMENT METHODS

// CreateFolder creates a new folder with proper business logic
func (s *DocumentService) CreateFolder(ctx context.Context, tenantID, userID uuid.UUID, name, description string, parentID *uuid.UUID, color, icon string) (*models.Folder, error) {
	// Validate name
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("folder name cannot be empty")
	}

	// Build folder path and level
	path := name
	level := 0

	if parentID != nil {
		// Get parent folder and validate it belongs to the same tenant
		parent, err := s.folderRepo.GetByID(ctx, *parentID)
		if err != nil {
			return nil, fmt.Errorf("parent folder not found: %w", err)
		}
		if parent.TenantID != tenantID {
			return nil, fmt.Errorf("parent folder belongs to different tenant")
		}

		// Check for name conflicts in the same parent
		if existingFolder, err := s.folderRepo.GetByPath(ctx, tenantID, parent.Path+"/"+name); err == nil && existingFolder != nil {
			return nil, fmt.Errorf("folder with this name already exists in parent directory")
		}

		path = parent.Path + "/" + name
		level = parent.Level + 1
	} else {
		// Root folder - check for name conflicts at root level
		if existingFolder, err := s.folderRepo.GetByPath(ctx, tenantID, "/"+name); err == nil && existingFolder != nil {
			return nil, fmt.Errorf("folder with this name already exists at root level")
		}
		path = "/" + name
		level = 0
	}

	// Set defaults
	if color == "" {
		color = "#6B7280"
	}
	if icon == "" {
		icon = "folder"
	}

	// Create folder
	folder := &models.Folder{
		ID:          uuid.New(),
		TenantID:    tenantID,
		ParentID:    parentID,
		Name:        name,
		Description: description,
		Path:        path,
		Level:       level,
		IsSystem:    false,
		Color:       color,
		Icon:        icon,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.folderRepo.Create(ctx, folder); err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, tenantID, userID, folder.ID, models.AuditCreate, "Folder created: "+name)

	return folder, nil
}

// GetFolder retrieves a folder with access control
func (s *DocumentService) GetFolder(ctx context.Context, folderID, tenantID uuid.UUID) (*models.Folder, error) {
	folder, err := s.folderRepo.GetByID(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("folder not found")
	}

	// Verify tenant access
	if folder.TenantID != tenantID {
		return nil, fmt.Errorf("unauthorized access to folder")
	}

	return folder, nil
}

// GetFolders lists folders with optional parent filtering
func (s *DocumentService) GetFolders(ctx context.Context, tenantID uuid.UUID, parentID *uuid.UUID) ([]models.Folder, error) {
	if parentID != nil {
		// Get children of specific parent
		return s.folderRepo.GetChildren(ctx, *parentID)
	}

	// Get all root folders (parentID is null)
	var folders []models.Folder
	// This would require a new repository method to get root folders by tenant
	// For now, we'll use the tree method and extract root folders
	tree, err := s.folderRepo.GetTree(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	for _, node := range tree {
		folders = append(folders, *node.Folder)
	}

	return folders, nil
}

// UpdateFolder updates an existing folder
func (s *DocumentService) UpdateFolder(ctx context.Context, folderID, tenantID uuid.UUID, updates map[string]interface{}, userID uuid.UUID) (*models.Folder, error) {
	// Get existing folder
	folder, err := s.GetFolder(ctx, folderID, tenantID)
	if err != nil {
		return nil, err
	}

	// Check if it's a system folder
	if folder.IsSystem {
		return nil, fmt.Errorf("cannot modify system folder")
	}

	// Apply updates
	updated := false
	if name, ok := updates["name"].(string); ok && strings.TrimSpace(name) != "" {
		if name != folder.Name {
			// Check for name conflicts
			newPath := strings.Replace(folder.Path, "/"+folder.Name, "/"+name, 1)
			if existingFolder, err := s.folderRepo.GetByPath(ctx, tenantID, newPath); err == nil && existingFolder != nil && existingFolder.ID != folder.ID {
				return nil, fmt.Errorf("folder with this name already exists")
			}

			folder.Name = name
			folder.Path = newPath
			updated = true
		}
	}

	if description, ok := updates["description"].(string); ok {
		folder.Description = description
		updated = true
	}

	if color, ok := updates["color"].(string); ok && color != "" {
		folder.Color = color
		updated = true
	}

	if icon, ok := updates["icon"].(string); ok && icon != "" {
		folder.Icon = icon
		updated = true
	}

	if updated {
		folder.UpdatedAt = time.Now()
		if err := s.folderRepo.Update(ctx, folder); err != nil {
			return nil, fmt.Errorf("failed to update folder: %w", err)
		}

		// Create audit log
		s.createAuditLog(ctx, tenantID, userID, folder.ID, models.AuditUpdate, "Folder updated")
	}

	return folder, nil
}

// DeleteFolder deletes a folder with validation
func (s *DocumentService) DeleteFolder(ctx context.Context, folderID, tenantID, userID uuid.UUID) error {
	// Get folder first to check permissions
	folder, err := s.GetFolder(ctx, folderID, tenantID)
	if err != nil {
		return err
	}

	// Check if it's a system folder
	if folder.IsSystem {
		return fmt.Errorf("cannot delete system folder")
	}

	// Delete using repository (it handles validation for children and documents)
	if err := s.folderRepo.Delete(ctx, folderID); err != nil {
		return err
	}

	// Create audit log
	s.createAuditLog(ctx, tenantID, userID, folderID, models.AuditDelete, "Folder deleted: "+folder.Name)

	return nil
}

// GetFolderTree retrieves the complete folder hierarchy
func (s *DocumentService) GetFolderTree(ctx context.Context, tenantID uuid.UUID) ([]repositories.FolderNode, error) {
	tree, err := s.folderRepo.GetTree(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder tree: %w", err)
	}

	// Populate document counts for each folder
	for i := range tree {
		s.populateDocumentCount(ctx, &tree[i])
	}

	return tree, nil
}

// MoveFolder moves a folder to a new parent location
func (s *DocumentService) MoveFolder(ctx context.Context, folderID, newParentID, tenantID, userID uuid.UUID) (*models.Folder, error) {
	// Get folder to move
	folder, err := s.GetFolder(ctx, folderID, tenantID)
	if err != nil {
		return nil, err
	}

	// Check if it's a system folder
	if folder.IsSystem {
		return nil, fmt.Errorf("cannot move system folder")
	}

	// Validate new parent exists and belongs to same tenant
	newParent, err := s.GetFolder(ctx, newParentID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("new parent folder not found")
	}

	// Prevent moving folder to itself or its descendant
	if folderID == newParentID {
		return nil, fmt.Errorf("cannot move folder to itself")
	}

	// TODO: Add cycle detection logic here
	// For now, we'll rely on the repository implementation

	// Use repository move method
	if err := s.folderRepo.Move(ctx, folderID, newParentID); err != nil {
		return nil, fmt.Errorf("failed to move folder: %w", err)
	}

	// Get updated folder
	updatedFolder, err := s.GetFolder(ctx, folderID, tenantID)
	if err != nil {
		return nil, err
	}

	// Create audit log
	s.createAuditLog(ctx, tenantID, userID, folderID, models.AuditUpdate,
		fmt.Sprintf("Folder moved to %s", newParent.Name))

	return updatedFolder, nil
}

// GetFolderDocuments retrieves documents in a specific folder
func (s *DocumentService) GetFolderDocuments(ctx context.Context, folderID, tenantID uuid.UUID, filters repositories.DocumentFilters) ([]models.Document, int64, error) {
	// Verify folder access
	_, err := s.GetFolder(ctx, folderID, tenantID)
	if err != nil {
		return nil, 0, err
	}

	// Set folder filter and call existing list method
	filters.FolderID = &folderID
	return s.docRepo.List(ctx, tenantID, filters)
}

// GetFolderChildren gets immediate child folders
func (s *DocumentService) GetFolderChildren(ctx context.Context, folderID uuid.UUID) ([]models.Folder, error) {
	return s.folderRepo.GetChildren(ctx, folderID)
}

// Helper method to populate document counts recursively
func (s *DocumentService) populateDocumentCount(ctx context.Context, node *repositories.FolderNode) {
	// Get document count for this folder
	count, err := s.folderRepo.GetDocumentCount(ctx, node.Folder.ID)
	if err == nil {
		node.DocumentCount = count
	}

	// Recursively populate children
	for i := range node.Children {
		s.populateDocumentCount(ctx, &node.Children[i])
	}
}

// TAG MANAGEMENT METHODS

// CreateTag creates a new tag with validation
func (s *DocumentService) CreateTag(ctx context.Context, tenantID, userID uuid.UUID, name, color string) (*models.Tag, error) {
	// Validate name
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("tag name cannot be empty")
	}

	// Check for duplicate names in tenant
	if existingTag, err := s.tagRepo.GetByName(ctx, tenantID, name); err == nil && existingTag != nil {
		return nil, fmt.Errorf("tag with name '%s' already exists", name)
	}

	// Set defaults
	if color == "" {
		color = "#6B7280"
	}

	// Create tag
	tag := &models.Tag{
		ID:            uuid.New(),
		TenantID:      tenantID,
		Name:          name,
		Color:         color,
		IsAIGenerated: false,
		UsageCount:    0,
		CreatedAt:     time.Now(),
	}

	if err := s.tagRepo.Create(ctx, tag); err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, tenantID, userID, tag.ID, models.AuditCreate, "Tag created: "+name)

	return tag, nil
}

// GetTag retrieves a tag with access control
func (s *DocumentService) GetTag(ctx context.Context, tagID, tenantID uuid.UUID) (*models.Tag, error) {
	tag, err := s.tagRepo.GetByID(ctx, tagID)
	if err != nil {
		return nil, fmt.Errorf("tag not found")
	}

	// Verify tenant access
	if tag.TenantID != tenantID {
		return nil, fmt.Errorf("unauthorized access to tag")
	}

	return tag, nil
}

// GetTagByName retrieves a tag by name
func (s *DocumentService) GetTagByName(ctx context.Context, tenantID uuid.UUID, name string) (*models.Tag, error) {
	return s.tagRepo.GetByName(ctx, tenantID, name)
}

// ListTags lists all tags for a tenant
func (s *DocumentService) ListTags(ctx context.Context, tenantID uuid.UUID) ([]models.Tag, error) {
	return s.tagRepo.ListByTenant(ctx, tenantID)
}

// GetPopularTags gets most used tags
func (s *DocumentService) GetPopularTags(ctx context.Context, tenantID uuid.UUID, limit int) ([]models.Tag, error) {
	if limit <= 0 {
		limit = 20 // Default limit
	}
	return s.tagRepo.GetPopular(ctx, tenantID, limit)
}

// UpdateTag updates an existing tag
func (s *DocumentService) UpdateTag(ctx context.Context, tagID, tenantID uuid.UUID, updates map[string]interface{}, userID uuid.UUID) (*models.Tag, error) {
	// Get existing tag
	tag, err := s.GetTag(ctx, tagID, tenantID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	updated := false
	if name, ok := updates["name"].(string); ok && strings.TrimSpace(name) != "" {
		if name != tag.Name {
			// Check for name conflicts
			if existingTag, err := s.tagRepo.GetByName(ctx, tenantID, name); err == nil && existingTag != nil && existingTag.ID != tag.ID {
				return nil, fmt.Errorf("tag with name '%s' already exists", name)
			}
			tag.Name = name
			updated = true
		}
	}

	if color, ok := updates["color"].(string); ok && color != "" {
		tag.Color = color
		updated = true
	}

	if updated {
		if err := s.tagRepo.Update(ctx, tag); err != nil {
			return nil, fmt.Errorf("failed to update tag: %w", err)
		}

		// Create audit log
		s.createAuditLog(ctx, tenantID, userID, tag.ID, models.AuditUpdate, "Tag updated")
	}

	return tag, nil
}

// DeleteTag deletes a tag with validation
func (s *DocumentService) DeleteTag(ctx context.Context, tagID, tenantID, userID uuid.UUID) error {
	// Get tag first to check permissions
	tag, err := s.GetTag(ctx, tagID, tenantID)
	if err != nil {
		return err
	}

	// Delete tag (repository handles document associations)
	if err := s.tagRepo.Delete(ctx, tagID); err != nil {
		return err
	}

	// Create audit log
	s.createAuditLog(ctx, tenantID, userID, tagID, models.AuditDelete, "Tag deleted: "+tag.Name)

	return nil
}

// GetTagSuggestions generates tag suggestions for text using keyword extraction
func (s *DocumentService) GetTagSuggestions(ctx context.Context, tenantID uuid.UUID, text string, limit int) ([]string, error) {
	// Extract keywords from text and match with existing tags
	keywords := s.extractKeywordsFromText(text)
	existingTags, err := s.tagRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing tags: %w", err)
	}

	var suggestions []string
	suggestionMap := make(map[string]bool)

	// Match keywords with existing tag names
	for _, keyword := range keywords {
		for _, tag := range existingTags {
			if strings.Contains(strings.ToLower(tag.Name), strings.ToLower(keyword)) {
				if !suggestionMap[tag.Name] {
					suggestions = append(suggestions, tag.Name)
					suggestionMap[tag.Name] = true
				}
			}
		}
	}

	// Add raw keywords as suggestions if they don't match existing tags
	for _, keyword := range keywords {
		if !suggestionMap[keyword] && len(keyword) > 2 {
			suggestions = append(suggestions, keyword)
			suggestionMap[keyword] = true
		}
	}

	// Limit suggestions
	if limit > 0 && len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions, nil
}

// BulkCreateTags creates multiple tags at once
func (s *DocumentService) BulkCreateTags(ctx context.Context, tenantID, userID uuid.UUID, tagNames []string) ([]models.Tag, error) {
	if len(tagNames) == 0 {
		return []models.Tag{}, nil
	}

	var tags []models.Tag
	var createdTags []models.Tag

	// Prepare tags for creation
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// Check if tag already exists
		if existingTag, err := s.tagRepo.GetByName(ctx, tenantID, name); err == nil && existingTag != nil {
			createdTags = append(createdTags, *existingTag)
			continue
		}

		tag := models.Tag{
			ID:            uuid.New(),
			TenantID:      tenantID,
			Name:          name,
			Color:         "#6B7280",
			IsAIGenerated: false,
			UsageCount:    0,
			CreatedAt:     time.Now(),
		}
		tags = append(tags, tag)
	}

	// Bulk create new tags
	if len(tags) > 0 {
		if err := s.tagRepo.BulkCreate(ctx, tags); err != nil {
			return nil, fmt.Errorf("failed to bulk create tags: %w", err)
		}
		createdTags = append(createdTags, tags...)

		// Create audit log for bulk creation
		s.createAuditLog(ctx, tenantID, userID, uuid.New(), models.AuditCreate,
			fmt.Sprintf("Bulk created %d tags", len(tags)))
	}

	return createdTags, nil
}

// Helper method to extract keywords from text for tag suggestions
func (s *DocumentService) extractKeywordsFromText(text string) []string {
	// Simple keyword extraction - split by common separators and filter
	text = strings.ToLower(text)
	separators := []string{" ", ",", ".", ";", ":", "\n", "\t", "-", "_", "(", ")", "[", "]", "{", "}"}

	words := []string{text}
	for _, sep := range separators {
		var newWords []string
		for _, word := range words {
			newWords = append(newWords, strings.Split(word, sep)...)
		}
		words = newWords
	}

	// Filter and clean keywords
	var keywords []string
	stopWords := map[string]bool{
		"the": true, "and": true, "or": true, "but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
		"a": true, "an": true, "as": true, "if": true, "it": true, "its": true, "this": true, "that": true,
	}

	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) > 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	// Remove duplicates and limit
	uniqueKeywords := make(map[string]bool)
	var result []string
	for _, keyword := range keywords {
		if !uniqueKeywords[keyword] {
			result = append(result, keyword)
			uniqueKeywords[keyword] = true
		}
		if len(result) >= 20 { // Limit to 20 keywords
			break
		}
	}

	return result
}

// CATEGORY MANAGEMENT METHODS

// CreateCategory creates a new category with validation
func (s *DocumentService) CreateCategory(ctx context.Context, tenantID, userID uuid.UUID, name, description, color, icon string, sortOrder int) (*models.Category, error) {
	// Validate name
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("category name cannot be empty")
	}

	// Check for duplicate names in tenant
	if existingCategory, err := s.categoryRepo.GetByName(ctx, tenantID, name); err == nil && existingCategory != nil {
		return nil, fmt.Errorf("category with name '%s' already exists", name)
	}

	// Set defaults
	if color == "" {
		color = "#6B7280"
	}
	if icon == "" {
		icon = "folder"
	}
	if sortOrder < 0 {
		sortOrder = 0
	}

	// Create category
	category := &models.Category{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        name,
		Description: description,
		Color:       color,
		Icon:        icon,
		IsSystem:    false,
		SortOrder:   sortOrder,
		CreatedAt:   time.Now(),
	}

	if err := s.categoryRepo.Create(ctx, category); err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, tenantID, userID, category.ID, models.AuditCreate, "Category created: "+name)

	return category, nil
}

// GetCategory retrieves a category with access control
func (s *DocumentService) GetCategory(ctx context.Context, categoryID, tenantID uuid.UUID) (*models.Category, error) {
	category, err := s.categoryRepo.GetByID(ctx, categoryID)
	if err != nil {
		return nil, fmt.Errorf("category not found")
	}

	// Verify tenant access
	if category.TenantID != tenantID {
		return nil, fmt.Errorf("unauthorized access to category")
	}

	return category, nil
}

// GetCategoryByName retrieves a category by name
func (s *DocumentService) GetCategoryByName(ctx context.Context, tenantID uuid.UUID, name string) (*models.Category, error) {
	return s.categoryRepo.GetByName(ctx, tenantID, name)
}

// ListCategories lists all categories for a tenant
func (s *DocumentService) ListCategories(ctx context.Context, tenantID uuid.UUID) ([]models.Category, error) {
	return s.categoryRepo.ListByTenant(ctx, tenantID)
}

// ListCategoriesWithDocumentCount lists categories with document counts
func (s *DocumentService) ListCategoriesWithDocumentCount(ctx context.Context, tenantID uuid.UUID) ([]CategoryWithCount, error) {
	categories, err := s.categoryRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var result []CategoryWithCount
	for _, category := range categories {
		count, err := s.categoryRepo.GetDocumentCount(ctx, category.ID)
		if err != nil {
			count = 0 // Continue with 0 count if we can't get the count
		}

		result = append(result, CategoryWithCount{
			Category:      category,
			DocumentCount: int(count),
		})
	}

	return result, nil
}

// GetSystemCategories gets system-defined categories
func (s *DocumentService) GetSystemCategories(ctx context.Context, tenantID uuid.UUID) ([]models.Category, error) {
	categories, err := s.categoryRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var systemCategories []models.Category
	for _, category := range categories {
		if category.IsSystem {
			systemCategories = append(systemCategories, category)
		}
	}

	return systemCategories, nil
}

// UpdateCategory updates an existing category
func (s *DocumentService) UpdateCategory(ctx context.Context, categoryID, tenantID uuid.UUID, updates map[string]interface{}, userID uuid.UUID) (*models.Category, error) {
	// Get existing category
	category, err := s.GetCategory(ctx, categoryID, tenantID)
	if err != nil {
		return nil, err
	}

	// Check if it's a system category
	if category.IsSystem {
		return nil, fmt.Errorf("cannot modify system category")
	}

	// Apply updates
	updated := false
	if name, ok := updates["name"].(string); ok && strings.TrimSpace(name) != "" {
		if name != category.Name {
			// Check for name conflicts
			if existingCategory, err := s.categoryRepo.GetByName(ctx, tenantID, name); err == nil && existingCategory != nil && existingCategory.ID != category.ID {
				return nil, fmt.Errorf("category with name '%s' already exists", name)
			}
			category.Name = name
			updated = true
		}
	}

	if description, ok := updates["description"].(string); ok {
		category.Description = description
		updated = true
	}

	if color, ok := updates["color"].(string); ok && color != "" {
		category.Color = color
		updated = true
	}

	if icon, ok := updates["icon"].(string); ok && icon != "" {
		category.Icon = icon
		updated = true
	}

	if sortOrder, ok := updates["sort_order"].(int); ok && sortOrder >= 0 {
		category.SortOrder = sortOrder
		updated = true
	}

	if updated {
		if err := s.categoryRepo.Update(ctx, category); err != nil {
			return nil, fmt.Errorf("failed to update category: %w", err)
		}

		// Create audit log
		s.createAuditLog(ctx, tenantID, userID, category.ID, models.AuditUpdate, "Category updated")
	}

	return category, nil
}

// DeleteCategory deletes a category with validation
func (s *DocumentService) DeleteCategory(ctx context.Context, categoryID, tenantID, userID uuid.UUID) error {
	// Get category first to check permissions
	category, err := s.GetCategory(ctx, categoryID, tenantID)
	if err != nil {
		return err
	}

	// Check if it's a system category
	if category.IsSystem {
		return fmt.Errorf("cannot delete system category")
	}

	// Delete category (repository handles document associations)
	if err := s.categoryRepo.Delete(ctx, categoryID); err != nil {
		return err
	}

	// Create audit log
	s.createAuditLog(ctx, tenantID, userID, categoryID, models.AuditDelete, "Category deleted: "+category.Name)

	return nil
}

// Helper struct for categories with document counts
type CategoryWithCount struct {
	models.Category
	DocumentCount int `json:"document_count"`
}

// GetDocumentAIResults retrieves AI processing results for a document
func (s *DocumentService) GetDocumentAIResults(ctx context.Context, documentID, tenantID uuid.UUID) (*AIResultsResponse, error) {
	// Get completed AI jobs for the document
	jobs, err := s.aiJobRepo.ListByDocument(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI jobs: %w", err)
	}

	result := &AIResultsResponse{
		DocumentID: documentID,
		HasResults: false,
	}

	// Process each completed job and extract results
	for _, job := range jobs {
		if job.Status != models.ProcessingCompleted || job.Result == nil {
			continue
		}

		result.HasResults = true
		if job.CompletedAt != nil {
			result.ProcessedAt = job.CompletedAt
		}

		// Extract results based on job type
		switch job.JobType {
		case JobTypeDocumentSummarization:
			if summary, ok := job.Result["summary"].(string); ok && summary != "" {
				result.Summary = &summary
			}

		case JobTypeEntityExtraction:
			if entities, ok := job.Result["entities"].(map[string]interface{}); ok {
				result.Entities = entities
			}

		case JobTypeDocumentClassification:
			if docType, ok := job.Result["document_type"].(string); ok {
				if confidence, ok := job.Result["confidence"].(float64); ok {
					reasoning, _ := job.Result["reasoning"].(string)
					result.Classification = &DocumentClassification{
						Type:       models.DocumentType(docType),
						Confidence: confidence,
						Reasoning:  reasoning,
					}
				}
			}

		case JobTypeSemanticAnalysis:
			if tags, ok := job.Result["tags"].([]interface{}); ok {
				var tagStrings []string
				for _, tag := range tags {
					if tagStr, ok := tag.(string); ok {
						tagStrings = append(tagStrings, tagStr)
					}
				}
				result.Tags = tagStrings
			}
		}
	}

	return result, nil
}

// GetDocumentJobs retrieves AI processing jobs for a document
func (s *DocumentService) GetDocumentJobs(ctx context.Context, documentID, tenantID uuid.UUID) ([]models.AIProcessingJob, error) {
	jobs, err := s.aiJobRepo.ListByDocument(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI jobs: %w", err)
	}

	// Filter jobs to only include AI processing jobs (not file processing)
	var aiJobs []models.AIProcessingJob
	aiJobTypes := map[string]bool{
		JobTypeDocumentSummarization:  true,
		JobTypeEntityExtraction:       true,
		JobTypeDocumentClassification: true,
		JobTypeSemanticAnalysis:       true,
		JobTypeEmbeddingGeneration:    true,
		"financial_extraction":        true,
		"ocr_extraction":              true,
	}

	for _, job := range jobs {
		if aiJobTypes[job.JobType] {
			aiJobs = append(aiJobs, job)
		}
	}

	return aiJobs, nil
}

// Response types for AI processing
type AIResultsResponse struct {
	DocumentID     uuid.UUID               `json:"document_id"`
	Summary        *string                 `json:"summary,omitempty"`
	Entities       map[string]interface{}  `json:"entities,omitempty"`
	Classification *DocumentClassification `json:"classification,omitempty"`
	Tags           []string                `json:"tags,omitempty"`
	ProcessedAt    *time.Time              `json:"processed_at,omitempty"`
	HasResults     bool                    `json:"has_results"`
}

type DocumentClassification struct {
	Type       models.DocumentType `json:"type"`
	Confidence float64             `json:"confidence"`
	Reasoning  string              `json:"reasoning,omitempty"`
}
