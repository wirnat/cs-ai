# CS-AI Storage System Guide

## Overview

The CS-AI package now supports multiple storage backends through a unified `StorageProvider` interface. This allows you to easily switch between Redis, MongoDB, DynamoDB, and in-memory storage without changing your application code.

## Features

- **Multiple Storage Backends**: Redis, MongoDB, DynamoDB, In-Memory
- **Unified Interface**: Same API for all storage types
- **Backward Compatibility**: Existing Redis code continues to work
- **Easy Migration**: Switch storage types with configuration changes
- **Health Monitoring**: Built-in health checks and statistics
- **TTL Support**: Automatic session expiration

## Available Storage Providers

### 1. Redis Storage (Fully Implemented)
- Production-ready with connection pooling
- Supports authentication and database selection
- Configurable TTL and timeout settings

### 2. In-Memory Storage (Fully Implemented)
- Fast local storage for development/testing
- Built-in cleanup and statistics
- No external dependencies

### 3. MongoDB Storage (Stub Implementation)
- Interface ready, implementation pending
- Requires MongoDB driver dependency
- Planned features: connection pooling, indexes

### 4. DynamoDB Storage (Stub Implementation)
- Interface ready, implementation pending
- Requires AWS SDK dependency
- Planned features: auto-scaling, backup

## Quick Start

### Basic Usage

```go
package main

import (
    "time"
    cs_ai "github.com/wirnat/cs-ai"
    "github.com/wirnat/cs-ai/model"
)

func main() {
    // Create storage configuration
    config := &cs_ai.StorageConfig{
        Type:        cs_ai.StorageTypeInMemory,
        SessionTTL:  1 * time.Hour,
        Timeout:     5 * time.Second,
    }

    // Create CsAI with storage
    cs := cs_ai.New("your-api-key", model.NewDeepSeekChat(), cs_ai.Options{
        StorageConfig: config,
        SessionTTL:   1 * time.Hour,
    })

    // Use normally - storage is handled automatically
    response, err := cs.Exec(context.Background(), "session-123", cs_ai.UserMessage{
        Message: "Hello, how are you?",
    })
}
```

### Redis Storage

```go
// Redis configuration
redisConfig := &cs_ai.StorageConfig{
    Type:          cs_ai.StorageTypeRedis,
    RedisAddress:  "localhost:6379",
    RedisPassword: "your-password",
    RedisDB:       0,
    SessionTTL:    12 * time.Hour,
    Timeout:       5 * time.Second,
}

// Create CsAI with Redis
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: redisConfig,
})
```

### In-Memory Storage

```go
// In-memory configuration
memoryConfig := &cs_ai.StorageConfig{
    Type:        cs_ai.StorageTypeInMemory,
    SessionTTL:  1 * time.Hour,
    Timeout:     1 * time.Second,
}

// Create CsAI with in-memory storage
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: memoryConfig,
})
```

### Custom Storage Provider

```go
// Implement StorageProvider interface
type MyStorage struct {
    // Your storage implementation
}

func (m *MyStorage) GetSessionMessages(ctx context.Context, sessionID string) ([]cs_ai.Message, error) {
    // Your implementation
}

func (m *MyStorage) SaveSessionMessages(ctx context.Context, sessionID string, messages []cs_ai.Message, ttl time.Duration) error {
    // Your implementation
}

// ... implement other methods ...

// Use custom storage
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageProvider: &MyStorage{},
})
```

## Configuration Options

### StorageConfig Fields

```go
type StorageConfig struct {
    Type             StorageType // Storage type (redis, mongo, dynamo, memory)
    SessionTTL       time.Duration // Session expiration time
    Timeout          time.Duration // Operation timeout
    MaxRetries       int          // Maximum retry attempts
    
    // Redis specific
    RedisAddress     string       // Redis server address
    RedisPassword    string       // Redis password
    RedisDB          int          // Redis database number
    
    // MongoDB specific
    MongoURI         string       // MongoDB connection string
    MongoDatabase    string       // Database name
    MongoCollection  string       // Collection name
    
    // DynamoDB specific
    DynamoRegion     string       // AWS region
    DynamoTable      string       // Table name
    DynamoEndpoint   string       // Custom endpoint (for local testing)
}
```

### Storage Types

```go
const (
    StorageTypeRedis   StorageType = "redis"
    StorageTypeMongo   StorageType = "mongo"
    StorageTypeDynamo  StorageType = "dynamo"
    StorageTypeInMemory StorageType = "memory"
)
```

## Migration Guide

### From Redis to MongoDB

1. **Update Configuration**
```go
// Before (Redis)
config := &cs_ai.StorageConfig{
    Type:          cs_ai.StorageTypeRedis,
    RedisAddress:  "localhost:6379",
    SessionTTL:    12 * time.Hour,
}

// After (MongoDB)
config := &cs_ai.StorageConfig{
    Type:             cs_ai.StorageTypeMongo,
    MongoURI:         "mongodb://localhost:27017",
    MongoDatabase:    "cs_ai",
    MongoCollection:  "sessions",
    SessionTTL:       24 * time.Hour,
}
```

2. **No Code Changes Required**
   - All method calls remain the same
   - Storage abstraction handles the differences
   - Session data automatically migrates

### From Redis to In-Memory

```go
// For development/testing
config := &cs_ai.StorageConfig{
    Type:        cs_ai.StorageTypeInMemory,
    SessionTTL:  1 * time.Hour,
}

cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: config,
})
```

## Health Monitoring

### Health Checks

```go
// Check storage health
if err := cs.options.StorageProvider.HealthCheck(); err != nil {
    log.Printf("Storage health check failed: %v", err)
} else {
    log.Println("Storage health check passed")
}
```

### Statistics (In-Memory Only)

```go
if inMemory, ok := cs.options.StorageProvider.(*cs_ai.InMemoryStorageProvider); ok {
    stats := inMemory.GetStorageStats()
    log.Printf("Active sessions: %d", stats.ActiveSessions)
    log.Printf("Total sessions: %d", stats.TotalSessions)
    log.Printf("Memory usage: %d bytes", stats.MemoryUsage)
}
```

## Best Practices

### 1. Choose Storage Based on Use Case

- **Redis**: Production, high-performance, caching
- **MongoDB**: Document storage, complex queries, analytics
- **DynamoDB**: AWS ecosystem, auto-scaling, global distribution
- **In-Memory**: Development, testing, simple applications

### 2. Configure TTL Appropriately

```go
// Short TTL for development
config.SessionTTL = 1 * time.Hour

// Longer TTL for production
config.SessionTTL = 24 * time.Hour

// Very long TTL for analytics
config.SessionTTL = 7 * 24 * time.Hour
```

### 3. Handle Errors Gracefully

```go
messages, err := cs.GetSessionMessages(sessionID)
if err != nil {
    // Log error but continue
    log.Printf("Failed to get session messages: %v", err)
    messages = []cs_ai.Message{} // Use empty messages
}
```

### 4. Monitor Performance

```go
// Check storage performance
start := time.Now()
err := cs.SaveSessionMessages(sessionID, messages)
duration := time.Since(start)

if duration > 100*time.Millisecond {
    log.Printf("Slow storage operation: %v", duration)
}
```

## Troubleshooting

### Common Issues

1. **Storage Provider Not Found**
   - Check `StorageType` constant spelling
   - Ensure required dependencies are installed
   - Verify configuration parameters

2. **Connection Timeouts**
   - Increase `Timeout` value
   - Check network connectivity
   - Verify server addresses

3. **Memory Issues (In-Memory)**
   - Monitor session count
   - Implement cleanup strategies
   - Consider switching to persistent storage

### Debug Mode

```go
// Enable debug logging
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: config,
    LogChatFile:   true, // Log all operations
})
```

## Roadmap

### Phase 1 (Current)
- ✅ Redis storage provider
- ✅ In-memory storage provider
- ✅ Storage interface abstraction
- ✅ Backward compatibility

### Phase 2 (Next)
- 🔄 MongoDB storage provider
- 🔄 DynamoDB storage provider
- 🔄 Connection pooling improvements
- 🔄 Performance optimizations

### Phase 3 (Future)
- 📋 Storage encryption
- 📋 Backup and restore
- 📋 Multi-region support
- 📋 Advanced analytics

## Support

For issues and questions:
- Check the test files for examples
- Review the storage provider implementations
- Open an issue with detailed error information
- Include configuration and environment details

---

**Note**: This storage system is designed to be backward compatible. Existing code using Redis will continue to work without modifications.
