package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
)

var (
	ErrAIServiceUnavailable = errors.New("AI service unavailable")
	ErrInvalidFileFormat    = errors.New("invalid file format for AI processing")
	ErrProcessingTimeout    = errors.New("AI processing timeout")
	ErrInsufficientCredits  = errors.New("insufficient AI credits")
)

// AIProcessingService orchestrates AI-powered document analysis
type AIProcessingService struct {
	aiJobRepo    repositories.AIProcessingJobRepository
	documentRepo repositories.DocumentRepository
	tagRepo      repositories.TagRepository
	categoryRepo repositories.CategoryRepository
	tenantRepo   repositories.TenantRepository
	auditRepo    repositories.AuditLogRepository

	openAIService  OpenAIService
	ocrService     OCRService
	storageService StorageService
	config         AIServiceConfig
}

// AIServiceConfig holds configuration for AI processing
type AIServiceConfig struct {
	OpenAIAPIKey             string
	MaxConcurrentJobs        int
	ProcessingTimeout        time.Duration
	EnableSemanticSearch     bool
	EnableAutoTagging        bool
	EnableAutoClassification bool
	DefaultModel             string
	EmbeddingModel           string
	MaxTokens                int
	Temperature              float64
}

// NewAIProcessingService creates a new AI processing service
func NewAIProcessingService(
	aiJobRepo repositories.AIProcessingJobRepository,
	documentRepo repositories.DocumentRepository,
	tagRepo repositories.TagRepository,
	categoryRepo repositories.CategoryRepository,
	tenantRepo repositories.TenantRepository,
	auditRepo repositories.AuditLogRepository,
	openAIService OpenAIService,
	ocrService OCRService,
	storageService StorageService,
	config AIServiceConfig,
) *AIProcessingService {
	return &AIProcessingService{
		aiJobRepo:      aiJobRepo,
		documentRepo:   documentRepo,
		tagRepo:        tagRepo,
		categoryRepo:   categoryRepo,
		tenantRepo:     tenantRepo,
		auditRepo:      auditRepo,
		openAIService:  openAIService,
		ocrService:     ocrService,
		storageService: storageService,
		config:         config,
	}
}

// ProcessNextJob processes the next available AI job
func (s *AIProcessingService) ProcessNextJob(ctx context.Context) error {
	// Get next job from queue
	job, err := s.aiJobRepo.GetNextJob(ctx)
	if err != nil {
		return fmt.Errorf("failed to get next job: %w", err)
	}

	if job == nil {
		return nil // No jobs to process
	}

	// Check tenant quota
	quotaStatus, err := s.tenantRepo.CheckQuotaLimits(ctx, job.TenantID)
	if err != nil {
		return fmt.Errorf("failed to check quota: %w", err)
	}

	if !quotaStatus.CanProcessAI {
		s.failJob(ctx, job, "AI quota exceeded")
		return ErrInsufficientCredits
	}

	// Mark job as started
	job.Status = models.ProcessingInProgress
	startTime := time.Now()
	job.StartedAt = &startTime
	job.Attempts++

	if err := s.aiJobRepo.Update(ctx, job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Process the job
	err = s.processJob(ctx, job)

	// Update job completion status
	endTime := time.Now()
	job.ProcessingTimeMs = int(endTime.Sub(startTime).Milliseconds())
	job.CompletedAt = &endTime

	if err != nil {
		job.Status = models.ProcessingFailed
		job.ErrorMessage = err.Error()

		// Retry if under max attempts
		if job.Attempts < job.MaxAttempts {
			job.Status = models.ProcessingQueued
		}
	} else {
		job.Status = models.ProcessingCompleted
	}

	s.aiJobRepo.Update(ctx, job)

	// Update tenant API usage
	s.tenantRepo.UpdateUsage(ctx, job.TenantID, 0, 1)

	return err
}

// processJob handles the actual AI processing based on job type
func (s *AIProcessingService) processJob(ctx context.Context, job *models.AIProcessingJob) error {
	// Get document
	document, err := s.documentRepo.GetByID(ctx, job.DocumentID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Download file content
	fileContent, err := s.storageService.Get(ctx, document.StoragePath)
	if err != nil {
		return fmt.Errorf("failed to get file content: %w", err)
	}
	defer fileContent.Close()

	switch job.JobType {
	case "text_extraction":
		return s.processTextExtraction(ctx, job, document, fileContent)
	case "ocr":
		return s.processOCR(ctx, job, document, fileContent)
	case "categorization":
		return s.processDocumentClassification(ctx, job, document)
	case "tagging":
		return s.processAutoTagging(ctx, job, document)
	case "financial_extraction":
		return s.processFinancialExtraction(ctx, job, document)
	case "summarization":
		return s.processSummarization(ctx, job, document)
	case "entity_extraction":
		return s.processEntityExtraction(ctx, job, document)
	case "embedding_generation":
		return s.processEmbeddingGeneration(ctx, job, document)
	default:
		return fmt.Errorf("unknown job type: %s", job.JobType)
	}
}

// processTextExtraction extracts text from documents
func (s *AIProcessingService) processTextExtraction(ctx context.Context, job *models.AIProcessingJob, document *models.Document, fileContent io.ReadCloser) error {
	var extractedText string
	var err error

	// Choose extraction method based on file type
	switch document.ContentType {
	case "application/pdf":
		extractedText, err = s.extractTextFromPDF(fileContent)
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		extractedText, err = s.extractTextFromDocx(fileContent)
	case "text/plain":
		extractedText, err = s.extractTextFromPlain(fileContent)
	default:
		// Try OCR for image formats
		extractedText, err = s.ocrService.ExtractText(ctx, document.StoragePath)
	}

	if err != nil {
		return fmt.Errorf("text extraction failed: %w", err)
	}

	// Update document with extracted text
	document.ExtractedText = extractedText
	if err := s.documentRepo.Update(ctx, document); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	// Store result in job
	job.Result = models.JSONB{
		"extracted_text": extractedText,
		"text_length":    len(extractedText),
	}

	return nil
}

// processOCR performs OCR on image documents
func (s *AIProcessingService) processOCR(ctx context.Context, job *models.AIProcessingJob, document *models.Document, fileContent io.ReadCloser) error {
	ocrText, err := s.ocrService.ExtractText(ctx, document.StoragePath)
	if err != nil {
		return fmt.Errorf("OCR failed: %w", err)
	}

	// Update document with OCR text
	document.OCRText = ocrText
	if err := s.documentRepo.Update(ctx, document); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	job.Result = models.JSONB{
		"ocr_text":    ocrText,
		"text_length": len(ocrText),
	}

	return nil
}

// processDocumentClassification classifies documents using AI
func (s *AIProcessingService) processDocumentClassification(ctx context.Context, job *models.AIProcessingJob, document *models.Document) error {
	// Get text content for classification
	text := s.getDocumentText(document)
	if text == "" {
		return errors.New("no text available for classification")
	}

	// Use AI to classify document
	docType, confidence, err := s.openAIService.ClassifyDocument(ctx, text)
	if err != nil {
		return fmt.Errorf("classification failed: %w", err)
	}

	// Update document if confidence is high enough
	if confidence > 0.7 {
		document.DocumentType = docType
		document.AIConfidence = confidence

		if err := s.documentRepo.Update(ctx, document); err != nil {
			return fmt.Errorf("failed to update document: %w", err)
		}
	}

	job.Result = models.JSONB{
		"document_type": string(docType),
		"confidence":    confidence,
		"applied":       confidence > 0.7,
	}

	return nil
}

// processAutoTagging generates and applies tags using AI
func (s *AIProcessingService) processAutoTagging(ctx context.Context, job *models.AIProcessingJob, document *models.Document) error {
	text := s.getDocumentText(document)
	if text == "" {
		return errors.New("no text available for tagging")
	}

	// Generate tags using AI
	suggestedTags, err := s.openAIService.GenerateTags(ctx, text)
	if err != nil {
		return fmt.Errorf("tag generation failed: %w", err)
	}

	// Create or get existing tags
	var createdTags []string
	for _, tagName := range suggestedTags {
		// Clean and validate tag name
		cleanTag := s.cleanTagName(tagName)
		if cleanTag == "" || len(cleanTag) > 50 {
			continue
		}

		// Get or create tag
		tag, err := s.tagRepo.GetByName(ctx, document.TenantID, cleanTag)
		if err != nil {
			// Create new tag
			newTag := &models.Tag{
				TenantID:      document.TenantID,
				Name:          cleanTag,
				IsAIGenerated: true,
			}

			if err := s.tagRepo.Create(ctx, newTag); err != nil {
				continue // Skip this tag if creation fails
			}
			createdTags = append(createdTags, cleanTag)
		} else {
			// Increment usage for existing tag
			s.tagRepo.IncrementUsage(ctx, tag.ID)
			createdTags = append(createdTags, cleanTag)
		}
	}

	job.Result = models.JSONB{
		"suggested_tags": suggestedTags,
		"created_tags":   createdTags,
		"tag_count":      len(createdTags),
	}

	return nil
}

// processFinancialExtraction extracts financial data from documents
func (s *AIProcessingService) processFinancialExtraction(ctx context.Context, job *models.AIProcessingJob, document *models.Document) error {
	text := s.getDocumentText(document)
	if text == "" {
		return errors.New("no text available for financial extraction")
	}

	// Extract financial data using AI
	financialData, err := s.openAIService.ExtractFinancialData(ctx, text, document.DocumentType)
	if err != nil {
		return fmt.Errorf("financial extraction failed: %w", err)
	}

	// Apply extracted data to document
	s.applyFinancialData(document, financialData)

	// Update document
	if err := s.documentRepo.Update(ctx, document); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	job.Result = models.JSONB(financialData)

	return nil
}

// processSummarization generates document summaries
func (s *AIProcessingService) processSummarization(ctx context.Context, job *models.AIProcessingJob, document *models.Document) error {
	text := s.getDocumentText(document)
	if text == "" {
		return errors.New("no text available for summarization")
	}

	// Generate summary using AI
	summary, err := s.openAIService.GenerateSummary(ctx, text)
	if err != nil {
		return fmt.Errorf("summarization failed: %w", err)
	}

	// Update document with summary
	document.Summary = summary
	if err := s.documentRepo.Update(ctx, document); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	job.Result = models.JSONB{
		"summary":           summary,
		"summary_length":    len(summary),
		"compression_ratio": float64(len(summary)) / float64(len(text)),
	}

	return nil
}

// processEntityExtraction extracts entities from documents
func (s *AIProcessingService) processEntityExtraction(ctx context.Context, job *models.AIProcessingJob, document *models.Document) error {
	text := s.getDocumentText(document)
	if text == "" {
		return errors.New("no text available for entity extraction")
	}

	// Extract entities using AI
	entities, err := s.openAIService.ExtractEntities(ctx, text)
	if err != nil {
		return fmt.Errorf("entity extraction failed: %w", err)
	}

	// Store extracted entities in document
	if document.ExtractedData == nil {
		document.ExtractedData = make(models.JSONB)
	}
	document.ExtractedData["entities"] = entities

	if err := s.documentRepo.Update(ctx, document); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	job.Result = models.JSONB{
		"entities":     entities,
		"entity_count": len(entities),
	}

	return nil
}

// processEmbeddingGeneration generates vector embeddings for semantic search
func (s *AIProcessingService) processEmbeddingGeneration(ctx context.Context, job *models.AIProcessingJob, document *models.Document) error {
	text := s.getDocumentText(document)
	if text == "" {
		return errors.New("no text available for embedding generation")
	}

	// Generate embedding using AI
	embedding, err := s.openAIService.GenerateEmbedding(ctx, text)
	if err != nil {
		return fmt.Errorf("embedding generation failed: %w", err)
	}

	// Update document with embedding
	// Note: You'll need to convert []float32 to pgvector.Vector
	// document.Embedding = pgvector.NewVector(embedding)
	if err := s.documentRepo.Update(ctx, document); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	job.Result = models.JSONB{
		"embedding_dimensions": len(embedding),
		"generated":            true,
	}

	return nil
}

// QueueDocumentProcessing queues AI processing jobs for a document
func (s *AIProcessingService) QueueDocumentProcessing(ctx context.Context, documentID uuid.UUID, jobTypes []string) error {
	document, err := s.documentRepo.GetByID(ctx, documentID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	for i, jobType := range jobTypes {
		job := &models.AIProcessingJob{
			TenantID:   document.TenantID,
			DocumentID: documentID,
			JobType:    jobType,
			Priority:   5 - i, // Earlier jobs get higher priority
		}

		if err := s.aiJobRepo.Create(ctx, job); err != nil {
			return fmt.Errorf("failed to queue job %s: %w", jobType, err)
		}
	}

	return nil
}

// GetRecommendedJobs returns recommended AI processing jobs for a document
func (s *AIProcessingService) GetRecommendedJobs(document *models.Document) []string {
	var jobs []string

	// Always recommend text extraction if not done
	if document.ExtractedText == "" {
		jobs = append(jobs, "text_extraction")
	}

	// Recommend OCR for image formats
	if s.isImageFormat(document.ContentType) && document.OCRText == "" {
		jobs = append(jobs, "ocr")
	}

	// Recommend classification if confidence is low
	if document.AIConfidence < 0.7 {
		jobs = append(jobs, "categorization")
	}

	// Recommend tagging for all documents
	jobs = append(jobs, "tagging")

	// Recommend financial extraction for financial documents
	if s.isFinancialDocument(document.DocumentType) {
		jobs = append(jobs, "financial_extraction")
	}

	// Recommend summarization for large documents
	if len(document.ExtractedText) > 1000 && document.Summary == "" {
		jobs = append(jobs, "summarization")
	}

	// Recommend embedding generation for semantic search
	if s.config.EnableSemanticSearch {
		jobs = append(jobs, "embedding_generation")
	}

	return jobs
}

// ProcessBatch processes multiple documents in batch
func (s *AIProcessingService) ProcessBatch(ctx context.Context, documentIDs []uuid.UUID, jobTypes []string) error {
	for _, documentID := range documentIDs {
		if err := s.QueueDocumentProcessing(ctx, documentID, jobTypes); err != nil {
			// Log error but continue with other documents
			continue
		}
	}
	return nil
}

// Helper methods

func (s *AIProcessingService) getDocumentText(document *models.Document) string {
	if document.ExtractedText != "" {
		return document.ExtractedText
	}
	if document.OCRText != "" {
		return document.OCRText
	}
	return ""
}

func (s *AIProcessingService) cleanTagName(tag string) string {
	// Clean and normalize tag names
	tag = strings.TrimSpace(tag)
	tag = strings.ToLower(tag)

	// Remove special characters
	reg := regexp.MustCompile(`[^a-z0-9\s-_]`)
	tag = reg.ReplaceAllString(tag, "")

	// Replace multiple spaces with single space
	reg = regexp.MustCompile(`\s+`)
	tag = reg.ReplaceAllString(tag, " ")

	return strings.TrimSpace(tag)
}

func (s *AIProcessingService) applyFinancialData(document *models.Document, data map[string]interface{}) {
	if amount, ok := data["amount"].(float64); ok {
		document.Amount = &amount
	}

	if currency, ok := data["currency"].(string); ok {
		document.Currency = currency
	}

	if taxAmount, ok := data["tax_amount"].(float64); ok {
		document.TaxAmount = &taxAmount
	}

	if vendor, ok := data["vendor_name"].(string); ok {
		document.VendorName = vendor
	}

	if customer, ok := data["customer_name"].(string); ok {
		document.CustomerName = customer
	}

	if docNumber, ok := data["document_number"].(string); ok {
		document.DocumentNumber = docNumber
	}

	// Parse dates
	if dateStr, ok := data["document_date"].(string); ok {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			document.DocumentDate = &date
		}
	}

	if dueDateStr, ok := data["due_date"].(string); ok {
		if date, err := time.Parse("2006-01-02", dueDateStr); err == nil {
			document.DueDate = &date
		}
	}

	// Store all extracted data
	if document.ExtractedData == nil {
		document.ExtractedData = make(models.JSONB)
	}
	document.ExtractedData["financial_data"] = data
}

func (s *AIProcessingService) isImageFormat(contentType string) bool {
	imageTypes := []string{
		"image/jpeg", "image/jpg", "image/png", "image/tiff", "image/bmp", "image/gif",
	}

	for _, imageType := range imageTypes {
		if contentType == imageType {
			return true
		}
	}
	return false
}

func (s *AIProcessingService) isFinancialDocument(docType models.DocumentType) bool {
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

func (s *AIProcessingService) failJob(ctx context.Context, job *models.AIProcessingJob, reason string) {
	job.Status = models.ProcessingFailed
	job.ErrorMessage = reason
	s.aiJobRepo.Update(ctx, job)
}

// Text extraction helper methods (simplified implementations)
func (s *AIProcessingService) extractTextFromPDF(reader io.ReadCloser) (string, error) {
	// Implementation would use a PDF parsing library
	return "", nil
}

func (s *AIProcessingService) extractTextFromDocx(reader io.ReadCloser) (string, error) {
	// Implementation would use a DOCX parsing library
	return "", nil
}

func (s *AIProcessingService) extractTextFromPlain(reader io.ReadCloser) (string, error) {
	// Read plain text file
	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// External service interfaces

type OpenAIService interface {
	ExtractText(ctx context.Context, text string) (string, error)
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GenerateSummary(ctx context.Context, text string) (string, error)
	ExtractEntities(ctx context.Context, text string) (map[string]interface{}, error)
	ClassifyDocument(ctx context.Context, text string) (models.DocumentType, float64, error)
	GenerateTags(ctx context.Context, text string) ([]string, error)
	ExtractFinancialData(ctx context.Context, text string, docType models.DocumentType) (map[string]interface{}, error)
}

type OCRService interface {
	ExtractText(ctx context.Context, imagePath string) (string, error)
	GetConfidence(ctx context.Context, imagePath string) (float64, error)
}

// External service interfaces are now defined in external_interfaces.go
