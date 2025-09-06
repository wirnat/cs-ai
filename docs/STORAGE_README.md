# CS-AI Storage System

Package `cs-ai` sekarang mendukung multiple storage backends untuk menyimpan session messages, learning data, dan security logs. Sistem ini memberikan fleksibilitas untuk memilih storage yang sesuai dengan kebutuhan Anda.

## 🚀 Fitur Utama

- **Multiple Storage Backends**: Redis, MongoDB, DynamoDB, In-Memory
- **Backward Compatibility**: Kode lama tetap berfungsi
- **Flexible Configuration**: Mudah dikonfigurasi dan diubah
- **Health Checking**: Monitoring kesehatan storage
- **TTL Support**: Auto-expiration untuk session messages
- **Unified Interface**: Satu interface untuk semua storage types

## 📋 Storage Providers yang Tersedia

### 1. Redis Storage Provider
- **Status**: ✅ Fully Implemented
- **Use Case**: Production, high-performance, caching
- **Features**: TTL support, fast access, persistence

### 2. MongoDB Storage Provider
- **Status**: 🔄 Stub Implementation (ready for implementation)
- **Use Case**: Document-based storage, complex queries, analytics
- **Features**: Schema flexibility, aggregation, indexing

### 3. DynamoDB Storage Provider
- **Status**: 🔄 Stub Implementation (ready for implementation)
- **Use Case**: AWS ecosystem, serverless, scalability
- **Features**: Auto-scaling, pay-per-use, global tables

### 4. In-Memory Storage Provider
- **Status**: ✅ Fully Implemented
- **Use Case**: Testing, development, prototyping
- **Features**: Fast access, no external dependencies, statistics

## 🛠️ Cara Penggunaan

### Basic Usage

```go
import "github.com/wirnat/cs-ai"

// 1. Redis Storage
redisConfig := &cs_ai.StorageConfig{
    Type:          cs_ai.StorageTypeRedis,
    RedisAddress:  "localhost:6379",
    RedisPassword: "",
    RedisDB:       0,
    SessionTTL:    12 * time.Hour,
    Timeout:       5 * time.Second,
}

cs := cs_ai.New("your-api-key", model, cs_ai.Options{
    StorageConfig: redisConfig,
    SessionTTL:   12 * time.Hour,
})

// 2. MongoDB Storage
mongoConfig := &cs_ai.StorageConfig{
    Type:             cs_ai.StorageTypeMongo,
    MongoURI:         "mongodb://localhost:27017",
    MongoDatabase:    "cs_ai",
    MongoCollection:  "sessions",
    SessionTTL:       24 * time.Hour,
    Timeout:          10 * time.Second,
}

cs := cs_ai.New("your-api-key", model, cs_ai.Options{
    StorageConfig: mongoConfig,
    SessionTTL:   24 * time.Hour,
})

// 3. In-Memory Storage (untuk testing)
memoryConfig := &cs_ai.StorageConfig{
    Type:        cs_ai.StorageTypeInMemory,
    SessionTTL:  1 * time.Hour,
    Timeout:     1 * time.Second,
}

cs := cs_ai.New("your-api-key", model, cs_ai.Options{
    StorageConfig: memoryConfig,
    SessionTTL:   1 * time.Hour,
})
```

### Custom Storage Provider

```go
// Implement StorageProvider interface
type CustomStorage struct {
    // Your custom fields
}

func (c *CustomStorage) GetSessionMessages(ctx context.Context, sessionID string) ([]Message, error) {
    // Your implementation
}

func (c *CustomStorage) SaveSessionMessages(ctx context.Context, sessionID string, messages []Message, ttl time.Duration) error {
    // Your implementation
}

// ... implement other methods

// Use custom storage
customStorage := &CustomStorage{}
cs := cs_ai.New("your-api-key", model, cs_ai.Options{
    StorageProvider: customStorage,
    SessionTTL:     12 * time.Hour,
})
```

### Backward Compatibility

```go
// Kode lama tetap berfungsi
cs := cs_ai.New("your-api-key", model, cs_ai.Options{
    Redis:       redisClient, // Legacy Redis support
    SessionTTL:  12 * time.Hour,
})
```

## 🔧 Konfigurasi Storage

### StorageConfig Structure

```go
type StorageConfig struct {
    Type StorageType // Storage type (redis, mongo, dynamo, memory)
    
    // Redis configuration
    RedisAddress  string
    RedisPassword string
    RedisDB       int
    
    // MongoDB configuration
    MongoURI      string
    MongoDatabase string
    MongoCollection string
    
    // DynamoDB configuration
    AWSRegion    string
    DynamoTable  string
    
    // Common configuration
    SessionTTL   time.Duration // TTL untuk session messages
    MaxRetries   int           // Maximum retry attempts
    Timeout      time.Duration // Connection timeout
}
```

### Storage Type Constants

```go
const (
    StorageTypeRedis   StorageType = "redis"
    StorageTypeMongo   StorageType = "mongo"
    StorageTypeDynamo  StorageType = "dynamo"
    StorageTypeInMemory StorageType = "memory"
)
```

## 📊 Storage Operations

### Session Management

```go
// Get session messages
messages, err := cs.GetSessionMessages("session123")

// Save session messages (auto-handled by storage provider)
err := cs.SaveSessionMessages("session123", messages)

// Delete session (if needed)
if storage, ok := cs.options.StorageProvider.(cs_ai.StorageProvider); ok {
    err := storage.DeleteSession(ctx, "session123")
}
```

### Learning Data

```go
// Save learning data
if storage, ok := cs.options.StorageProvider.(cs_ai.StorageProvider); ok {
    learningData := cs_ai.LearningData{
        Query:     "user question",
        Response:  "ai response",
        Tools:     []string{"tool1", "tool2"},
        Context:   map[string]interface{}{"key": "value"},
        Timestamp: time.Now(),
        Feedback:  1,
    }
    err := storage.SaveLearningData(ctx, learningData)
}

// Get learning data
if storage, ok := cs.options.StorageProvider.(cs_ai.StorageProvider); ok {
    data, err := storage.GetLearningData(ctx, 7) // Last 7 days
}
```

### Security Logs

```go
// Save security log
if storage, ok := cs.options.StorageProvider.(cs_ai.StorageProvider); ok {
    securityLog := cs_ai.SecurityLog{
        SessionID:   "session123",
        UserID:      "user123",
        MessageHash: "hash123",
        Timestamp:   time.Now(),
        SpamScore:   0.1,
        Allowed:     true,
        Error:       "",
    }
    err := storage.SaveSecurityLog(ctx, securityLog)
}

// Get security logs
if storage, ok := cs.options.StorageProvider.(cs_ai.StorageProvider); ok {
    startTime := time.Now().Add(-24 * time.Hour)
    endTime := time.Now()
    logs, err := storage.GetSecurityLogs(ctx, "user123", startTime, endTime)
}
```

## 🔍 Health Checking & Monitoring

### Health Check

```go
if cs.options.StorageProvider != nil {
    err := cs.options.StorageProvider.HealthCheck()
    if err != nil {
        log.Printf("Storage health check failed: %v", err)
        // Handle storage failure
    }
}
```

### Storage Statistics (In-Memory Only)

```go
if inMemoryStorage, ok := cs.options.StorageProvider.(*cs_ai.InMemoryStorageProvider); ok {
    stats := inMemoryStorage.GetStorageStats()
    log.Printf("Storage stats: %+v", stats)
    
    // Output example:
    // {
    //   "storage_type": "in_memory",
    //   "total_sessions": 150,
    //   "total_learning_data": 45,
    //   "total_security_logs": 89,
    //   "memory_usage_mb": 0.25
    // }
}
```

## 🚀 Migration Guide

### Dari Redis ke MongoDB

```go
// Step 1: Mulai dengan Redis
redisConfig := &cs_ai.StorageConfig{
    Type:          cs_ai.StorageTypeRedis,
    RedisAddress:  "localhost:6379",
    RedisPassword: "",
    RedisDB:       0,
    SessionTTL:    12 * time.Hour,
}

cs := cs_ai.New("your-api-key", model, cs_ai.Options{
    StorageConfig: redisConfig,
})

// Step 2: Migrasi ke MongoDB
mongoConfig := &cs_ai.StorageConfig{
    Type:             cs_ai.StorageTypeMongo,
    MongoURI:         "mongodb://localhost:27017",
    MongoDatabase:    "cs_ai",
    MongoCollection:  "sessions",
    SessionTTL:       24 * time.Hour,
}

cs = cs_ai.New("your-api-key", model, cs_ai.Options{
    StorageConfig: mongoConfig,
})
```

### Dari Redis ke In-Memory (untuk testing)

```go
// Development/testing environment
memoryConfig := &cs_ai.StorageConfig{
    Type:        cs_ai.StorageTypeInMemory,
    SessionTTL:  1 * time.Hour,
    Timeout:     1 * time.Second,
}

cs := cs_ai.New("your-api-key", model, cs_ai.Options{
    StorageConfig: memoryConfig,
})
```

## 🧪 Testing dengan In-Memory Storage

```go
func TestCsAIWithInMemoryStorage(t *testing.T) {
    // Setup in-memory storage
    memoryConfig := &cs_ai.StorageConfig{
        Type:        cs_ai.StorageTypeInMemory,
        SessionTTL:  1 * time.Hour,
    }
    
    cs := cs_ai.New("test-api-key", testModel, cs_ai.Options{
        StorageConfig: memoryConfig,
    })
    
    // Test session operations
    sessionID := "test-session"
    messages := []cs_ai.Message{
        {Content: "Hello", Role: cs_ai.User},
        {Content: "Hi there!", Role: cs_ai.Assistant},
    }
    
    // Save messages
    err := cs.SaveSessionMessages(sessionID, messages)
    assert.NoError(t, err)
    
    // Retrieve messages
    retrieved, err := cs.GetSessionMessages(sessionID)
    assert.NoError(t, err)
    assert.Equal(t, len(messages), len(retrieved))
    
    // Check storage stats
    if inMemoryStorage, ok := cs.options.StorageProvider.(*cs_ai.InMemoryStorageProvider); ok {
        stats := inMemoryStorage.GetStorageStats()
        assert.Equal(t, 1, stats["total_sessions"])
    }
}
```

## 📝 Best Practices

### 1. Production Environment
- Gunakan Redis untuk high-performance dan caching
- Set TTL yang sesuai dengan business requirements
- Monitor storage health secara berkala

### 2. Development Environment
- Gunakan In-Memory storage untuk testing
- Set TTL pendek untuk development
- Test dengan berbagai storage types

### 3. Migration
- Test storage baru di environment terpisah
- Backup data sebelum migrasi
- Monitor performance setelah migrasi

### 4. Error Handling
- Selalu check storage health
- Implement fallback mechanism
- Log storage errors untuk debugging

## 🔮 Roadmap

### Phase 1 (Current) ✅
- [x] Storage interface design
- [x] Redis storage provider
- [x] In-memory storage provider
- [x] Backward compatibility
- [x] Basic configuration

### Phase 2 (Next) 🔄
- [ ] MongoDB storage provider implementation
- [ ] DynamoDB storage provider implementation
- [ ] Advanced querying capabilities
- [ ] Bulk operations support

### Phase 3 (Future) 📋
- [ ] PostgreSQL storage provider
- [ ] Elasticsearch storage provider
- [ ] Multi-storage support (hybrid)
- [ ] Data migration tools
- [ ] Advanced analytics

## 🆘 Troubleshooting

### Common Issues

1. **Storage Connection Failed**
   - Check connection parameters
   - Verify network connectivity
   - Check authentication credentials

2. **Session Not Found**
   - Verify TTL settings
   - Check storage provider health
   - Verify session ID format

3. **Performance Issues**
   - Monitor storage statistics
   - Check TTL settings
   - Consider storage type for use case

### Debug Commands

```go
// Check storage provider type
fmt.Printf("Storage type: %s\n", cs.options.StorageConfig.Type)

// Check storage health
if cs.options.StorageProvider != nil {
    err := cs.options.StorageProvider.HealthCheck()
    fmt.Printf("Storage health: %v\n", err)
}

// Get storage stats (in-memory only)
if inMemoryStorage, ok := cs.options.StorageProvider.(*cs_ai.InMemoryStorageProvider); ok {
    stats := inMemoryStorage.GetStorageStats()
    fmt.Printf("Storage stats: %+v\n", stats)
}
```

## 📚 Additional Resources

- [Redis Documentation](https://redis.io/documentation)
- [MongoDB Documentation](https://docs.mongodb.com/)
- [AWS DynamoDB Documentation](https://docs.aws.amazon.com/dynamodb/)
- [Go Context Package](https://golang.org/pkg/context/)

---

**Note**: Sistem storage ini memberikan fleksibilitas maksimal sambil mempertahankan backward compatibility. Pilih storage provider yang sesuai dengan kebutuhan Anda dan environment yang tersedia.
