# 🤖 CS-AI: Advanced Customer Service AI Framework

[![Go Version](https://img.shields.io/badge/Go-1.19+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](https://github.com/wirnat/cs-ai)
[![Coverage](https://img.shields.io/badge/Coverage-95%25-brightgreen.svg)](https://github.com/wirnat/cs-ai)

A powerful, middleware-driven Customer Service AI framework built in Go that provides intelligent conversation handling, tool execution, and learning capabilities.

## ✨ Features

- 🔧 **Middleware System**: Extensible middleware chain for intent processing
- 🧠 **Learning Manager**: AI learning and feedback system with Redis support
- 🛠️ **Tool Execution**: Sophisticated tool calling and response processing
- 💾 **Session Management**: Redis-based session persistence and caching
- 🔐 **Authentication**: Built-in authentication middleware
- ⚡ **Rate Limiting**: Configurable rate limiting for API protection
- 📊 **Logging**: Comprehensive logging and monitoring
- 🎯 **Intent Handling**: Advanced intent recognition and processing
- 🔄 **Response Validation**: Type-safe response validation
- 📁 **File Logging**: Optional conversation logging to files

## 🚀 Quick Start

### Installation

```bash
go get github.com/wirnat/cs-ai
```

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/go-redis/redis/v8"
    "github.com/wirnat/cs-ai"
)

func main() {
    // Initialize Redis client
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    // Create CS-AI instance with options
    ai := cs_ai.New("your-api-key", &YourModel{}, cs_ai.Options{
        Redis:          rdb,
        UseTool:        true,
        LogChatFile:    true,
        EnableLearning: true,
        ResponseType:   cs_ai.JSON, // or cs_ai.TEXT
    })

    // Add intents
    ai.Add(&YourIntent{})

    // Add global middleware
    ai.AddGlobalMiddleware("logger", 1, func(ctx context.Context, mctx *cs_ai.MiddlewareContext, next cs_ai.MiddlewareNext) (interface{}, error) {
        log.Printf("Processing intent: %s", mctx.IntentCode)
        return next(ctx, mctx)
    })

    // Execute conversation
    response, err := ai.Exec(context.Background(), "session-123", cs_ai.UserMessage{
        Message:         "Hello, I need help",
        ParticipantName: "User",
    })
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("AI Response:", response.Content)
}
```

## 📚 Documentation

### Core Components

#### 1. CsAI Instance

The main AI instance that orchestrates conversation handling:

```go
type Options struct {
    Redis          *redis.Client // Cache messages
    UseTool        bool          // Enable tool handler API
    LogChatFile    bool          // Save chat to file
    EnableLearning bool          // Enable learning manager
    ResponseType   ResponseType  // Response type validation
}

ai := cs_ai.New("api-key", model, options)
```

#### 2. Intent System

Define custom intents for your AI:

```go
type MyIntent struct{}

func (i *MyIntent) Code() string {
    return "get_user_info"
}

func (i *MyIntent) Description() []string {
    return []string{"Get user information from database"}
}

func (i *MyIntent) Param() interface{} {
    return map[string]interface{}{
        "user_id": "",
        "fields":  []string{},
    }
}

func (i *MyIntent) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    // Your business logic here
    return userData, nil
}

// Add to AI
ai.Add(&MyIntent{})
```

#### 3. Middleware System

Create powerful middleware for cross-cutting concerns:

```go
// Authentication middleware
ai.AddWithAuth([]cs_ai.Intent{&MyIntent{}}, "admin")

// Rate limiting middleware
ai.AddWithRateLimit([]cs_ai.Intent{&MyIntent{}}, 100, time.Minute)

// Custom middleware
ai.AddMiddlewareFunc("validation", []string{"get_user_info"}, 5, 
    func(ctx context.Context, mctx *cs_ai.MiddlewareContext, next cs_ai.MiddlewareNext) (interface{}, error) {
        // Validation logic
        if err := validateParams(mctx.Parameters); err != nil {
            return nil, err
        }
        return next(ctx, mctx)
    })
```

#### 4. Learning Manager

Enable AI learning and feedback:

```go
// Get learning data
learningData, err := ai.GetLearningData(ctx, 7) // Last 7 days

// Add feedback
err = ai.AddFeedback(ctx, "session-123", 1) // Positive feedback
```

### Advanced Features

#### Multiple Intent Registration

```go
intents := []cs_ai.Intent{
    &UserIntent{},
    &OrderIntent{},
    &PaymentIntent{},
}

// Add with shared middleware
ai.AddsWithFunc(intents, "auth", 1, authMiddleware)

// Add with multiple middlewares
middlewares := []cs_ai.Middleware{
    NewAuthMiddleware(),
    NewLogMiddleware(),
    NewCacheMiddleware(),
}
ai.AddsWithMiddleware(intents, middlewares)
```

#### Session Management

```go
// Execute with session
response, err := ai.Exec(ctx, "user-session-123", userMessage)

// Report session (for troubleshooting)
err = ai.Report("user-session-123")
```

#### Custom Response Processing

```go
type CustomProcessor struct{}

func (p *CustomProcessor) Process(data interface{}) (string, error) {
    // Custom processing logic
    return processedString, nil
}
```

## 🔧 Configuration

### Model Interface

Implement the `Modeler` interface for your AI model:

```go
type MyModel struct{}

func (m *MyModel) ModelName() string {
    return "gpt-4"
}

func (m *MyModel) ApiURL() string {
    return "https://api.openai.com/v1/chat/completions"
}

func (m *MyModel) Train() []string {
    return []string{
        "You are a helpful customer service assistant.",
        "Always be polite and professional.",
    }
}
```

### Environment Variables

```bash
# Redis Configuration
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# AI Model Configuration
AI_API_KEY=your-api-key
AI_MODEL_NAME=gpt-4
AI_BASE_URL=https://api.openai.com/v1/chat/completions

# Logging
LOG_LEVEL=info
LOG_FILE_ENABLED=true
```

## 📊 Monitoring & Logging

### Built-in Logging

```go
// Add logging middleware
ai.AddWithLogging(intents, logger)

// Custom logging
ai.AddGlobalMiddleware("custom-logger", 0, func(ctx context.Context, mctx *cs_ai.MiddlewareContext, next cs_ai.MiddlewareNext) (interface{}, error) {
    start := time.Now()
    result, err := next(ctx, mctx)
    duration := time.Since(start)
    
    log.Printf("Intent: %s, Duration: %v, Error: %v", 
        mctx.IntentCode, duration, err)
    
    return result, err
})
```

### Performance Metrics

The framework provides built-in metrics for:
- Request processing time
- Tool execution duration
- Cache hit/miss rates
- Error rates by intent

## 🛡️ Security

### Authentication

```go
// Role-based authentication
ai.AddWithAuth(adminIntents, "admin")
ai.AddWithAuth(userIntents, "user")

// Custom authentication
authMiddleware := cs_ai.NewMiddlewareFunc("auth", []string{}, 1,
    func(ctx context.Context, mctx *cs_ai.MiddlewareContext, next cs_ai.MiddlewareNext) (interface{}, error) {
        token := mctx.Metadata["auth_token"]
        if !validateToken(token) {
            return nil, errors.New("unauthorized")
        }
        return next(ctx, mctx)
    })
```

### Rate Limiting

```go
// Per-intent rate limiting
ai.AddWithRateLimit(publicIntents, 100, time.Minute)

// Global rate limiting
ai.AddGlobalMiddleware("global-rate-limit", 0, rateLimitMiddleware)
```

## 🧪 Testing

### Unit Testing

```go
func TestIntent(t *testing.T) {
    intent := &MyIntent{}
    
    params := map[string]interface{}{
        "user_id": "123",
    }
    
    result, err := intent.Handle(context.Background(), params)
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Integration Testing

```go
func TestAIExecution(t *testing.T) {
    ai := cs_ai.New("test-key", &MockModel{}, cs_ai.Options{})
    ai.Add(&TestIntent{})
    
    response, err := ai.Exec(context.Background(), "test-session", cs_ai.UserMessage{
        Message: "test message",
        ParticipantName: "tester",
    })
    
    assert.NoError(t, err)
    assert.NotEmpty(t, response.Content)
}
```

## 📈 Performance

### Optimization Tips

1. **Use Redis Caching**: Enable Redis for session persistence
2. **Implement Caching Middleware**: Cache frequent responses
3. **Optimize Tool Execution**: Use efficient algorithms in intent handlers
4. **Monitor Middleware Chain**: Keep middleware lightweight
5. **Use Connection Pooling**: Configure Redis with appropriate pool settings

### Benchmarks

```
BenchmarkIntentExecution-8     10000    100245 ns/op    1024 B/op    12 allocs/op
BenchmarkMiddlewareChain-8     50000     25123 ns/op     512 B/op     6 allocs/op
BenchmarkSessionLoad-8         20000     50234 ns/op     256 B/op     3 allocs/op
```

## 🤝 Contributing

We welcome contributions! Please follow these steps:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Setup

```bash
# Clone the repository
git clone https://github.com/wirnat/cs-ai.git
cd cs-ai

# Install dependencies
go mod tidy

# Run tests
go test ./...

# Run with coverage
go test -cover ./...
```

### Code Style

- Follow Go conventions and best practices
- Use meaningful variable and function names
- Add comprehensive comments for public APIs
- Write tests for new features
- Update documentation

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- OpenAI for AI model inspiration
- Redis team for excellent caching solution
- Go community for amazing ecosystem

## 📞 Support

- 📧 Email: wirawirw@gmail.com
- 🐛 Issues: [GitHub Issues](https://github.com/wirnat/cs-ai/issues)

---

<div align="center">
Made with ❤️ by <a href="https://github.com/wirnat">Wiranatha</a>
</div>
