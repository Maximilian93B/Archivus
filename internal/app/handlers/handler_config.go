package handlers

import (
	"os"
	"strconv"
	"time"
)

// HandlerConfig provides environment-aware configuration for handlers
type HandlerConfig struct {
	// Pagination settings
	MaxPageSize     int `json:"max_page_size"`
	DefaultPageSize int `json:"default_page_size"`

	// File upload settings
	MaxFileSize      int64    `json:"max_file_size"`
	AllowedFileTypes []string `json:"allowed_file_types"`

	// Error handling settings
	EnableDebugErrors bool `json:"enable_debug_errors"`
	IncludeStackTrace bool `json:"include_stack_trace"`

	// Rate limiting settings
	EnableRateLimit bool          `json:"enable_rate_limit"`
	RateLimit       int           `json:"rate_limit"`
	RateLimitWindow time.Duration `json:"rate_limit_window"`

	// Environment
	Environment string `json:"environment"`

	// Logging
	EnableRequestLogging bool   `json:"enable_request_logging"`
	LogLevel             string `json:"log_level"`
}

// NewHandlerConfig creates a new handler configuration with environment-specific defaults
func NewHandlerConfig() *HandlerConfig {
	config := &HandlerConfig{
		// Default values
		MaxPageSize:          100,
		DefaultPageSize:      20,
		MaxFileSize:          10 * 1024 * 1024, // 10MB
		AllowedFileTypes:     []string{"application/pdf", "image/jpeg", "image/png", "application/msword"},
		EnableDebugErrors:    false,
		IncludeStackTrace:    false,
		EnableRateLimit:      true,
		RateLimit:            100,
		RateLimitWindow:      time.Minute,
		Environment:          "production",
		EnableRequestLogging: true,
		LogLevel:             "info",
	}

	// Override with environment variables
	config.loadFromEnv()

	// Apply environment-specific overrides
	config.applyEnvironmentDefaults()

	return config
}

// loadFromEnv loads configuration from environment variables
func (c *HandlerConfig) loadFromEnv() {
	if val := os.Getenv("MAX_PAGE_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			c.MaxPageSize = parsed
		}
	}

	if val := os.Getenv("DEFAULT_PAGE_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			c.DefaultPageSize = parsed
		}
	}

	if val := os.Getenv("MAX_FILE_SIZE"); val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			c.MaxFileSize = parsed
		}
	}

	if val := os.Getenv("ENABLE_DEBUG_ERRORS"); val != "" {
		c.EnableDebugErrors = val == "true"
	}

	if val := os.Getenv("INCLUDE_STACK_TRACE"); val != "" {
		c.IncludeStackTrace = val == "true"
	}

	if val := os.Getenv("ENABLE_RATE_LIMIT"); val != "" {
		c.EnableRateLimit = val == "true"
	}

	if val := os.Getenv("RATE_LIMIT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			c.RateLimit = parsed
		}
	}

	if val := os.Getenv("RATE_LIMIT_WINDOW"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			c.RateLimitWindow = parsed
		}
	}

	if val := os.Getenv("ENVIRONMENT"); val != "" {
		c.Environment = val
	}

	if val := os.Getenv("ENABLE_REQUEST_LOGGING"); val != "" {
		c.EnableRequestLogging = val == "true"
	}

	if val := os.Getenv("LOG_LEVEL"); val != "" {
		c.LogLevel = val
	}
}

// applyEnvironmentDefaults applies environment-specific default values
func (c *HandlerConfig) applyEnvironmentDefaults() {
	switch c.Environment {
	case "development", "dev":
		c.EnableDebugErrors = true
		c.IncludeStackTrace = true
		c.EnableRateLimit = false
		c.LogLevel = "debug"
		c.EnableRequestLogging = true

	case "test", "testing":
		c.EnableDebugErrors = true
		c.IncludeStackTrace = false
		c.EnableRateLimit = false
		c.LogLevel = "info"
		c.EnableRequestLogging = false

	case "production", "prod":
		c.EnableDebugErrors = false
		c.IncludeStackTrace = false
		c.EnableRateLimit = true
		c.LogLevel = "warn"
		c.EnableRequestLogging = true
		// Stricter limits in production
		if c.MaxPageSize > 50 {
			c.MaxPageSize = 50
		}

	case "staging", "stage":
		c.EnableDebugErrors = false
		c.IncludeStackTrace = false
		c.EnableRateLimit = true
		c.LogLevel = "info"
		c.EnableRequestLogging = true
	}
}

// IsDevelopment returns true if running in development environment
func (c *HandlerConfig) IsDevelopment() bool {
	return c.Environment == "development" || c.Environment == "dev"
}

// IsProduction returns true if running in production environment
func (c *HandlerConfig) IsProduction() bool {
	return c.Environment == "production" || c.Environment == "prod"
}

// IsTest returns true if running in test environment
func (c *HandlerConfig) IsTest() bool {
	return c.Environment == "test" || c.Environment == "testing"
}

// ValidatePageSize ensures page size is within acceptable limits
func (c *HandlerConfig) ValidatePageSize(pageSize int) int {
	if pageSize < 1 {
		return c.DefaultPageSize
	}
	if pageSize > c.MaxPageSize {
		return c.MaxPageSize
	}
	return pageSize
}

// ValidateFileSize checks if file size is acceptable
func (c *HandlerConfig) ValidateFileSize(size int64) bool {
	return size <= c.MaxFileSize
}

// ValidateFileType checks if file type is allowed
func (c *HandlerConfig) ValidateFileType(contentType string) bool {
	for _, allowedType := range c.AllowedFileTypes {
		if allowedType == contentType {
			return true
		}
	}
	return false
}
