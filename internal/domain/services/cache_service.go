package services

import (
	"context"
	"time"
)

// CacheService interface for caching operations
type CacheService interface {
	// Basic operations
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Atomic operations
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
	Increment(ctx context.Context, key string) (int64, error)

	// Hash operations for structured data
	HSet(ctx context.Context, key string, field string, value interface{}) error
	HGet(ctx context.Context, key string, field string) (string, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)

	// List operations for queues
	LPush(ctx context.Context, key string, values ...interface{}) error
	RPop(ctx context.Context, key string) (string, error)

	// Set operations for unique collections
	SAdd(ctx context.Context, key string, members ...interface{}) error
	SMembers(ctx context.Context, key string) ([]string, error)

	// Health check
	Ping(ctx context.Context) error
	Close() error
}

// Cache key patterns for the application
const (
	// Session keys
	SessionKeyPattern = "session:%s"

	// User cache keys
	UserCacheKeyPattern = "user:%s"

	// Document cache keys
	DocumentCacheKeyPattern = "doc:%s"
	DocumentListKeyPattern  = "doc_list:%s:%s" // tenant:filter_hash

	// Tenant cache keys
	TenantCacheKeyPattern = "tenant:%s"

	// AI processing cache
	AIJobQueueKey      = "ai_jobs:queue"
	AIResultKeyPattern = "ai_result:%s"

	// Rate limiting keys
	RateLimitKeyPattern = "rate_limit:%s:%s" // tenant:user

	// Analytics cache
	DashboardCacheKeyPattern = "dashboard:%s:%s" // tenant:period

	// Search cache
	SearchCacheKeyPattern = "search:%s:%s" // tenant:query_hash
)

// Common cache durations
const (
	CacheShortTerm  = 5 * time.Minute
	CacheMediumTerm = 30 * time.Minute
	CacheLongTerm   = 2 * time.Hour
	CacheDay        = 24 * time.Hour
	CacheWeek       = 7 * 24 * time.Hour

	// Session duration
	SessionDuration = 24 * time.Hour

	// Rate limiting windows
	RateLimitWindow = time.Minute
)
