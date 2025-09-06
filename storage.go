package cs_ai

import (
	"context"
	"time"
)

// StorageProvider defines the interface for different storage backends
type StorageProvider interface {
	// Session management
	GetSessionMessages(ctx context.Context, sessionID string) ([]Message, error)
	SaveSessionMessages(ctx context.Context, sessionID string, messages []Message, ttl time.Duration) error
	DeleteSession(ctx context.Context, sessionID string) error

	// Learning data management
	SaveLearningData(ctx context.Context, data LearningData) error
	GetLearningData(ctx context.Context, days int) ([]LearningData, error)

	// Security data management
	SaveSecurityLog(ctx context.Context, log SecurityLog) error
	GetSecurityLogs(ctx context.Context, userID string, startTime, endTime time.Time) ([]SecurityLog, error)

	// Utility methods
	Close() error
	HealthCheck() error
}

// StorageType represents different storage backends
type StorageType string

const (
	StorageTypeRedis    StorageType = "redis"
	StorageTypeMongo    StorageType = "mongo"
	StorageTypeDynamo   StorageType = "dynamo"
	StorageTypeInMemory StorageType = "memory"
)

// StorageConfig holds configuration for different storage backends
type StorageConfig struct {
	Type StorageType `json:"type"`

	// Redis configuration
	RedisAddress  string `json:"redis_address,omitempty"`
	RedisPassword string `json:"redis_password,omitempty"`
	RedisDB       int    `json:"redis_db,omitempty"`

	// MongoDB configuration
	MongoURI        string `json:"mongo_uri,omitempty"`
	MongoDatabase   string `json:"mongo_database,omitempty"`
	MongoCollection string `json:"mongo_collection,omitempty"`

	// DynamoDB configuration
	AWSRegion   string `json:"aws_region,omitempty"`
	DynamoTable string `json:"dynamo_table,omitempty"`

	// Common configuration
	SessionTTL time.Duration `json:"session_ttl,omitempty"`
	MaxRetries int           `json:"max_retries,omitempty"`
	Timeout    time.Duration `json:"timeout,omitempty"`
}

// NewStorageProvider creates a new storage provider based on configuration
func NewStorageProvider(config StorageConfig) (StorageProvider, error) {
	switch config.Type {
	case StorageTypeRedis:
		return NewRedisStorageProvider(config)
	case StorageTypeMongo:
		return NewMongoStorageProvider(config)
	case StorageTypeDynamo:
		return NewDynamoStorageProvider(config)
	case StorageTypeInMemory:
		return NewInMemoryStorageProvider(config)
	default:
		return NewRedisStorageProvider(config) // Default fallback
	}
}
