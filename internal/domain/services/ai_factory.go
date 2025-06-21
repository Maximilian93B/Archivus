package services

import (
	"fmt"

	"github.com/archivus/archivus/internal/app/config"
)

// AIServiceFactory creates and manages AI service instances
type AIServiceFactory struct {
	openAIService *OpenAIService
	claudeService *ClaudeService
	config        *config.Config
}

// NewAIServiceFactory creates a new AI service factory
func NewAIServiceFactory(cfg *config.Config) (*AIServiceFactory, error) {
	factory := &AIServiceFactory{
		config: cfg,
	}

	// Initialize Claude service if enabled
	if cfg.AI.Claude.Enabled {
		claudeConfig := ClaudeServiceConfig{
			APIKey:         cfg.AI.Claude.APIKey,
			BaseURL:        cfg.AI.Claude.BaseURL,
			Model:          cfg.AI.Claude.Model,
			MaxTokens:      cfg.AI.Claude.MaxTokens,
			Temperature:    cfg.AI.Claude.Temperature,
			TimeoutSeconds: cfg.AI.Claude.TimeoutSeconds,
			RateLimitRPM:   cfg.AI.Claude.RateLimitRPM,
			RetryAttempts:  cfg.AI.Claude.RetryAttempts,
			Enabled:        cfg.AI.Claude.Enabled,
		}

		claudeService, err := NewClaudeService(claudeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Claude service: %w", err)
		}
		factory.claudeService = claudeService
	}

	return factory, nil
}

// GetPreferredService returns the preferred AI service based on configuration
func (f *AIServiceFactory) GetPreferredService() string {
	if f.claudeService != nil && f.claudeService.IsEnabled() {
		return "claude"
	}
	if f.openAIService != nil {
		return "openai"
	}
	return "none"
}

// GetClaudeService returns the Claude service instance
func (f *AIServiceFactory) GetClaudeService() *ClaudeService {
	return f.claudeService
}

// GetOpenAIService returns the OpenAI service instance
func (f *AIServiceFactory) GetOpenAIService() *OpenAIService {
	return f.openAIService
}

// IsClaudeEnabled returns whether Claude service is enabled and available
func (f *AIServiceFactory) IsClaudeEnabled() bool {
	return f.claudeService != nil && f.claudeService.IsEnabled()
}

// GetTokenUsage returns token usage statistics for all services
func (f *AIServiceFactory) GetTokenUsage() map[string]interface{} {
	usage := make(map[string]interface{})

	if f.claudeService != nil {
		inputTokens, outputTokens, requests := f.claudeService.GetTokenUsage()
		usage["claude"] = map[string]interface{}{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"requests":      requests,
		}
	}

	// TODO: Add OpenAI usage tracking when implemented
	usage["openai"] = map[string]interface{}{
		"input_tokens":  0,
		"output_tokens": 0,
		"requests":      0,
	}

	return usage
}
