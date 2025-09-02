# Secure CsAI Package

This package provides hardened security features for the CsAI package, including rate limiting, spam detection, and comprehensive logging.

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/wirnat/cs-ai"
)

func main() {
    // Create security configuration
    securityConfig := &cs_ai.SecurityConfig{
        MaxRequestsPerMinute: 10,
        MaxRequestsPerHour:   100,
        MaxRequestsPerDay:    1000,
        SpamThreshold:        0.5,
        EnableLogging:        true,
    }
    
    // Create secure CsAI instance
    secureAI := cs_ai.NewSecureCsAI(
        "your-deepseek-api-key",
        &YourModelImplementation{}, // Your Modeler implementation
        securityConfig,
    )
    
    // Use normally - security features are automatically applied
    ctx := context.Background()
    userMessage := cs_ai.UserMessage{
        Message:         "Hello, how can I help you?",
        ParticipantName: "user123",
    }
    
    response, err := secureAI.Exec(ctx, "session123", userMessage)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Response: %s\n", response.Content)
}
```

### Advanced Configuration

```go
// Custom security settings
securityConfig := &cs_ai.SecurityConfig{
    MaxRequestsPerMinute: 5,    // Stricter limits
    MaxRequestsPerHour:   50,
    MaxRequestsPerDay:    500,
    SpamThreshold:        0.3,  // More sensitive spam detection
    EnableLogging:        true,
}

secureAI := cs_ai.NewSecureCsAI("api-key", model, securityConfig)

// Dynamically adjust settings
secureAI.SetRateLimits(20, 200, 2000)  // Increase limits
secureAI.SetSpamThreshold(0.7)         // Decrease sensitivity
secureAI.EnableSecurityLogging(false)  // Disable logging
```

## Security Features

### Rate Limiting
- **Per-user limits**: Configurable per-minute, per-hour, and per-day limits
- **Automatic cleanup**: Old requests are automatically cleaned up
- **Graceful handling**: Clear error messages when limits are exceeded

### Spam Detection
- **Rule-based detection**: Multiple spam detection rules
- **Configurable threshold**: Adjust sensitivity as needed
- **Real-time blocking**: Immediate spam prevention

### Logging & Analytics
- **Request logging**: Complete audit trail with privacy protection
- **Response logging**: Track all AI responses
- **Analytics**: User behavior and security statistics
- **Privacy protection**: Messages are hashed before logging

## API Reference

### SecurityConfig
```go
type SecurityConfig struct {
    MaxRequestsPerMinute int     // Maximum requests per minute (default: 10)
    MaxRequestsPerHour   int     // Maximum requests per hour (default: 100)
    MaxRequestsPerDay    int     // Maximum requests per day (default: 1000)
    SpamThreshold        float64 // Spam detection threshold 0.0-1.0 (default: 0.5)
    EnableLogging        bool    // Enable security logging (default: true)
    UserIDField          string  // Field to identify user (default: "ParticipantName")
}
```

### SecureCsAI Methods

#### NewSecureCsAI
Creates a new secure CsAI instance with security features.

```go
func NewSecureCsAI(apiKey string, modeler Modeler, config *SecurityConfig, options ...Options) *SecureCsAI
```

#### Exec
Executes a message with security features applied.

```go
func (sc *SecureCsAI) Exec(ctx context.Context, sessionID string, userMessage UserMessage, additionalSystemMessage ...string) (Message, error)
```

#### GetUserAnalytics
Returns analytics for a specific user.

```go
func (sc *SecureCsAI) GetUserAnalytics(userID string, startTime, endTime time.Time) UserAnalytics
```

#### GetAllAnalytics
Returns analytics for all users.

```go
func (sc *SecureCsAI) GetAllAnalytics(startTime, endTime time.Time) []UserAnalytics
```

#### SetRateLimits
Dynamically updates rate limits.

```go
func (sc *SecureCsAI) SetRateLimits(perMinute, perHour, perDay int)
```

#### SetSpamThreshold
Dynamically updates spam detection threshold.

```go
func (sc *SecureCsAI) SetSpamThreshold(threshold float64)
```

#### ResetUserLimits
Resets rate limits for a specific user.

```go
func (sc *SecureCsAI) ResetUserLimits(userID string)
```

## Error Handling

The secure package returns clear error messages:

- **Rate limit exceeded**: `"rate limit exceeded: daily limit exceeded (1000 requests)"`
- **Spam detected**: `"message rejected: spam detected (score: 0.75)"`

## Analytics

### UserAnalytics
```go
type UserAnalytics struct {
    UserID           string
    StartTime        time.Time
    EndTime          time.Time
    TotalRequests    int
    AllowedRequests  int
    BlockedRequests  int
    TotalSpamScore   float64
    AverageSpamScore float64
}
```

### Example Analytics Usage

```go
// Get analytics for last 24 hours
startTime := time.Now().Add(-24 * time.Hour)
endTime := time.Now()

// User-specific analytics
userAnalytics := secureAI.GetUserAnalytics("user123", startTime, endTime)
fmt.Printf("User made %d requests, %d blocked\n", 
    userAnalytics.TotalRequests, 
    userAnalytics.BlockedRequests)

// All users analytics
allAnalytics := secureAI.GetAllAnalytics(startTime, endTime)
for _, analytics := range allAnalytics {
    fmt.Printf("User %s: %d requests, avg spam score: %.2f\n",
        analytics.UserID,
        analytics.TotalRequests,
        analytics.AverageSpamScore)
}
```

## Migration Guide

### From Regular CsAI to Secure CsAI

**Before:**
```go
cs := cs_ai.New("api-key", model, options)
response, err := cs.Exec(ctx, sessionID, userMessage)
```

**After:**
```go
securityConfig := &cs_ai.SecurityConfig{
    MaxRequestsPerMinute: 10,
    MaxRequestsPerHour:   100,
    MaxRequestsPerDay:    1000,
    SpamThreshold:        0.5,
    EnableLogging:        true,
}

secureCS := cs_ai.NewSecureCsAI("api-key", model, securityConfig, options)
response, err := secureCS.Exec(ctx, sessionID, userMessage)
```

## Spam Detection Rules

The spam detector uses the following rules:

1. **Excessive punctuation**: More than 5 consecutive punctuation marks
2. **All caps**: Messages with >80% uppercase characters
3. **Excessive length**: Messages longer than 1000 characters
4. **Repetitive characters**: Repeated characters or patterns
5. **Spam keywords**: Common spam words like "free", "win", "click", etc.

## Privacy

- **Message hashing**: All messages are SHA256 hashed before logging
- **No sensitive data**: Original message content is never stored
- **Configurable logging**: Logging can be disabled entirely

## Performance

- **In-memory storage**: Fast access with automatic cleanup
- **Minimal overhead**: Security checks add <1ms per request
- **Scalable**: Handles thousands of users efficiently