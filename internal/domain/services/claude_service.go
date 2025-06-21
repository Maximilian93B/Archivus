package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/go-resty/resty/v2"
)

// Claude API constants
const (
	ClaudeAPIVersion     = "2023-06-01"
	ClaudeMaxTokensLimit = 200000 // Context window limit
	ClaudeDefaultModel   = "claude-3-5-sonnet-20241022"
)

// Claude API request/response structures
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	Messages    []ClaudeMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type ClaudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string       `json:"model"`
	StopReason   string       `json:"stop_reason"`
	StopSequence *string      `json:"stop_sequence"`
	Usage        ClaudeUsage  `json:"usage"`
	Error        *ClaudeError `json:"error,omitempty"`
}

type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ClaudeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type ClaudeStreamEvent struct {
	Type    string          `json:"type"`
	Message *ClaudeResponse `json:"message,omitempty"`
	Delta   *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
}

// ClaudeServiceConfig holds configuration for Claude API integration
type ClaudeServiceConfig struct {
	APIKey         string
	BaseURL        string
	Model          string
	MaxTokens      int
	Temperature    float64
	TimeoutSeconds int
	RateLimitRPM   int
	RetryAttempts  int
	Enabled        bool
}

// ClaudeService implements AI operations using Anthropic's Claude API
type ClaudeService struct {
	config       ClaudeServiceConfig
	client       *resty.Client
	rateLimiter  *RateLimiter
	tokenTracker *TokenTracker
	mu           sync.RWMutex
}

// RateLimiter manages API rate limiting
type RateLimiter struct {
	requests    []time.Time
	maxRequests int
	window      time.Duration
	mu          sync.Mutex
}

// TokenTracker tracks token usage for optimization
type TokenTracker struct {
	totalInputTokens  int64
	totalOutputTokens int64
	totalRequests     int64
	mu                sync.RWMutex
}

// NewClaudeService creates a new Claude service instance
func NewClaudeService(config ClaudeServiceConfig) (*ClaudeService, error) {
	if !config.Enabled {
		return nil, errors.New("Claude service is not enabled")
	}

	if config.APIKey == "" {
		return nil, errors.New("Claude API key is required")
	}

	// Set defaults
	if config.BaseURL == "" {
		config.BaseURL = "https://api.anthropic.com"
	}
	if config.Model == "" {
		config.Model = ClaudeDefaultModel
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 4096
	}
	if config.Temperature == 0 {
		config.Temperature = 0.1
	}
	if config.TimeoutSeconds == 0 {
		config.TimeoutSeconds = 60
	}
	if config.RateLimitRPM == 0 {
		config.RateLimitRPM = 1000
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}

	// Create HTTP client with proper headers
	client := resty.New()
	client.SetTimeout(time.Duration(config.TimeoutSeconds) * time.Second)
	client.SetHeader("Content-Type", "application/json")
	client.SetHeader("x-api-key", config.APIKey)
	client.SetHeader("anthropic-version", ClaudeAPIVersion)
	client.SetBaseURL(config.BaseURL)

	// Initialize rate limiter
	rateLimiter := &RateLimiter{
		requests:    make([]time.Time, 0),
		maxRequests: config.RateLimitRPM,
		window:      time.Minute,
	}

	// Initialize token tracker
	tokenTracker := &TokenTracker{}

	return &ClaudeService{
		config:       config,
		client:       client,
		rateLimiter:  rateLimiter,
		tokenTracker: tokenTracker,
	}, nil
}

// ExtractText extracts and analyzes text content (Claude doesn't need separate extraction)
func (s *ClaudeService) ExtractText(ctx context.Context, text string) (string, error) {
	// Claude works with text directly, so return as-is
	return text, nil
}

// GenerateEmbedding generates embeddings (Note: Claude doesn't provide embeddings directly)
func (s *ClaudeService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Claude doesn't provide embeddings - this would need a separate service
	// For now, return an error indicating this limitation
	return nil, errors.New("Claude API does not provide embeddings - use OpenAI or another service")
}

// GenerateSummary generates intelligent document summaries using Claude
func (s *ClaudeService) GenerateSummary(ctx context.Context, text string) (string, error) {
	if len(text) == 0 {
		return "", errors.New("empty text provided for summarization")
	}

	prompt := fmt.Sprintf(`Please provide a comprehensive summary of the following document. 
Include:
1. Executive Summary (2-3 sentences)
2. Key Points (bullet points)
3. Important Details
4. Action Items (if any)

Document:
%s`, text)

	response, err := s.makeRequest(ctx, prompt, false)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	return response, nil
}

// ExtractEntities extracts named entities and important concepts from text
func (s *ClaudeService) ExtractEntities(ctx context.Context, text string) (map[string]interface{}, error) {
	if len(text) == 0 {
		return nil, errors.New("empty text provided for entity extraction")
	}

	prompt := fmt.Sprintf(`Extract and categorize the following entities from this document. Return as JSON with these categories:
- people: Names of individuals
- organizations: Company names, institutions
- locations: Cities, countries, addresses  
- dates: Important dates and time periods
- amounts: Monetary values, quantities
- concepts: Key topics and themes
- contacts: Email addresses, phone numbers

Document:
%s

Return only valid JSON.`, text)

	response, err := s.makeRequest(ctx, prompt, false)
	if err != nil {
		return nil, fmt.Errorf("failed to extract entities: %w", err)
	}

	// Parse JSON response
	var entities map[string]interface{}
	if err := json.Unmarshal([]byte(response), &entities); err != nil {
		// If JSON parsing fails, return a basic structure
		return map[string]interface{}{
			"raw_response":  response,
			"parsing_error": err.Error(),
		}, nil
	}

	return entities, nil
}

// ClassifyDocument classifies document type with confidence score
func (s *ClaudeService) ClassifyDocument(ctx context.Context, text string) (models.DocumentType, float64, error) {
	if len(text) == 0 {
		return models.DocTypeGeneral, 0.0, errors.New("empty text provided for classification")
	}

	prompt := fmt.Sprintf(`Classify this document into one of these types and provide a confidence score (0-1):
- CONTRACT: Legal agreements, terms of service
- INVOICE: Bills, receipts, payment documents  
- REPORT: Business reports, analysis documents
- CORRESPONDENCE: Emails, letters, memos
- PRESENTATION: Slides, pitch decks
- SPREADSHEET: Data tables, financial sheets
- RESUME: CVs, job applications
- MANUAL: Instructions, documentation
- FORM: Applications, surveys, forms
- GENERAL: Other document types

Return only JSON: {"type": "DOCUMENT_TYPE", "confidence": 0.95, "reasoning": "brief explanation"}

Document:
%s`, text)

	response, err := s.makeRequest(ctx, prompt, false)
	if err != nil {
		return models.DocTypeGeneral, 0.0, fmt.Errorf("failed to classify document: %w", err)
	}

	// Parse JSON response
	var result struct {
		Type       string  `json:"type"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// Return general type with low confidence if parsing fails
		return models.DocTypeGeneral, 0.1, nil
	}

	// Convert string to DocumentType
	docType := s.stringToDocumentType(result.Type)
	return docType, result.Confidence, nil
}

// GenerateTags generates relevant tags for document categorization
func (s *ClaudeService) GenerateTags(ctx context.Context, text string) ([]string, error) {
	if len(text) == 0 {
		return nil, errors.New("empty text provided for tag generation")
	}

	prompt := fmt.Sprintf(`Generate 5-10 relevant tags for this document. Tags should be:
- Single words or short phrases
- Descriptive of content and topic
- Useful for categorization and search
- Professional and consistent

Return only JSON array: ["tag1", "tag2", "tag3"]

Document:
%s`, text)

	response, err := s.makeRequest(ctx, prompt, false)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tags: %w", err)
	}

	// Parse JSON response
	var tags []string
	if err := json.Unmarshal([]byte(response), &tags); err != nil {
		// If JSON parsing fails, extract tags from text
		return s.extractTagsFromText(response), nil
	}

	return tags, nil
}

// ExtractFinancialData extracts financial information from documents
func (s *ClaudeService) ExtractFinancialData(ctx context.Context, text string, docType models.DocumentType) (map[string]interface{}, error) {
	if len(text) == 0 {
		return nil, errors.New("empty text provided for financial extraction")
	}

	prompt := fmt.Sprintf(`Extract financial data from this %s document. Return as JSON with these fields:
- total_amount: Main monetary value
- currency: Currency code (USD, EUR, etc.)
- due_date: Payment or due date
- invoice_number: Reference number
- tax_amount: Tax/VAT amount
- items: Array of line items with description and amount
- payment_terms: Payment conditions
- vendor: Company/person providing service
- client: Company/person receiving service

Document:
%s

Return only valid JSON.`, docType, text)

	response, err := s.makeRequest(ctx, prompt, false)
	if err != nil {
		return nil, fmt.Errorf("failed to extract financial data: %w", err)
	}

	// Parse JSON response
	var financialData map[string]interface{}
	if err := json.Unmarshal([]byte(response), &financialData); err != nil {
		// Return basic structure if parsing fails
		return map[string]interface{}{
			"raw_response":  response,
			"parsing_error": err.Error(),
		}, nil
	}

	return financialData, nil
}

// PerformOCR is not supported by Claude API
func (s *ClaudeService) PerformOCR(ctx context.Context, filePath string) (string, error) {
	return "", errors.New("OCR is not supported by Claude API - use a dedicated OCR service")
}

// makeRequest makes a request to Claude API with rate limiting and error handling
func (s *ClaudeService) makeRequest(ctx context.Context, prompt string, stream bool) (string, error) {
	// Check rate limiting
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Prepare request
	request := ClaudeRequest{
		Model:       s.config.Model,
		MaxTokens:   s.config.MaxTokens,
		Temperature: s.config.Temperature,
		Stream:      stream,
		Messages: []ClaudeMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	var response ClaudeResponse
	var err error

	// Retry logic
	for attempt := 0; attempt < s.config.RetryAttempts; attempt++ {
		resp, reqErr := s.client.R().
			SetContext(ctx).
			SetBody(request).
			SetResult(&response).
			Post("/v1/messages")

		if reqErr != nil {
			if attempt == s.config.RetryAttempts-1 {
				err = reqErr
				break
			}
			// Exponential backoff
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		if resp.StatusCode() == 200 {
			err = nil
			break
		}

		// Handle specific error codes
		switch resp.StatusCode() {
		case 429: // Rate limited
			if attempt < s.config.RetryAttempts-1 {
				time.Sleep(time.Duration(attempt+2) * time.Second)
				continue
			}
			err = errors.New("rate limit exceeded")
		case 401:
			err = errors.New("invalid API key")
		case 400:
			err = fmt.Errorf("bad request: %s", resp.String())
		default:
			err = fmt.Errorf("API error %d: %s", resp.StatusCode(), resp.String())
		}

		if attempt == s.config.RetryAttempts-1 {
			break
		}
	}

	if err != nil {
		return "", err
	}

	// Handle API errors in response
	if response.Error != nil {
		return "", fmt.Errorf("Claude API error: %s", response.Error.Message)
	}

	// Track token usage
	s.tokenTracker.Track(response.Usage.InputTokens, response.Usage.OutputTokens)

	// Extract text from response
	if len(response.Content) > 0 {
		return response.Content[0].Text, nil
	}

	return "", errors.New("empty response from Claude API")
}

// Wait implements rate limiting
func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Remove old requests outside the window
	cutoff := now.Add(-rl.window)
	validRequests := make([]time.Time, 0)
	for _, reqTime := range rl.requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}
	rl.requests = validRequests

	// Check if we can make a new request
	if len(rl.requests) >= rl.maxRequests {
		// Calculate wait time
		oldestRequest := rl.requests[0]
		waitTime := rl.window - now.Sub(oldestRequest)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue after waiting
		}
	}

	// Add current request
	rl.requests = append(rl.requests, now)
	return nil
}

// Track records token usage
func (tt *TokenTracker) Track(inputTokens, outputTokens int) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	tt.totalInputTokens += int64(inputTokens)
	tt.totalOutputTokens += int64(outputTokens)
	tt.totalRequests++
}

// GetUsage returns current token usage statistics
func (tt *TokenTracker) GetUsage() (inputTokens, outputTokens, requests int64) {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	return tt.totalInputTokens, tt.totalOutputTokens, tt.totalRequests
}

// GetTokenUsage returns token usage statistics for the service
func (s *ClaudeService) GetTokenUsage() (inputTokens, outputTokens, requests int64) {
	return s.tokenTracker.GetUsage()
}

// IsEnabled returns whether the Claude service is enabled
func (s *ClaudeService) IsEnabled() bool {
	return s.config.Enabled
}

// stringToDocumentType converts string to DocumentType enum
func (s *ClaudeService) stringToDocumentType(typeStr string) models.DocumentType {
	switch strings.ToUpper(typeStr) {
	case "CONTRACT":
		return models.DocTypeContract
	case "INVOICE":
		return models.DocTypeInvoice
	case "REPORT":
		return models.DocTypeReport
	case "CORRESPONDENCE":
		return models.DocTypeGeneral // No correspondence type in models, use general
	case "PRESENTATION":
		return models.DocTypePresentationn // Note: there's a typo in the model with double 'n'
	case "SPREADSHEET":
		return models.DocTypeSpreadsheet
	case "RESUME":
		return models.DocTypeHR // Map resume to HR documents
	case "MANUAL":
		return models.DocTypeGeneral
	case "FORM":
		return models.DocTypeGeneral
	case "RECEIPT":
		return models.DocTypeReceipt
	default:
		return models.DocTypeGeneral
	}
}

// extractTagsFromText extracts tags from response text when JSON parsing fails
func (s *ClaudeService) extractTagsFromText(text string) []string {
	// Simple fallback - extract words that look like tags
	words := strings.Fields(text)
	var tags []string

	for _, word := range words {
		cleaned := strings.Trim(word, "[]\"',.")
		if len(cleaned) > 2 && len(cleaned) < 30 {
			tags = append(tags, cleaned)
		}

		// Limit to reasonable number of tags
		if len(tags) >= 10 {
			break
		}
	}

	return tags
}
