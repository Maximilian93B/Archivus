package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment string
	Server      ServerConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	JWT         JWTConfig
	Storage     StorageConfig
	Supabase    SupabaseConfig
	AI          AIConfig
	Features    FeatureConfig
	Limits      LimitsConfig
}

type ServerConfig struct {
	Host           string
	Port           string
	AllowedOrigins []string
}

type DatabaseConfig struct {
	URL     string
	TestURL string
}

type RedisConfig struct {
	URL      string
	Host     string
	Port     int
	Username string
	Password string
	DB       int
	PoolSize int
}

type JWTConfig struct {
	Secret string
	Expiry time.Duration
}

type StorageConfig struct {
	Type      string
	Path      string
	S3Bucket  string
	S3Region  string
	AccessKey string
	SecretKey string
}

type SupabaseConfig struct {
	URL        string
	APIKey     string
	ServiceKey string
	Bucket     string
	JWTSecret  string
}

type AIConfig struct {
	OpenAI  OpenAIConfig
	Ollama  OllamaConfig
	Enabled bool
}

type OpenAIConfig struct {
	APIKey    string
	Model     string
	MaxTokens int
}

type OllamaConfig struct {
	Host  string
	Model string
}

type FeatureConfig struct {
	AIProcessing bool
	OCR          bool
	Webhooks     bool
}

type LimitsConfig struct {
	MaxFileSize      int64
	AllowedFileTypes []string
	RateLimit        int
	RateLimitWindow  time.Duration
}

// Load configuration from environment variables
func Load() (*Config, error) {
	// Load .env file in non-production environments
	env := os.Getenv("ENVIRONMENT")
	if env != "production" {
		if err := godotenv.Load(); err != nil {
			// .env file is optional
		}
	}

	config := &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		Server: ServerConfig{
			Host:           getEnv("HOST", "localhost"),
			Port:           getEnv("PORT", "8080"),
			AllowedOrigins: strings.Split(getEnv("ALLOWED_ORIGINS", "http://localhost:3000"), ","),
		},
		Database: DatabaseConfig{
			URL:     getEnv("DATABASE_URL", ""),
			TestURL: getEnv("DATABASE_URL_TEST", ""),
		},
		Redis: RedisConfig{
			URL:      getEnv("REDIS_URL", "redis://localhost:6379"),
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     parseInt(getEnv("REDIS_PORT", "6379")),
			Username: getEnv("REDIS_USERNAME", ""),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       parseInt(getEnv("REDIS_DB", "0")),
			PoolSize: parseInt(getEnv("REDIS_POOL_SIZE", "10")),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", ""),
			Expiry: parseDuration(getEnv("JWT_EXPIRY", "24h")),
		},
		Storage: StorageConfig{
			Type:      getEnv("STORAGE_TYPE", "local"),
			Path:      getEnv("STORAGE_PATH", "./uploads"),
			S3Bucket:  getEnv("S3_BUCKET", ""),
			S3Region:  getEnv("S3_REGION", "us-west-2"),
			AccessKey: getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		},
		Supabase: SupabaseConfig{
			URL:        getEnv("SUPABASE_URL", ""),
			APIKey:     getEnv("SUPABASE_API_KEY", ""),
			ServiceKey: getEnv("SUPABASE_SERVICE_KEY", ""),
			Bucket:     getEnv("SUPABASE_BUCKET", ""),
			JWTSecret:  getEnv("SUPABASE_JWT_SECRET", ""),
		},
		AI: AIConfig{
			OpenAI: OpenAIConfig{
				APIKey:    getEnv("OPENAI_API_KEY", ""),
				Model:     getEnv("OPENAI_MODEL", "gpt-3.5-turbo"),
				MaxTokens: parseInt(getEnv("OPENAI_MAX_TOKENS", "1000")),
			},
			Ollama: OllamaConfig{
				Host:  getEnv("OLLAMA_HOST", "http://localhost:11434"),
				Model: getEnv("OLLAMA_MODEL", "llama2"),
			},
			Enabled: parseBool(getEnv("ENABLE_AI_PROCESSING", "false")),
		},
		Features: FeatureConfig{
			AIProcessing: parseBool(getEnv("ENABLE_AI_PROCESSING", "false")),
			OCR:          parseBool(getEnv("ENABLE_OCR", "false")),
			Webhooks:     parseBool(getEnv("ENABLE_WEBHOOKS", "false")),
		},
		Limits: LimitsConfig{
			MaxFileSize:      parseInt64(getEnv("MAX_FILE_SIZE", "104857600")),
			AllowedFileTypes: strings.Split(getEnv("ALLOWED_FILE_TYPES", "pdf,doc,docx,txt,jpg,jpeg,png"), ","),
			RateLimit:        parseInt(getEnv("RATE_LIMIT_REQUESTS", "100")),
			RateLimitWindow:  parseDuration(getEnv("RATE_LIMIT_WINDOW", "60s")),
		},
	}

	// Validate required configuration
	if err := validate(config); err != nil {
		return nil, err
	}

	return config, nil
}

// GetDatabaseURL returns the appropriate database URL based on environment
func (c *Config) GetDatabaseURL() string {
	if c.Environment == "test" && c.Database.TestURL != "" {
		return c.Database.TestURL
	}
	return c.Database.URL
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsTest returns true if running in test environment
func (c *Config) IsTest() bool {
	return c.Environment == "test"
}

func validate(config *Config) error {
	// Database URL is optional for development
	if config.IsProduction() && config.GetDatabaseURL() == "" {
		return fmt.Errorf("DATABASE_URL is required in production")
	}
	if config.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if config.Features.AIProcessing && config.AI.OpenAI.APIKey == "" {
		return fmt.Errorf("OPENAI_API_KEY is required when AI processing is enabled")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt(value string) int {
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	return 0
}

func parseInt64(value string) int64 {
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i
	}
	return 0
}

func parseBool(value string) bool {
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}
	return false
}

func parseDuration(value string) time.Duration {
	if d, err := time.ParseDuration(value); err == nil {
		return d
	}
	return 0
}
