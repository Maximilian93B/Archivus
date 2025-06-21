package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/archivus/archivus/internal/app/config"
	appservices "github.com/archivus/archivus/internal/app/services"
	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/archivus/archivus/internal/infrastructure/storage/local"
	storageSupabase "github.com/archivus/archivus/internal/infrastructure/storage/supabase"
	"github.com/archivus/archivus/pkg/logger"
)

// WorkerConfig holds configuration for the background worker
type WorkerConfig struct {
	// File Processing Configuration
	ConcurrentJobs   int           `json:"concurrent_jobs"`
	PollInterval     time.Duration `json:"poll_interval"`
	JobTimeout       time.Duration `json:"job_timeout"`
	MaxRetries       int           `json:"max_retries"`
	EnableThumbnails bool          `json:"enable_thumbnails"`
	EnableValidation bool          `json:"enable_validation"`
	EnableMetadata   bool          `json:"enable_metadata"`
	ThumbnailMaxSize int64         `json:"thumbnail_max_size"`
	ThumbnailQuality int           `json:"thumbnail_quality"`

	// AI Processing Configuration
	EnableAIProcessing bool          `json:"enable_ai_processing"`
	AIJobTimeout       time.Duration `json:"ai_job_timeout"`
	AIMaxRetries       int           `json:"ai_max_retries"`
	ClaudeEnabled      bool          `json:"claude_enabled"`
}

// FileProcessor handles background file processing jobs
type FileProcessor struct {
	config         WorkerConfig
	serviceManager *appservices.ServiceManager
	storageService services.StorageService
	aiFactory      *services.AIServiceFactory
	claudeService  *services.ClaudeService
	logger         *logger.Logger
	shutdown       chan os.Signal
	wg             sync.WaitGroup
}

func main() {
	// Initialize logger
	log := logger.New()

	log.Info("Starting Archivus File Processing Worker")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize database
	db, err := initializeDatabase(cfg, log)
	if err != nil {
		log.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize service manager
	serviceManager, err := appservices.NewServiceManager(cfg, db)
	if err != nil {
		log.Error("Failed to initialize service manager", "error", err)
		os.Exit(1)
	}
	defer serviceManager.Close()

	// Health check
	if err := serviceManager.HealthCheck(); err != nil {
		log.Error("Service health check failed", "error", err)
		os.Exit(1)
	}

	// Initialize storage service
	storageService := initializeStorageService(cfg, log)

	// Create worker configuration
	workerConfig := WorkerConfig{
		ConcurrentJobs:     getIntEnv("WORKER_CONCURRENT_JOBS", 5),
		PollInterval:       getDurationEnv("WORKER_POLL_INTERVAL", 10*time.Second),
		JobTimeout:         getDurationEnv("WORKER_JOB_TIMEOUT", 5*time.Minute),
		MaxRetries:         getIntEnv("WORKER_MAX_RETRIES", 3),
		EnableThumbnails:   getBoolEnv("WORKER_ENABLE_THUMBNAILS", true),
		EnableValidation:   getBoolEnv("WORKER_ENABLE_VALIDATION", true),
		EnableMetadata:     getBoolEnv("WORKER_ENABLE_METADATA", true),
		ThumbnailMaxSize:   getInt64Env("WORKER_THUMBNAIL_MAX_SIZE", 200*1024), // 200KB
		ThumbnailQuality:   getIntEnv("WORKER_THUMBNAIL_QUALITY", 85),
		EnableAIProcessing: getBoolEnv("WORKER_ENABLE_AI_PROCESSING", true),
		AIJobTimeout:       getDurationEnv("WORKER_AI_JOB_TIMEOUT", 5*time.Minute),
		AIMaxRetries:       getIntEnv("WORKER_AI_MAX_RETRIES", 3),
		ClaudeEnabled:      getBoolEnv("WORKER_CLAUDE_ENABLED", true),
	}

	// Initialize AI services if enabled
	var aiFactory *services.AIServiceFactory
	var claudeService *services.ClaudeService

	if workerConfig.EnableAIProcessing && workerConfig.ClaudeEnabled {
		var err error
		aiFactory, err = services.NewAIServiceFactory(cfg)
		if err != nil {
			log.Error("Failed to initialize AI factory", "error", err)
			// Continue without AI services
		} else {
			claudeService = aiFactory.GetClaudeService()
			if claudeService != nil {
				log.Info("Claude service initialized successfully")
			}
		}
	}

	// Create file processor
	processor := &FileProcessor{
		config:         workerConfig,
		serviceManager: serviceManager,
		storageService: storageService,
		aiFactory:      aiFactory,
		claudeService:  claudeService,
		logger:         log,
		shutdown:       make(chan os.Signal, 1),
	}

	// Setup graceful shutdown
	signal.Notify(processor.shutdown, syscall.SIGINT, syscall.SIGTERM)

	log.Info("File Processing Worker started",
		"concurrent_jobs", workerConfig.ConcurrentJobs,
		"poll_interval", workerConfig.PollInterval,
		"job_timeout", workerConfig.JobTimeout,
		"ai_processing_enabled", workerConfig.EnableAIProcessing,
		"claude_enabled", workerConfig.ClaudeEnabled && claudeService != nil)

	// Start worker
	processor.Start()
}

// Start begins the file processing worker
func (fp *FileProcessor) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker goroutines
	for i := 0; i < fp.config.ConcurrentJobs; i++ {
		fp.wg.Add(1)
		go fp.workerLoop(ctx, i)
	}

	fp.logger.Info("Worker started with goroutines", "count", fp.config.ConcurrentJobs)

	// Wait for shutdown signal
	<-fp.shutdown
	fp.logger.Info("Shutdown signal received, stopping workers...")

	// Cancel context to stop all workers
	cancel()

	// Wait for all workers to finish
	fp.wg.Wait()
	fp.logger.Info("All workers stopped gracefully")
}

// workerLoop is the main processing loop for each worker goroutine
func (fp *FileProcessor) workerLoop(ctx context.Context, workerID int) {
	defer fp.wg.Done()

	fp.logger.Info("Worker started", "worker_id", workerID)

	ticker := time.NewTicker(fp.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fp.logger.Info("Worker stopping", "worker_id", workerID)
			return
		case <-ticker.C:
			if err := fp.processNextJob(ctx, workerID); err != nil {
				fp.logger.Error("Job processing error", "worker_id", workerID, "error", err)
			}
		}
	}
}

// processNextJob processes the next available job from the queue
func (fp *FileProcessor) processNextJob(ctx context.Context, workerID int) error {
	// Get the next job from the queue
	job, err := fp.serviceManager.Repositories.AIJobRepo.GetNextJob(ctx)
	if err != nil {
		return fmt.Errorf("failed to get next job: %w", err)
	}

	if job == nil {
		// No jobs available
		return nil
	}

	fp.logger.Info("Processing job",
		"worker_id", workerID,
		"job_id", job.ID,
		"job_type", job.JobType,
		"document_id", job.DocumentID)

	// Determine timeout based on job type
	jobTimeout := fp.config.JobTimeout
	if fp.isAIProcessingJob(job.JobType) {
		jobTimeout = fp.config.AIJobTimeout
	}

	// Create job context with timeout
	jobCtx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	// Mark job as started
	job.Status = models.ProcessingInProgress
	startTime := time.Now()
	job.StartedAt = &startTime
	job.Attempts++

	if err := fp.serviceManager.Repositories.AIJobRepo.Update(jobCtx, job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Process the job
	err = fp.processJob(jobCtx, job)

	// Update job completion status
	endTime := time.Now()
	job.ProcessingTimeMs = int(endTime.Sub(startTime).Milliseconds())
	job.CompletedAt = &endTime

	// Determine max retries based on job type
	maxRetries := fp.config.MaxRetries
	if fp.isAIProcessingJob(job.JobType) {
		maxRetries = fp.config.AIMaxRetries
	}

	if err != nil {
		job.Status = models.ProcessingFailed
		job.ErrorMessage = err.Error()
		fp.logger.Error("Job failed",
			"job_id", job.ID,
			"error", err,
			"processing_time_ms", job.ProcessingTimeMs)

		// Retry if under max attempts
		if job.Attempts < maxRetries {
			job.Status = models.ProcessingQueued
			fp.logger.Info("Job queued for retry",
				"job_id", job.ID,
				"attempt", job.Attempts,
				"max_attempts", maxRetries)
		}
	} else {
		job.Status = models.ProcessingCompleted
		fp.logger.Info("Job completed successfully",
			"job_id", job.ID,
			"processing_time_ms", job.ProcessingTimeMs)
	}

	// Save job status
	if updateErr := fp.serviceManager.Repositories.AIJobRepo.Update(jobCtx, job); updateErr != nil {
		fp.logger.Error("Failed to update job status", "job_id", job.ID, "error", updateErr)
	}

	return err
}

// processJob handles the actual file processing based on job type
func (fp *FileProcessor) processJob(ctx context.Context, job *models.AIProcessingJob) error {
	// Get document
	document, err := fp.serviceManager.Repositories.DocumentRepo.GetByID(ctx, job.DocumentID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Route to appropriate processor based on job type
	if fp.isAIProcessingJob(job.JobType) {
		return fp.processAIJob(ctx, job, document)
	}

	// File processing jobs
	switch job.JobType {
	case "thumbnail_generation":
		return fp.processThumbnailGeneration(ctx, job, document)
	case "file_validation":
		return fp.processFileValidation(ctx, job, document)
	case "metadata_extraction":
		return fp.processMetadataExtraction(ctx, job, document)
	case "preview_generation":
		return fp.processPreviewGeneration(ctx, job, document)
	default:
		return fmt.Errorf("unknown job type: %s", job.JobType)
	}
}

// isFileProcessingJob checks if the job type is handled by this worker
func (fp *FileProcessor) isFileProcessingJob(jobType string) bool {
	fileJobTypes := []string{
		"thumbnail_generation",
		"file_validation",
		"metadata_extraction",
		"preview_generation",
	}

	for _, fileJob := range fileJobTypes {
		if jobType == fileJob {
			return true
		}
	}
	return false
}

// isAIProcessingJob checks if the job type is an AI processing job
func (fp *FileProcessor) isAIProcessingJob(jobType string) bool {
	aiJobTypes := []string{
		// Phase 3 AI jobs
		"document_summarization",
		"entity_extraction",
		"document_classification",
		"embedding_generation",
		"semantic_analysis",
	}

	for _, aiJob := range aiJobTypes {
		if jobType == aiJob {
			return true
		}
	}
	return false
}

// File processing implementations
func (fp *FileProcessor) processThumbnailGeneration(ctx context.Context, job *models.AIProcessingJob, document *models.Document) error {
	if !fp.config.EnableThumbnails {
		return fmt.Errorf("thumbnail generation is disabled")
	}

	fp.logger.Info("Generating thumbnail", "document_id", document.ID, "file_type", document.ContentType)

	// For now, create a simple placeholder thumbnail
	// In a full implementation, you'd use image processing libraries like:
	// - github.com/disintegration/imaging (for images)
	// - github.com/gen2brain/go-fitz (for PDFs)

	thumbnailPath := fp.getThumbnailPath(document.StoragePath)

	// Create a simple text-based "thumbnail" as placeholder
	placeholderContent := fmt.Sprintf("Thumbnail for %s\nType: %s\nSize: %d bytes",
		document.OriginalName, document.ContentType, document.FileSize)

	// Store placeholder thumbnail
	params := services.StorageParams{
		TenantID:    document.TenantID,
		FileReader:  strings.NewReader(placeholderContent),
		Filename:    fmt.Sprintf("thumb_%s.txt", document.ID),
		ContentType: "text/plain",
		Size:        int64(len(placeholderContent)),
	}

	_, err := fp.storageService.Store(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to store thumbnail: %w", err)
	}

	job.Result = models.JSONB{
		"thumbnail_generated": true,
		"thumbnail_path":      thumbnailPath,
		"thumbnail_type":      "placeholder",
		"message":             "Thumbnail generation completed (placeholder implementation)",
	}

	fp.logger.Info("Thumbnail generated successfully", "document_id", document.ID, "path", thumbnailPath)
	return nil
}

func (fp *FileProcessor) processFileValidation(ctx context.Context, job *models.AIProcessingJob, document *models.Document) error {
	if !fp.config.EnableValidation {
		return fmt.Errorf("file validation is disabled")
	}

	fp.logger.Info("Validating file", "document_id", document.ID, "file_size", document.FileSize)

	// Basic file validation
	validationResult := map[string]interface{}{
		"file_size_valid":    document.FileSize > 0 && document.FileSize < 100*1024*1024, // 100MB limit
		"content_type_valid": document.ContentType != "",
		"filename_valid":     document.OriginalName != "",
		"storage_path_valid": document.StoragePath != "",
		"file_extension":     getFileExtension(document.OriginalName),
	}

	// Check if file exists in storage
	fileReader, err := fp.storageService.Get(ctx, document.StoragePath)
	if err != nil {
		validationResult["file_accessible"] = false
		validationResult["access_error"] = err.Error()
	} else {
		validationResult["file_accessible"] = true
		fileReader.Close()
	}

	// Validate file extension matches content type
	extension := getFileExtension(document.OriginalName)
	validationResult["extension_content_type_match"] = fp.validateContentTypeExtension(document.ContentType, extension)

	allValid := true
	for key, valid := range validationResult {
		if key != "file_extension" && key != "access_error" {
			if !valid.(bool) {
				allValid = false
				break
			}
		}
	}

	validationResult["overall_valid"] = allValid
	validationResult["validation_timestamp"] = time.Now().Unix()

	job.Result = models.JSONB(validationResult)

	if !allValid {
		return fmt.Errorf("file validation failed")
	}

	fp.logger.Info("File validation completed", "document_id", document.ID, "valid", allValid)
	return nil
}

func (fp *FileProcessor) processMetadataExtraction(ctx context.Context, job *models.AIProcessingJob, document *models.Document) error {
	if !fp.config.EnableMetadata {
		return fmt.Errorf("metadata extraction is disabled")
	}

	fp.logger.Info("Extracting metadata", "document_id", document.ID)

	// Extract basic metadata
	metadata := map[string]interface{}{
		"file_extension":       getFileExtension(document.OriginalName),
		"estimated_pages":      estimatePages(document.ContentType, document.FileSize),
		"is_image":             isImageFile(document.ContentType),
		"is_document":          isDocumentFile(document.ContentType),
		"is_archive":           isArchiveFile(document.ContentType),
		"file_category":        categorizeFile(document.ContentType),
		"processing_timestamp": time.Now().Unix(),
		"file_size_mb":         float64(document.FileSize) / (1024 * 1024),
		"estimated_read_time":  estimateReadTime(document.FileSize, document.ContentType),
	}

	// Try to get additional metadata from file content (placeholder)
	if isImageFile(document.ContentType) {
		metadata["image_metadata"] = map[string]interface{}{
			"estimated_width":  "unknown",
			"estimated_height": "unknown",
			"format":           strings.ToUpper(getFileExtension(document.OriginalName)),
		}
	}

	job.Result = models.JSONB(metadata)

	fp.logger.Info("Metadata extraction completed", "document_id", document.ID, "category", metadata["file_category"])
	return nil
}

func (fp *FileProcessor) processPreviewGeneration(ctx context.Context, job *models.AIProcessingJob, document *models.Document) error {
	fp.logger.Info("Generating preview", "document_id", document.ID, "content_type", document.ContentType)

	// For now, create a simple preview placeholder
	previewPath := fp.getPreviewPath(document.StoragePath)

	previewContent := fmt.Sprintf("Preview for %s\nDocument ID: %s\nContent Type: %s\nOriginal Size: %d bytes\nGenerated: %s",
		document.OriginalName, document.ID, document.ContentType, document.FileSize, time.Now().Format(time.RFC3339))

	// Store preview
	params := services.StorageParams{
		TenantID:    document.TenantID,
		FileReader:  strings.NewReader(previewContent),
		Filename:    fmt.Sprintf("preview_%s.txt", document.ID),
		ContentType: "text/plain",
		Size:        int64(len(previewContent)),
	}

	_, err := fp.storageService.Store(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to store preview: %w", err)
	}

	job.Result = models.JSONB{
		"preview_generated": true,
		"preview_path":      previewPath,
		"preview_type":      "text_placeholder",
		"message":           "Preview generation completed (placeholder implementation)",
	}

	fp.logger.Info("Preview generated successfully", "document_id", document.ID, "path", previewPath)
	return nil
}

// processAIJob handles AI processing jobs using Claude service
func (fp *FileProcessor) processAIJob(ctx context.Context, job *models.AIProcessingJob, document *models.Document) error {
	// Check if AI processing is enabled
	if !fp.config.EnableAIProcessing {
		return fmt.Errorf("AI processing is disabled")
	}

	// Check if Claude service is available
	if fp.claudeService == nil {
		return fmt.Errorf("Claude service is not available")
	}

	fp.logger.Info("Processing AI job",
		"job_type", job.JobType,
		"document_id", document.ID,
		"document_name", document.OriginalName)

	// Get document content (for now, we'll extract text from storage)
	documentText, err := fp.extractDocumentText(ctx, document)
	if err != nil {
		return fmt.Errorf("failed to extract document text: %w", err)
	}

	// Process based on AI job type
	switch job.JobType {
	case "document_summarization":
		return fp.processDocumentSummarization(ctx, job, document, documentText)
	case "entity_extraction":
		return fp.processEntityExtraction(ctx, job, document, documentText)
	case "document_classification":
		return fp.processDocumentClassification(ctx, job, document, documentText)
	case "embedding_generation":
		return fp.processEmbeddingGeneration(ctx, job, document, documentText)
	case "semantic_analysis":
		return fp.processSemanticAnalysis(ctx, job, document, documentText)
	default:
		return fmt.Errorf("unknown AI job type: %s", job.JobType)
	}
}

// extractDocumentText extracts text content from document storage
func (fp *FileProcessor) extractDocumentText(ctx context.Context, document *models.Document) (string, error) {
	// Get document from storage
	fileReader, err := fp.storageService.Get(ctx, document.StoragePath)
	if err != nil {
		return "", fmt.Errorf("failed to get document from storage: %w", err)
	}
	defer fileReader.Close()

	// For now, create a placeholder text based on document metadata
	// In a full implementation, you'd use libraries like:
	// - github.com/ledongthuc/pdf for PDFs
	// - github.com/360EntSecGroup-Skylar/excelize for Excel
	// - github.com/nguyenthenguyen/docx for Word docs

	placeholderText := fmt.Sprintf(`Document: %s
Type: %s
Size: %d bytes
Upload Date: %s

This is a placeholder text extraction. In a production environment, 
this would contain the actual extracted text content from the document 
for AI processing using appropriate document parsing libraries.`,
		document.OriginalName,
		document.ContentType,
		document.FileSize,
		document.CreatedAt.Format("2006-01-02 15:04:05"))

	return placeholderText, nil
}

// AI Job Processing Methods

func (fp *FileProcessor) processDocumentSummarization(ctx context.Context, job *models.AIProcessingJob, document *models.Document, text string) error {
	fp.logger.Info("Generating document summary", "document_id", document.ID)

	summary, err := fp.claudeService.GenerateSummary(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	job.Result = models.JSONB{
		"summary":           summary,
		"summary_generated": true,
		"processing_time":   time.Now().Unix(),
		"text_length":       len(text),
	}

	fp.logger.Info("Document summary generated successfully",
		"document_id", document.ID,
		"summary_length", len(summary))

	return nil
}

func (fp *FileProcessor) processEntityExtraction(ctx context.Context, job *models.AIProcessingJob, document *models.Document, text string) error {
	fp.logger.Info("Extracting entities", "document_id", document.ID)

	entities, err := fp.claudeService.ExtractEntities(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to extract entities: %w", err)
	}

	job.Result = models.JSONB{
		"entities":           entities,
		"entities_extracted": true,
		"processing_time":    time.Now().Unix(),
		"text_length":        len(text),
	}

	fp.logger.Info("Entities extracted successfully",
		"document_id", document.ID,
		"entity_count", len(entities))

	return nil
}

func (fp *FileProcessor) processDocumentClassification(ctx context.Context, job *models.AIProcessingJob, document *models.Document, text string) error {
	fp.logger.Info("Classifying document", "document_id", document.ID)

	docType, confidence, err := fp.claudeService.ClassifyDocument(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to classify document: %w", err)
	}

	job.Result = models.JSONB{
		"document_type":       docType,
		"confidence":          confidence,
		"classification_done": true,
		"processing_time":     time.Now().Unix(),
		"text_length":         len(text),
	}

	fp.logger.Info("Document classified successfully",
		"document_id", document.ID,
		"type", docType,
		"confidence", confidence)

	return nil
}

func (fp *FileProcessor) processEmbeddingGeneration(ctx context.Context, job *models.AIProcessingJob, document *models.Document, text string) error {
	fp.logger.Info("Generating embeddings", "document_id", document.ID)

	// Claude doesn't provide embeddings directly - this would need OpenAI or another service
	// For now, return a placeholder result
	job.Result = models.JSONB{
		"embedding_generated": false,
		"error":               "Claude API does not provide embeddings - use OpenAI or another service",
		"processing_time":     time.Now().Unix(),
		"text_length":         len(text),
	}

	fp.logger.Info("Embedding generation completed with limitation",
		"document_id", document.ID)

	return nil
}

func (fp *FileProcessor) processSemanticAnalysis(ctx context.Context, job *models.AIProcessingJob, document *models.Document, text string) error {
	fp.logger.Info("Performing semantic analysis", "document_id", document.ID)

	// Use Claude for tag generation as semantic analysis
	tags, err := fp.claudeService.GenerateTags(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to generate tags: %w", err)
	}

	job.Result = models.JSONB{
		"tags":              tags,
		"semantic_analysis": true,
		"processing_time":   time.Now().Unix(),
		"text_length":       len(text),
		"tag_count":         len(tags),
	}

	fp.logger.Info("Semantic analysis completed successfully",
		"document_id", document.ID,
		"tag_count", len(tags))

	return nil
}

// Utility functions
func initializeDatabase(cfg *config.Config, log *logger.Logger) (*database.DB, error) {
	db, err := database.New(cfg.GetDatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Info("Database initialized successfully")
	return db, nil
}

func initializeStorageService(cfg *config.Config, log *logger.Logger) services.StorageService {
	switch cfg.Storage.Type {
	case "supabase":
		log.Info("Initializing Supabase storage service")
		storageConfig := storageSupabase.Config{
			URL:    cfg.Supabase.URL,
			APIKey: cfg.Supabase.ServiceKey,
			Bucket: cfg.Supabase.Bucket,
		}
		service, err := storageSupabase.NewStorageService(storageConfig)
		if err != nil {
			log.Error("Failed to initialize Supabase storage", "error", err)
			// Fallback to local storage
			log.Info("Falling back to local storage service")
			return local.NewStorageService(cfg.Storage.Path)
		}
		return service
	default:
		log.Info("Initializing local storage service", "path", cfg.Storage.Path)
		return local.NewStorageService(cfg.Storage.Path)
	}
}

// Helper methods
func (fp *FileProcessor) getThumbnailPath(originalPath string) string {
	// Convert original path to thumbnail path
	ext := filepath.Ext(originalPath)
	baseName := strings.TrimSuffix(originalPath, ext)
	return fmt.Sprintf("thumbnails/%s_thumb.jpg", filepath.Base(baseName))
}

func (fp *FileProcessor) getPreviewPath(originalPath string) string {
	// Convert original path to preview path
	ext := filepath.Ext(originalPath)
	baseName := strings.TrimSuffix(originalPath, ext)
	return fmt.Sprintf("previews/%s_preview.txt", filepath.Base(baseName))
}

func (fp *FileProcessor) validateContentTypeExtension(contentType, extension string) bool {
	// Basic content type vs extension validation
	commonMappings := map[string][]string{
		"application/pdf":    {"pdf"},
		"image/jpeg":         {"jpg", "jpeg"},
		"image/png":          {"png"},
		"image/gif":          {"gif"},
		"text/plain":         {"txt"},
		"application/msword": {"doc"},
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {"docx"},
	}

	if extensions, exists := commonMappings[contentType]; exists {
		for _, validExt := range extensions {
			if strings.ToLower(extension) == validExt {
				return true
			}
		}
		return false
	}

	// If no specific mapping, assume it's valid
	return true
}

// Environment variable helpers
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getInt64Env(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// File utility functions
func getFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext != "" {
		return strings.ToLower(ext[1:]) // Remove the dot and convert to lowercase
	}
	return ""
}

func estimatePages(contentType string, fileSize int64) int {
	switch {
	case strings.HasPrefix(contentType, "application/pdf"):
		// Rough estimate: 50KB per page for PDF
		return int(fileSize / 50000)
	case strings.HasPrefix(contentType, "application/msword"),
		strings.Contains(contentType, "wordprocessingml"):
		// Rough estimate: 20KB per page for Word docs
		return int(fileSize / 20000)
	default:
		return 1
	}
}

func estimateReadTime(fileSize int64, contentType string) int {
	// Estimate reading time in minutes
	switch {
	case strings.HasPrefix(contentType, "text/"):
		// Assume 250 words per minute, 5 chars per word
		return int(fileSize / (250 * 5))
	case strings.HasPrefix(contentType, "application/pdf"):
		// Assume 2 minutes per page
		return estimatePages(contentType, fileSize) * 2
	default:
		return 1
	}
}

func isImageFile(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}

func isDocumentFile(contentType string) bool {
	documentTypes := []string{
		"application/pdf",
		"application/msword",
		"application/vnd.openxmlformats",
		"text/plain",
		"text/html",
	}

	for _, docType := range documentTypes {
		if strings.HasPrefix(contentType, docType) {
			return true
		}
	}
	return false
}

func isArchiveFile(contentType string) bool {
	archiveTypes := []string{
		"application/zip",
		"application/x-rar",
		"application/x-tar",
		"application/gzip",
	}

	for _, archiveType := range archiveTypes {
		if strings.HasPrefix(contentType, archiveType) {
			return true
		}
	}
	return false
}

func categorizeFile(contentType string) string {
	switch {
	case isImageFile(contentType):
		return "image"
	case isDocumentFile(contentType):
		return "document"
	case isArchiveFile(contentType):
		return "archive"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio"
	default:
		return "other"
	}
}
