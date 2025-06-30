package cs_ai

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// ============================== MIDDLEWARE EXAMPLES ==============================

// LoggingMiddleware is an example middleware that logs intent execution
type LoggingMiddleware struct {
	logger *log.Logger
}

func NewLoggingMiddleware(logger *log.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{logger: logger}
}

func (m *LoggingMiddleware) Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
	start := time.Now()
	m.logger.Printf("[MIDDLEWARE] Starting execution of intent: %s for session: %s", mctx.IntentCode, mctx.SessionID)

	// Execute next middleware or intent handler
	result, err := next(ctx, mctx)

	duration := time.Since(start)
	if err != nil {
		m.logger.Printf("[MIDDLEWARE] Intent %s failed after %v: %v", mctx.IntentCode, duration, err)
	} else {
		m.logger.Printf("[MIDDLEWARE] Intent %s completed successfully in %v", mctx.IntentCode, duration)
	}

	return result, err
}

func (m *LoggingMiddleware) Name() string {
	return "logging_middleware"
}

func (m *LoggingMiddleware) AppliesTo() []string {
	return []string{} // Global middleware - applies to all intents
}

func (m *LoggingMiddleware) Priority() int {
	return 10 // Low priority - executes early
}

// AuthenticationMiddleware is an example middleware that checks authentication
type AuthenticationMiddleware struct {
	requiredRole string
}

func NewAuthenticationMiddleware(requiredRole string) *AuthenticationMiddleware {
	return &AuthenticationMiddleware{requiredRole: requiredRole}
}

func (m *AuthenticationMiddleware) Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
	// Check if user has required role (example implementation)
	userRole := mctx.Metadata["user_role"]
	if userRole == nil {
		return nil, fmt.Errorf("authentication required for intent: %s", mctx.IntentCode)
	}

	if userRole != m.requiredRole && m.requiredRole != "" {
		return nil, fmt.Errorf("insufficient permissions for intent: %s, required: %s", mctx.IntentCode, m.requiredRole)
	}

	// Continue with next middleware or intent handler
	return next(ctx, mctx)
}

func (m *AuthenticationMiddleware) Name() string {
	return "authentication_middleware"
}

func (m *AuthenticationMiddleware) AppliesTo() []string {
	return []string{"sensitive_operation", "admin_only"} // Only applies to specific intents
}

func (m *AuthenticationMiddleware) Priority() int {
	return 1 // High priority - executes first
}

// RateLimitMiddleware is an example middleware that implements rate limiting
type RateLimitMiddleware struct {
	requestCounts map[string][]time.Time
	maxRequests   int
	timeWindow    time.Duration
}

func NewRateLimitMiddleware(maxRequests int, timeWindow time.Duration) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		requestCounts: make(map[string][]time.Time),
		maxRequests:   maxRequests,
		timeWindow:    timeWindow,
	}
}

func (m *RateLimitMiddleware) Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
	sessionID := mctx.SessionID
	now := time.Now()

	// Clean old requests outside time window
	if requests, exists := m.requestCounts[sessionID]; exists {
		validRequests := make([]time.Time, 0)
		for _, reqTime := range requests {
			if now.Sub(reqTime) <= m.timeWindow {
				validRequests = append(validRequests, reqTime)
			}
		}
		m.requestCounts[sessionID] = validRequests
	}

	// Check if rate limit exceeded
	if len(m.requestCounts[sessionID]) >= m.maxRequests {
		return nil, fmt.Errorf("rate limit exceeded for session: %s. Max %d requests per %v",
			sessionID, m.maxRequests, m.timeWindow)
	}

	// Add current request
	m.requestCounts[sessionID] = append(m.requestCounts[sessionID], now)

	// Continue with next middleware or intent handler
	return next(ctx, mctx)
}

func (m *RateLimitMiddleware) Name() string {
	return "rate_limit_middleware"
}

func (m *RateLimitMiddleware) AppliesTo() []string {
	return []string{} // Global middleware
}

func (m *RateLimitMiddleware) Priority() int {
	return 5 // Medium priority
}

// CacheMiddleware is an example middleware that implements caching
type CacheMiddleware struct {
	cache     map[string]CacheEntry
	ttl       time.Duration
	appliesTo []string
}

type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
}

func NewCacheMiddleware(ttl time.Duration, appliesTo []string) *CacheMiddleware {
	return &CacheMiddleware{
		cache:     make(map[string]CacheEntry),
		ttl:       ttl,
		appliesTo: appliesTo,
	}
}

func (m *CacheMiddleware) Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
	// Create cache key based on intent and parameters
	cacheKey := fmt.Sprintf("%s:%s:%v", mctx.SessionID, mctx.IntentCode, mctx.Parameters)

	// Check cache
	if entry, exists := m.cache[cacheKey]; exists && time.Now().Before(entry.ExpiresAt) {
		log.Printf("[CACHE] Cache hit for key: %s", cacheKey)
		return entry.Value, nil
	}

	// Execute next middleware or intent handler
	result, err := next(ctx, mctx)
	if err != nil {
		return result, err
	}

	// Store in cache
	m.cache[cacheKey] = CacheEntry{
		Value:     result,
		ExpiresAt: time.Now().Add(m.ttl),
	}

	log.Printf("[CACHE] Cached result for key: %s", cacheKey)
	return result, err
}

func (m *CacheMiddleware) Name() string {
	return "cache_middleware"
}

func (m *CacheMiddleware) AppliesTo() []string {
	return m.appliesTo
}

func (m *CacheMiddleware) Priority() int {
	return 15 // Low priority - executes after auth/rate limiting
}

// ============================== USAGE EXAMPLES ==============================

// ExamplePracticalUsage demonstrates the most practical ways to use the middleware system
func ExamplePracticalUsage() {
	// Initialize CS-AI
	csAI := New("your-api-key", nil /* your modeler */)

	// Assume you have some intents
	var (
		userIntent1   Intent // your user intent 1
		userIntent2   Intent // your user intent 2
		adminIntent1  Intent // your admin intent 1
		adminIntent2  Intent // your admin intent 2
		searchIntent  Intent // your search intent
		publicIntent1 Intent // your public intent 1
		publicIntent2 Intent // your public intent 2
	)

	// 🚀 MOST PRACTICAL WAY: Add multiple intents with different middleware combinations

	// 1. Add admin intents with authentication
	csAI.AddWithAuth([]Intent{adminIntent1, adminIntent2}, "admin")

	// 2. Add user intents with rate limiting
	csAI.AddWithRateLimit([]Intent{userIntent1, userIntent2}, 10, time.Minute)

	// 3. Add search intents with caching
	csAI.AddWithCache([]Intent{searchIntent}, 5*time.Minute)

	// 4. Add public intents with logging
	logger := log.New(os.Stdout, "[AI] ", log.LstdFlags)
	csAI.AddWithLogging([]Intent{publicIntent1, publicIntent2}, logger)

	// 5. Add multiple intents with multiple middlewares
	csAI.AddsWithMiddleware([]Intent{userIntent1, userIntent2}, []Middleware{
		NewLoggingMiddleware(logger),
		NewRateLimitMiddleware(5, time.Minute),
	})

	// 6. Add intents with custom function middleware
	csAI.AddsWithFunc([]Intent{adminIntent1, userIntent1}, "validator", 15,
		func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
			if len(mctx.Parameters) == 0 {
				return nil, fmt.Errorf("parameters required for %s", mctx.IntentCode)
			}
			return next(ctx, mctx)
		})

	fmt.Println("✨ Practical middleware setup completed!")
}

// ExampleAdvancedUsage shows advanced middleware patterns
func ExampleAdvancedUsage() {
	csAI := New("your-api-key", nil /* your modeler */)

	// Assume you have intents
	var intents []Intent // your list of intents

	// 1. Chain multiple middlewares for the same intents
	authMiddleware := NewAuthenticationMiddleware("user")
	rateLimitMiddleware := NewRateLimitMiddleware(10, time.Minute)

	csAI.Adds(intents, authMiddleware)
	csAI.Adds(intents, rateLimitMiddleware)

	// 2. Add middleware to existing intent codes (without intent objects)
	csAI.AddMiddlewareFuncToIntents([]string{"search", "lookup", "find"}, "cache_middleware", 20,
		func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
			// Custom caching logic here
			return next(ctx, mctx)
		})

	// 3. Conditional middleware
	csAI.AddGlobalMiddleware("conditional_auth", 5,
		func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
			// Only apply auth for sensitive operations
			sensitiveOperations := []string{"delete", "update", "create"}
			for _, op := range sensitiveOperations {
				if strings.Contains(mctx.IntentCode, op) {
					if mctx.Metadata["token"] == nil {
						return nil, fmt.Errorf("authentication required for %s", mctx.IntentCode)
					}
					break
				}
			}
			return next(ctx, mctx)
		})

	fmt.Println("🔧 Advanced middleware setup completed!")
}

// ExampleUsage demonstrates how to use the middleware system (legacy)
func ExampleUsage() {
	// Initialize CS-AI
	csAI := New("your-api-key", nil /* your modeler */)

	// 1. Add global logging middleware
	csAI.AddGlobalMiddleware("request_logger", 10, func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
		start := time.Now()
		fmt.Printf("🚀 Starting %s for session %s\n", mctx.IntentCode, mctx.SessionID)

		result, err := next(ctx, mctx)

		duration := time.Since(start)
		if err != nil {
			fmt.Printf("❌ %s failed in %v: %v\n", mctx.IntentCode, duration, err)
		} else {
			fmt.Printf("✅ %s completed in %v\n", mctx.IntentCode, duration)
		}

		return result, err
	})

	// 2. Add authentication middleware for specific intents
	csAI.AddMiddlewareFunc("auth_checker", []string{"admin_action", "delete_user"}, 1,
		func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
			// Check authentication
			if mctx.Metadata["authenticated"] != true {
				return nil, fmt.Errorf("authentication required for %s", mctx.IntentCode)
			}

			return next(ctx, mctx)
		})

	// 3. Add rate limiting middleware
	rateLimiter := NewRateLimitMiddleware(10, time.Minute)
	csAI.AddMiddleware(rateLimiter)

	// 4. Add caching middleware for specific intents
	cacheMiddleware := NewCacheMiddleware(5*time.Minute, []string{"search_data", "get_info"})
	csAI.AddMiddleware(cacheMiddleware)

	// 5. Add custom validation middleware
	csAI.AddMiddlewareFunc("parameter_validator", []string{}, 20,
		func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
			// Validate parameters before execution
			if len(mctx.Parameters) == 0 {
				return nil, fmt.Errorf("no parameters provided for %s", mctx.IntentCode)
			}

			// Add metadata for next middlewares
			mctx.Metadata["validated"] = true

			return next(ctx, mctx)
		})

	fmt.Println("✨ Middleware system configured successfully!")
}

// ExampleRealWorldScenario shows a real-world usage scenario
func ExampleRealWorldScenario() {
	csAI := New("your-api-key", nil /* your modeler */)

	// Define your intents (example placeholders)
	var (
		loginIntent          Intent
		logoutIntent         Intent
		getUserProfileIntent Intent
		updateProfileIntent  Intent
		deleteAccountIntent  Intent
		searchProductsIntent Intent
		getProductIntent     Intent
		adminReportIntent    Intent
		adminDeleteIntent    Intent
	)

	// 🎯 Real-world middleware setup

	// 1. Public endpoints - just logging
	logger := log.New(os.Stdout, "[PUBLIC] ", log.LstdFlags)
	csAI.AddWithLogging([]Intent{loginIntent, logoutIntent, searchProductsIntent, getProductIntent}, logger)

	// 2. User endpoints - authentication + rate limiting
	userIntents := []Intent{getUserProfileIntent, updateProfileIntent}
	csAI.AddWithAuth(userIntents, "user")
	csAI.AddWithRateLimit(userIntents, 30, time.Minute) // 30 requests per minute

	// 3. Sensitive user endpoints - authentication + stricter rate limiting
	csAI.AddWithAuth([]Intent{deleteAccountIntent}, "user")
	csAI.AddWithRateLimit([]Intent{deleteAccountIntent}, 5, time.Hour) // 5 requests per hour

	// 4. Admin endpoints - admin auth + logging + rate limiting
	adminIntents := []Intent{adminReportIntent, adminDeleteIntent}
	csAI.AddWithAuth(adminIntents, "admin")
	csAI.AddWithRateLimit(adminIntents, 100, time.Minute)

	adminLogger := log.New(os.Stdout, "[ADMIN] ", log.LstdFlags)
	csAI.AddWithLogging(adminIntents, adminLogger)

	// 5. Search endpoints - caching for better performance
	csAI.AddWithCache([]Intent{searchProductsIntent, getProductIntent}, 10*time.Minute)

	// 6. Add global error handling middleware
	csAI.AddGlobalMiddleware("error_handler", 1,
		func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
			result, err := next(ctx, mctx)
			if err != nil {
				// Log error details
				log.Printf("Error in %s for session %s: %v", mctx.IntentCode, mctx.SessionID, err)

				// You could also send to error tracking service here
				// errorTracker.Report(err, mctx)
			}
			return result, err
		})

	fmt.Println("🌟 Real-world middleware setup completed!")
	fmt.Println("📊 Configured:")
	fmt.Println("   - Public endpoints with logging")
	fmt.Println("   - User endpoints with auth + rate limiting")
	fmt.Println("   - Admin endpoints with admin auth + enhanced logging")
	fmt.Println("   - Search endpoints with caching")
	fmt.Println("   - Global error handling")
}

// ExampleCustomMiddleware shows how to create a custom middleware struct
func ExampleCustomMiddleware() {
	// Create custom middleware
	logger := log.New(nil, "[AI] ", log.LstdFlags)
	loggingMiddleware := NewLoggingMiddleware(logger)

	authMiddleware := NewAuthenticationMiddleware("admin")

	// Initialize CS-AI and add middlewares
	csAI := New("your-api-key", nil /* your modeler */)
	csAI.AddMiddleware(loggingMiddleware)
	csAI.AddMiddleware(authMiddleware)

	fmt.Println("🔧 Custom middleware added successfully!")
}
