package cs_ai

import (
	"context"
	"time"
)

// ExampleStorageUsage demonstrates how to use different storage providers
func ExampleStorageUsage() {
	// Example 1: Using Redis storage provider
	redisConfig := StorageConfig{
		Type:          StorageTypeRedis,
		RedisAddress:  "localhost:6379",
		RedisPassword: "",
		RedisDB:       0,
		SessionTTL:    12 * time.Hour,
		Timeout:       5 * time.Second,
	}

	// Create CsAI with Redis storage
	csRedis := New("your-api-key", nil, Options{
		StorageConfig: &redisConfig,
		SessionTTL:    12 * time.Hour,
	})

	// Example 2: Using MongoDB storage provider
	mongoConfig := StorageConfig{
		Type:            StorageTypeMongo,
		MongoURI:        "mongodb://localhost:27017",
		MongoDatabase:   "cs_ai",
		MongoCollection: "sessions",
		SessionTTL:      24 * time.Hour, // Longer TTL for MongoDB
		Timeout:         10 * time.Second,
	}

	// Create CsAI with MongoDB storage
	csMongo := New("your-api-key", nil, Options{
		StorageConfig: &mongoConfig,
		SessionTTL:    24 * time.Hour,
	})

	// Example 3: Using in-memory storage provider (for testing/development)
	memoryConfig := StorageConfig{
		Type:       StorageTypeInMemory,
		SessionTTL: 1 * time.Hour, // Short TTL for testing
		Timeout:    1 * time.Second,
	}

	// Create CsAI with in-memory storage
	csMemory := New("your-api-key", nil, Options{
		StorageConfig: &memoryConfig,
		SessionTTL:    1 * time.Hour,
	})

	// Example 4: Using custom storage provider
	customStorage := &CustomStorageProvider{}
	csCustom := New("your-api-key", nil, Options{
		StorageProvider: customStorage,
		SessionTTL:      12 * time.Hour,
	})

	// Use the instances
	_ = csRedis
	_ = csMongo
	_ = csMemory
	_ = csCustom
}

// ExampleLegacyRedisUsage shows backward compatibility
func ExampleLegacyRedisUsage() {
	// This still works for backward compatibility
	cs := New("your-api-key", nil, Options{
		Redis:      nil, // Your Redis client
		SessionTTL: 12 * time.Hour,
	})

	_ = cs
}

// ExampleStorageMigration shows how to migrate from Redis to MongoDB
func ExampleStorageMigration() {
	// Step 1: Start with Redis
	redisConfig := StorageConfig{
		Type:          StorageTypeRedis,
		RedisAddress:  "localhost:6379",
		RedisPassword: "",
		RedisDB:       0,
		SessionTTL:    12 * time.Hour,
	}

	cs := New("your-api-key", nil, Options{
		StorageConfig: &redisConfig,
	})

	// Step 2: Later, migrate to MongoDB
	mongoConfig := StorageConfig{
		Type:            StorageTypeMongo,
		MongoURI:        "mongodb://localhost:27017",
		MongoDatabase:   "cs_ai",
		MongoCollection: "sessions",
		SessionTTL:      24 * time.Hour,
	}

	cs = New("your-api-key", nil, Options{
		StorageConfig: &mongoConfig,
	})

	_ = cs
}

// ExampleStorageHealthCheck shows how to check storage health
func ExampleStorageHealthCheck() {
	cs := New("your-api-key", nil, Options{
		StorageConfig: &StorageConfig{
			Type: StorageTypeRedis,
			// ... other config
		},
	})

	// Check storage health
	if cs.options.StorageProvider != nil {
		err := cs.options.StorageProvider.HealthCheck()
		if err != nil {
			// Handle storage error
			_ = err
		}
	}
}

// ExampleStorageStats shows how to get storage statistics
func ExampleStorageStats() {
	cs := New("your-api-key", nil, Options{
		StorageConfig: &StorageConfig{
			Type: StorageTypeInMemory,
		},
	})

	// Get storage statistics (only available for in-memory storage)
	if inMemoryStorage, ok := cs.options.StorageProvider.(*InMemoryStorageProvider); ok {
		stats := inMemoryStorage.GetStorageStats()
		_ = stats
	}
}

// CustomStorageProvider is an example of implementing a custom storage provider
type CustomStorageProvider struct {
	// Your custom implementation
}

func (c *CustomStorageProvider) GetSessionMessages(ctx context.Context, sessionID string) ([]Message, error) {
	// Implement your custom logic
	return nil, nil
}

func (c *CustomStorageProvider) SaveSessionMessages(ctx context.Context, sessionID string, messages []Message, ttl time.Duration) error {
	// Implement your custom logic
	return nil
}

func (c *CustomStorageProvider) DeleteSession(ctx context.Context, sessionID string) error {
	// Implement your custom logic
	return nil
}

func (c *CustomStorageProvider) SaveLearningData(ctx context.Context, data LearningData) error {
	// Implement your custom logic
	return nil
}

func (c *CustomStorageProvider) GetLearningData(ctx context.Context, days int) ([]LearningData, error) {
	// Implement your custom logic
	return nil, nil
}

func (c *CustomStorageProvider) SaveSecurityLog(ctx context.Context, log SecurityLog) error {
	// Implement your custom logic
	return nil
}

func (c *CustomStorageProvider) GetSecurityLogs(ctx context.Context, userID string, startTime, endTime time.Time) ([]SecurityLog, error) {
	// Implement your custom logic
	return nil, nil
}

func (c *CustomStorageProvider) Close() error {
	// Implement your custom logic
	return nil
}

func (c *CustomStorageProvider) HealthCheck() error {
	// Implement your custom logic
	return nil
}
