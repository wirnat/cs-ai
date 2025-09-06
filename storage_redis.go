package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisStorageProvider implements StorageProvider using Redis
type RedisStorageProvider struct {
	client *redis.Client
	config StorageConfig
}

// NewRedisStorageProvider creates a new Redis storage provider
func NewRedisStorageProvider(config StorageConfig) (StorageProvider, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddress,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return &RedisStorageProvider{
		client: client,
		config: config,
	}, nil
}

func (r *RedisStorageProvider) GetSessionMessages(ctx context.Context, sessionID string) ([]Message, error) {
	key := fmt.Sprintf("ai:session:%s", sessionID)
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Session not found
		}
		return nil, fmt.Errorf("failed to get session from Redis: %v", err)
	}

	var messages []Message
	if err := json.Unmarshal([]byte(data), &messages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session messages: %v", err)
	}

	return messages, nil
}

func (r *RedisStorageProvider) SaveSessionMessages(ctx context.Context, sessionID string, messages []Message, ttl time.Duration) error {
	key := fmt.Sprintf("ai:session:%s", sessionID)
	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("failed to marshal session messages: %v", err)
	}

	// Use provided TTL or default from config
	if ttl == 0 {
		ttl = r.config.SessionTTL
	}
	if ttl == 0 {
		ttl = 12 * time.Hour // Default fallback
	}

	return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *RedisStorageProvider) DeleteSession(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("ai:session:%s", sessionID)
	return r.client.Del(ctx, key).Err()
}

func (r *RedisStorageProvider) SaveLearningData(ctx context.Context, data LearningData) error {
	key := fmt.Sprintf("ai:learning:%s", time.Now().Format("2006-01-02"))
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal learning data: %v", err)
	}

	return r.client.RPush(ctx, key, dataJSON).Err()
}

func (r *RedisStorageProvider) GetLearningData(ctx context.Context, days int) ([]LearningData, error) {
	var allData []LearningData

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i)
		key := fmt.Sprintf("ai:learning:%s", date.Format("2006-01-02"))

		data, err := r.client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			continue // Skip failed dates
		}

		for _, item := range data {
			var learningData LearningData
			if err := json.Unmarshal([]byte(item), &learningData); err != nil {
				continue // Skip invalid items
			}
			allData = append(allData, learningData)
		}
	}

	return allData, nil
}

func (r *RedisStorageProvider) SaveSecurityLog(ctx context.Context, log SecurityLog) error {
	key := fmt.Sprintf("ai:security:%s:%s", log.UserID, time.Now().Format("2006-01-02"))
	dataJSON, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal security log: %v", err)
	}

	return r.client.RPush(ctx, key, dataJSON).Err()
}

func (r *RedisStorageProvider) GetSecurityLogs(ctx context.Context, userID string, startTime, endTime time.Time) ([]SecurityLog, error) {
	var logs []SecurityLog

	// Get logs for each day in the range
	for d := startTime; d.Before(endTime) || d.Equal(endTime); d = d.AddDate(0, 0, 1) {
		key := fmt.Sprintf("ai:security:%s:%s", userID, d.Format("2006-01-02"))

		data, err := r.client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			continue
		}

		for _, item := range data {
			var log SecurityLog
			if err := json.Unmarshal([]byte(item), &log); err != nil {
				continue
			}

			// Filter by time range
			if log.Timestamp.After(startTime) && log.Timestamp.Before(endTime) {
				logs = append(logs, log)
			}
		}
	}

	return logs, nil
}

func (r *RedisStorageProvider) Close() error {
	return r.client.Close()
}

func (r *RedisStorageProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return r.client.Ping(ctx).Err()
}
