package examples

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/archivus/archivus/internal/app/config"
	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/cache"
	"github.com/google/uuid"
)

// RedisUsageExample demonstrates how to use Redis in your Archivus application
func RedisUsageExample() {
	// 1. Load configuration (this loads your REDIS_URL from environment)
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// 2. Create Redis cache service using your configuration
	cacheService, err := cache.CreateCacheService(cfg.Redis.URL)
	if err != nil {
		log.Fatal("Failed to create cache service:", err)
	}
	defer cacheService.Close()

	ctx := context.Background()

	// Example 1: Basic caching
	fmt.Println("=== Basic Caching Example ===")
	key := "user:profile:123"
	userProfile := `{"id":"123","name":"John Doe","email":"john@example.com"}`

	// Cache user profile for 30 minutes
	err = cacheService.Set(ctx, key, userProfile, 30*time.Minute)
	if err != nil {
		log.Printf("Failed to cache user profile: %v", err)
	}

	// Retrieve cached profile
	if cached, err := cacheService.Get(ctx, key); err == nil {
		fmt.Printf("Cached user profile: %s\n", cached)
	}

	// Example 2: Session management
	fmt.Println("\n=== Session Management Example ===")
	sessionToken := "sess_" + uuid.New().String()
	sessionKey := fmt.Sprintf(services.SessionKeyPattern, sessionToken)

	// Store session data as a hash
	cacheService.HSet(ctx, sessionKey, "user_id", "123")
	cacheService.HSet(ctx, sessionKey, "created_at", fmt.Sprint(time.Now().Unix()))
	cacheService.HSet(ctx, sessionKey, "last_seen", fmt.Sprint(time.Now().Unix()))

	// Retrieve session data
	if userID, err := cacheService.HGet(ctx, sessionKey, "user_id"); err == nil {
		fmt.Printf("Session user ID: %s\n", userID)
	}

	// Example 3: Rate limiting
	fmt.Println("\n=== Rate Limiting Example ===")
	userID := uuid.New()
	rateLimitKey := fmt.Sprintf("rate_limit:%s:api_calls", userID)

	// Increment API call counter
	count, err := cacheService.Increment(ctx, rateLimitKey)
	if err != nil {
		log.Printf("Failed to increment rate limit: %v", err)
	} else {
		fmt.Printf("API calls for user %s: %d\n", userID, count)

		// Check if rate limit exceeded (example: 100 calls per hour)
		if count > 100 {
			fmt.Println("Rate limit exceeded!")
		}
	}

	// Example 4: Document search result caching
	fmt.Println("\n=== Search Result Caching Example ===")
	searchQuery := "invoice 2024"
	searchKey := fmt.Sprintf(services.SearchCacheKeyPattern, "tenant-123", searchQuery)

	searchResults := `[
		{"id":"doc1","title":"Invoice 2024-01","type":"invoice"},
		{"id":"doc2","title":"Invoice 2024-02","type":"invoice"}
	]`

	// Cache search results for 15 minutes
	cacheService.Set(ctx, searchKey, searchResults, 15*time.Minute)
	fmt.Printf("Cached search results for query: %s\n", searchQuery)

	// Example 5: Active users tracking
	fmt.Println("\n=== Active Users Tracking Example ===")
	activeUsersKey := "active_users"

	// Add users to active set
	cacheService.SAdd(ctx, activeUsersKey, "user-123", "user-456", "user-789")

	// Get all active users
	if activeUsers, err := cacheService.SMembers(ctx, activeUsersKey); err == nil {
		fmt.Printf("Active users: %v\n", activeUsers)
	}

	// Example 6: Document processing queue
	fmt.Println("\n=== Processing Queue Example ===")
	queueKey := services.AIJobQueueKey

	// Add jobs to queue
	cacheService.LPush(ctx, queueKey, "job-1", "job-2", "job-3")

	// Process jobs from queue
	if job, err := cacheService.RPop(ctx, queueKey); err == nil {
		fmt.Printf("Processing job: %s\n", job)
	}

	// Example 7: Analytics caching
	fmt.Println("\n=== Analytics Caching Example ===")
	dashboardKey := fmt.Sprintf(services.DashboardCacheKeyPattern, "tenant-123", "daily")

	analyticsData := `{
		"total_docs": 1500,
		"storage_used": 2048000,
		"active_users": 25,
		"recent_activity": 150
	}`

	// Cache analytics for 1 hour
	cacheService.Set(ctx, dashboardKey, analyticsData, services.CacheLongTerm)
	fmt.Printf("Cached analytics data for tenant dashboard\n")

	// Example 8: Cleanup - Optional, for demonstration
	fmt.Println("\n=== Cleanup ===")
	keys := []string{key, sessionKey, rateLimitKey, searchKey, dashboardKey}
	for _, k := range keys {
		cacheService.Delete(ctx, k)
	}
	fmt.Println("Cleaned up example cache keys")
}

// AdvancedRedisPatterns shows more advanced Redis usage patterns
func AdvancedRedisPatterns() {
	cfg, _ := config.Load()
	cacheService, _ := cache.CreateCacheService(cfg.Redis.URL)
	defer cacheService.Close()

	ctx := context.Background()

	// Pattern 1: Cache-aside pattern for user profiles
	fmt.Println("=== Cache-Aside Pattern ===")
	userID := uuid.New()

	getUserProfile := func(userID uuid.UUID) (string, error) {
		cacheKey := fmt.Sprintf("user_profile:%s", userID)

		// Try cache first
		if cached, err := cacheService.Get(ctx, cacheKey); err == nil {
			fmt.Println("Cache hit!")
			return cached, nil
		}

		// Cache miss - simulate database fetch
		fmt.Println("Cache miss - fetching from database")
		profile := fmt.Sprintf(`{"id":"%s","name":"User %s","loaded_from":"database"}`, userID, userID.String()[:8])

		// Store in cache for next time
		cacheService.Set(ctx, cacheKey, profile, 30*time.Minute)

		return profile, nil
	}

	// First call - cache miss
	profile1, _ := getUserProfile(userID)
	fmt.Printf("First call result: %s\n", profile1)

	// Second call - cache hit
	profile2, _ := getUserProfile(userID)
	fmt.Printf("Second call result: %s\n", profile2)

	// Pattern 2: Distributed locking (simple implementation)
	fmt.Println("\n=== Distributed Locking Pattern ===")
	lockKey := "lock:document_processing:doc123"

	// Try to acquire lock
	acquired, err := cacheService.SetNX(ctx, lockKey, "locked", 5*time.Minute)
	if err != nil {
		log.Printf("Lock error: %v", err)
	} else if acquired {
		fmt.Println("Lock acquired - processing document...")
		// Simulate processing
		time.Sleep(1 * time.Second)
		// Release lock
		cacheService.Delete(ctx, lockKey)
		fmt.Println("Processing complete - lock released")
	} else {
		fmt.Println("Could not acquire lock - document being processed by another worker")
	}

	// Pattern 3: Leaderboard using sorted sets (simulated with regular sets for this example)
	fmt.Println("\n=== User Activity Leaderboard ===")
	leaderboardKey := "user_activity_leaderboard"

	// Add users with their activity scores (in a real implementation you'd use ZADD)
	topUsers := []string{"user1:150", "user2:120", "user3:95", "user4:80"}
	for _, user := range topUsers {
		cacheService.SAdd(ctx, leaderboardKey, user)
	}

	if users, err := cacheService.SMembers(ctx, leaderboardKey); err == nil {
		fmt.Printf("Top active users: %v\n", users)
	}
}

// RedisHealthCheck demonstrates how to monitor Redis health
func RedisHealthCheck() {
	cfg, _ := config.Load()
	cacheService, _ := cache.CreateCacheService(cfg.Redis.URL)
	defer cacheService.Close()

	ctx := context.Background()

	// Health check
	if err := cacheService.Ping(ctx); err != nil {
		log.Printf("Redis health check failed: %v", err)
	} else {
		fmt.Println("Redis is healthy!")
	}

	// Test basic operations
	testKey := "health_check_test"
	if err := cacheService.Set(ctx, testKey, "ok", 1*time.Minute); err != nil {
		log.Printf("Redis write test failed: %v", err)
	} else {
		fmt.Println("Redis write test passed")
	}

	if value, err := cacheService.Get(ctx, testKey); err != nil {
		log.Printf("Redis read test failed: %v", err)
	} else {
		fmt.Printf("Redis read test passed: %s\n", value)
	}

	// Cleanup
	cacheService.Delete(ctx, testKey)
}
