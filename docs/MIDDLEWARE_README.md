# CS-AI Middleware System

A flexible and powerful middleware system for CS-AI intent handling that allows you to inject custom logic before intent execution.

## Features

- ✅ **Chain of Responsibility Pattern**: Multiple middlewares can be chained together
- ✅ **Priority-based Execution**: Control execution order with priority levels
- ✅ **Intent-specific or Global**: Apply to specific intents or all intents
- ✅ **Function-based Middleware**: Create middlewares using simple functions
- ✅ **Struct-based Middleware**: Create complex middlewares with state and configuration
- ✅ **Context Passing**: Rich context information passed to each middleware
- ✅ **Error Handling**: Stop execution chain on errors
- ✅ **Metadata Support**: Pass custom data between middlewares

## Core Components

### Middleware Interface

```go
type Middleware interface {
    Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error)
    Name() string          // Name of the middleware for logging/debugging
    AppliesTo() []string   // Intent codes this middleware applies to (empty = global)
    Priority() int         // Priority for ordering (lower = earlier execution)
}
```

### Middleware Context

```go
type MiddlewareContext struct {
    SessionID       string                 // Current session ID
    IntentCode      string                 // Intent being executed
    UserMessage     UserMessage            // Original user message
    Parameters      map[string]interface{} // Intent parameters
    StartTime       time.Time              // Execution start time
    Metadata        map[string]interface{} // Custom metadata
    PreviousResults []interface{}          // Results from previous middlewares
}
```

## Usage Examples

### 🚀 Most Practical Way (Recommended)

```go
// Initialize CS-AI
csAI := New("your-api-key", yourModeler)

// Define your intents
var (
    userIntents   []Intent = {...}  // your user intents
    adminIntents  []Intent = {...}  // your admin intents  
    searchIntents []Intent = {...}  // your search intents
)

// 1. Add admin intents with authentication
csAI.AddWithAuth(adminIntents, "admin")

// 2. Add user intents with rate limiting
csAI.AddWithRateLimit(userIntents, 10, time.Minute)

// 3. Add search intents with caching
csAI.AddWithCache(searchIntents, 5*time.Minute)

// 4. Add multiple intents with multiple middlewares
logger := log.New(os.Stdout, "[AI] ", log.LstdFlags)
csAI.AddsWithMiddleware(userIntents, []Middleware{
    NewLoggingMiddleware(logger),
    NewRateLimitMiddleware(5, time.Minute),
})

// 5. Add intents with custom function middleware
csAI.AddsWithFunc(adminIntents, "validator", 15,
    func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
        if len(mctx.Parameters) == 0 {
            return nil, fmt.Errorf("parameters required for %s", mctx.IntentCode)
        }
        return next(ctx, mctx)
    })

// That's it! Much simpler than before 🎉
```

### 🔧 Advanced Usage

```go
// 1. Add multiple intents with shared middleware
authMiddleware := NewAuthenticationMiddleware("user")
csAI.Adds(userIntents, authMiddleware)

// 2. Add middleware to existing intent codes
csAI.AddMiddlewareFuncToIntents([]string{"search", "lookup", "find"}, "cache_middleware", 20,
    func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
        // Custom caching logic here
        return next(ctx, mctx)
    })
```

### 📚 Traditional Way (Still Supported)

```go
// 1. Function-based Global Middleware
csAI.AddGlobalMiddleware("request_logger", 10, func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
    start := time.Now()
    log.Printf("🚀 Starting %s for session %s", mctx.IntentCode, mctx.SessionID)
    
    result, err := next(ctx, mctx)
    
    duration := time.Since(start)
    if err != nil {
        log.Printf("❌ %s failed in %v: %v", mctx.IntentCode, duration, err)
    } else {
        log.Printf("✅ %s completed in %v", mctx.IntentCode, duration)
    }
    
    return result, err
})

// 2. Intent-specific Middleware
csAI.AddMiddlewareFunc("auth_checker", []string{"admin_action", "delete_user"}, 1, 
    func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
        // Check authentication
        if mctx.Metadata["authenticated"] != true {
            return nil, fmt.Errorf("authentication required for %s", mctx.IntentCode)
        }
        
        return next(ctx, mctx)
    })
```

### 3. Struct-based Middleware

```go
// Create custom middleware with state
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
    
    // Check rate limit
    if len(m.requestCounts[sessionID]) >= m.maxRequests {
        return nil, fmt.Errorf("rate limit exceeded")
    }
    
    // Add current request
    m.requestCounts[sessionID] = append(m.requestCounts[sessionID], now)
    
    return next(ctx, mctx)
}

func (m *RateLimitMiddleware) Name() string { return "rate_limiter" }
func (m *RateLimitMiddleware) AppliesTo() []string { return []string{} } // Global
func (m *RateLimitMiddleware) Priority() int { return 5 }

// Use the middleware
rateLimiter := NewRateLimitMiddleware(10, time.Minute)
csAI.AddMiddleware(rateLimiter)
```

## Available Methods

### 🚀 Practical Methods (Recommended)

```go
// Add multiple intents with shared middleware
csAI.Adds(intents []Intent, middleware Middleware)

// Add multiple intents with multiple middlewares
csAI.AddsWithMiddleware(intents []Intent, middlewares []Middleware)

// Add multiple intents with function-based middleware
csAI.AddsWithFunc(intents []Intent, middlewareName string, priority int, handler func(...))

// Add multiple intents with built-in authentication
csAI.AddWithAuth(intents []Intent, requiredRole string)

// Add multiple intents with built-in rate limiting
csAI.AddWithRateLimit(intents []Intent, maxRequests int, timeWindow time.Duration)

// Add multiple intents with built-in caching
csAI.AddWithCache(intents []Intent, ttl time.Duration)

// Add multiple intents with built-in logging
csAI.AddWithLogging(intents []Intent, logger *log.Logger)
```

### 🔧 Advanced Methods

```go
// Add middleware to specific intent codes (without intent objects)
csAI.AddMiddlewareToIntents(intentCodes []string, middleware Middleware)

// Add function-based middleware to specific intent codes
csAI.AddMiddlewareFuncToIntents(intentCodes []string, middlewareName string, priority int, handler func(...))
```

### 🎯 Basic Methods

```go
// Add any middleware that implements the Middleware interface
csAI.AddMiddleware(middleware Middleware)

// Add function-based middleware for specific intents
csAI.AddMiddlewareFunc(name string, appliesTo []string, priority int, handler func(...))

// Add global middleware that applies to all intents
csAI.AddGlobalMiddleware(name string, priority int, handler func(...))
```

## Priority System

Middleware execution order is determined by priority (lower number = higher priority):

- **1-10**: High priority (Authentication, Authorization)
- **11-20**: Medium priority (Validation, Rate limiting)
- **21-30**: Low priority (Logging, Caching, Metrics)

## Common Middleware Patterns

### 1. Authentication Middleware

```go
csAI.AddMiddlewareFunc("auth", []string{"protected_action"}, 1,
    func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
        token := mctx.Metadata["token"]
        if !isValidToken(token) {
            return nil, fmt.Errorf("invalid token")
        }
        return next(ctx, mctx)
    })
```

### 2. Parameter Validation

```go
csAI.AddGlobalMiddleware("validator", 15,
    func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
        if len(mctx.Parameters) == 0 {
            return nil, fmt.Errorf("no parameters provided")
        }
        return next(ctx, mctx)
    })
```

### 3. Caching Middleware

```go
cacheMiddleware := NewCacheMiddleware(5*time.Minute, []string{"search", "lookup"})
csAI.AddMiddleware(cacheMiddleware)
```

### 4. Metrics Collection

```go
csAI.AddGlobalMiddleware("metrics", 25,
    func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
        start := time.Now()
        result, err := next(ctx, mctx)
        
        // Collect metrics
        duration := time.Since(start)
        collectMetrics(mctx.IntentCode, duration, err)
        
        return result, err
    })
```

## Error Handling

If any middleware returns an error, the execution chain stops and the error is returned to the caller. This allows you to:

- Stop execution for authentication failures
- Prevent rate limit violations
- Validate input before processing
- Handle any custom business logic

## Best Practices

1. **Use appropriate priorities**: Auth before validation, validation before caching
2. **Keep middlewares focused**: Each middleware should have a single responsibility
3. **Handle errors gracefully**: Return meaningful error messages
4. **Use metadata for communication**: Pass data between middlewares using `mctx.Metadata`
5. **Test middleware in isolation**: Unit test each middleware separately
6. **Document your middlewares**: Add clear comments explaining what each middleware does

## 🌟 Real-World Example

```go
func SetupRealWorldMiddleware(csAI *CsAI) {
    // Define your intents
    var (
        // Public intents
        loginIntent      Intent = &LoginIntent{}
        registerIntent   Intent = &RegisterIntent{}
        searchIntent     Intent = &SearchIntent{}
        
        // User intents
        profileIntent    Intent = &GetProfileIntent{}
        updateIntent     Intent = &UpdateProfileIntent{}
        
        // Admin intents
        adminReportIntent Intent = &AdminReportIntent{}
        adminDeleteIntent Intent = &AdminDeleteIntent{}
        
        // Sensitive intents
        deleteAccountIntent Intent = &DeleteAccountIntent{}
    )
    
    // 🎯 PRACTICAL SETUP - One-liner for each group
    
    // 1. Public endpoints - just logging
    logger := log.New(os.Stdout, "[PUBLIC] ", log.LstdFlags)
    csAI.AddWithLogging([]Intent{loginIntent, registerIntent, searchIntent}, logger)
    
    // 2. User endpoints - auth + rate limiting
    csAI.AddWithAuth([]Intent{profileIntent, updateIntent}, "user")
    csAI.AddWithRateLimit([]Intent{profileIntent, updateIntent}, 30, time.Minute)
    
    // 3. Admin endpoints - admin auth + enhanced logging + rate limiting
    adminIntents := []Intent{adminReportIntent, adminDeleteIntent}
    csAI.AddWithAuth(adminIntents, "admin")
    csAI.AddWithRateLimit(adminIntents, 100, time.Minute)
    
    adminLogger := log.New(os.Stdout, "[ADMIN] ", log.LstdFlags)
    csAI.AddWithLogging(adminIntents, adminLogger)
    
    // 4. Sensitive endpoints - strict rate limiting
    csAI.AddWithAuth([]Intent{deleteAccountIntent}, "user")
    csAI.AddWithRateLimit([]Intent{deleteAccountIntent}, 3, time.Hour) // Very strict
    
    // 5. Search endpoints - caching for performance
    csAI.AddWithCache([]Intent{searchIntent}, 10*time.Minute)
    
    // 6. Global error handling
    csAI.AddGlobalMiddleware("error_handler", 1,
        func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
            result, err := next(ctx, mctx)
            if err != nil {
                log.Printf("❌ Error in %s: %v", mctx.IntentCode, err)
                // Send to error tracking service
                // errorTracker.Report(err, mctx)
            }
            return result, err
        })
    
    fmt.Println("🚀 Middleware setup completed!")
    fmt.Println("✅ Public endpoints: logging only")
    fmt.Println("✅ User endpoints: auth + rate limiting") 
    fmt.Println("✅ Admin endpoints: admin auth + enhanced logging")
    fmt.Println("✅ Sensitive endpoints: strict rate limiting")
    fmt.Println("✅ Search endpoints: caching enabled")
    fmt.Println("✅ Global error handling active")
}
```

## 🔄 Migration from Old API

```go
// ❌ OLD WAY (verbose)
csAI.AddMiddlewareFunc("auth", []string{"intent1", "intent2", "intent3"}, 1, authHandler)
csAI.AddMiddlewareFunc("rate_limit", []string{"intent1", "intent2", "intent3"}, 5, rateLimitHandler)
csAI.AddMiddlewareFunc("logging", []string{"intent1", "intent2", "intent3"}, 10, loggingHandler)

// ✅ NEW WAY (practical)
intents := []Intent{intent1, intent2, intent3}
csAI.AddWithAuth(intents, "user")
csAI.AddWithRateLimit(intents, 10, time.Minute)
csAI.AddWithLogging(intents, logger)
```

## 🎁 Built-in Middleware Shortcuts

The new API provides built-in shortcuts for common middleware patterns:

- `AddWithAuth()` - Authentication middleware
- `AddWithRateLimit()` - Rate limiting middleware  
- `AddWithCache()` - Caching middleware
- `AddWithLogging()` - Logging middleware
- `AddsWithMiddleware()` - Multiple middlewares at once
- `AddsWithFunc()` - Custom function middleware

This middleware system provides a clean, practical, and flexible way to extend CS-AI functionality without modifying the core intent handling logic. The new API reduces boilerplate code significantly while maintaining full backward compatibility. 