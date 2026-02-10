package cs_ai

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoStorageProvider implements StorageProvider for MongoDB
type MongoStorageProvider struct {
	client     *mongo.Client
	database   *mongo.Database
	collection *mongo.Collection
	config     StorageConfig
}

// NewMongoStorageProvider creates a new MongoDB storage provider
func NewMongoStorageProvider(config StorageConfig) (StorageProvider, error) {
	if config.MongoURI == "" {
		return nil, fmt.Errorf("MongoDB URI is required")
	}

	// Set default values
	if config.MongoDatabase == "" {
		config.MongoDatabase = "cs_ai"
	}
	if config.MongoCollection == "" {
		config.MongoCollection = "sessions"
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	// Create MongoDB client
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoURI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Test connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	database := client.Database(config.MongoDatabase)
	collection := database.Collection(config.MongoCollection)

	// Create indexes for better performance
	indexModels := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "session_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "expires_at", Value: 1},
			},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "timestamp", Value: -1},
			},
		},
	}

	_, err = collection.Indexes().CreateMany(ctx, indexModels)
	if err != nil {
		// Log warning but don't fail - indexes might already exist
		fmt.Printf("Warning: Failed to create MongoDB indexes: %v\n", err)
	}

	return &MongoStorageProvider{
		client:     client,
		database:   database,
		collection: collection,
		config:     config,
	}, nil
}

// GetSessionMessages retrieves session messages from MongoDB
func (m *MongoStorageProvider) GetSessionMessages(ctx context.Context, sessionID string) ([]Message, error) {
	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	var sessionDoc struct {
		SessionID string    `bson:"session_id"`
		Messages  []Message `bson:"messages"`
		ExpiresAt time.Time `bson:"expires_at"`
	}

	findOpts := options.FindOne().SetSort(bson.D{{Key: "updated_at", Value: -1}})
	err := m.collection.FindOne(ctx, bson.M{"session_id": sessionID}, findOpts).Decode(&sessionDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Session not found
		}
		return nil, fmt.Errorf("failed to get session messages: %w", err)
	}

	// Check if session has expired
	if time.Now().After(sessionDoc.ExpiresAt) {
		// Delete all matching sessions to avoid stale duplicates.
		_, _ = m.collection.DeleteMany(ctx, bson.M{"session_id": sessionID})
		return nil, nil
	}

	return sessionDoc.Messages, nil
}

// SaveSessionMessages saves session messages to MongoDB with TTL
func (m *MongoStorageProvider) SaveSessionMessages(ctx context.Context, sessionID string, messages []Message, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	// Prepare messages for storage (populate ContentMap for JSON content)
	for i := range messages {
		messages[i].PrepareForStorage()
	}

	expiresAt := time.Now().Add(ttl)

	sessionDoc := bson.M{
		"session_id": sessionID,
		"messages":   messages,
		"expires_at": expiresAt,
		"updated_at": time.Now(),
	}

	// Use upsert to create or update session
	opts := options.Update().SetUpsert(true)
	filter := bson.M{"session_id": sessionID}
	update := bson.M{"$set": sessionDoc}

	_, err := m.collection.UpdateMany(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save session messages: %w", err)
	}

	return nil
}

// DeleteSession deletes a session from MongoDB
func (m *MongoStorageProvider) DeleteSession(ctx context.Context, sessionID string) error {
	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	_, err := m.collection.DeleteMany(ctx, bson.M{"session_id": sessionID})
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// GetSystemMessages retrieves system messages from MongoDB
func (m *MongoStorageProvider) GetSystemMessages(ctx context.Context, sessionID string) ([]Message, error) {
	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	var sessionDoc struct {
		SessionID      string    `bson:"session_id"`
		SystemMessages []Message `bson:"system_messages"`
		ExpiresAt      time.Time `bson:"expires_at"`
	}

	findOpts := options.FindOne().SetSort(bson.D{{Key: "updated_at", Value: -1}})
	err := m.collection.FindOne(ctx, bson.M{"session_id": sessionID}, findOpts).Decode(&sessionDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Session not found
		}
		return nil, fmt.Errorf("failed to get system messages: %w", err)
	}

	// Check if session has expired
	if time.Now().After(sessionDoc.ExpiresAt) {
		_, _ = m.collection.DeleteMany(ctx, bson.M{"session_id": sessionID})
		return nil, nil
	}

	return sessionDoc.SystemMessages, nil
}

// SaveSystemMessages saves system messages to MongoDB with TTL
func (m *MongoStorageProvider) SaveSystemMessages(ctx context.Context, sessionID string, messages []Message, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	expiresAt := time.Now().Add(ttl)

	// Use upsert to create or update session with system messages
	opts := options.Update().SetUpsert(true)
	filter := bson.M{"session_id": sessionID}
	update := bson.M{
		"$set": bson.M{
			"session_id":      sessionID,
			"system_messages": messages,
			"expires_at":      expiresAt,
			"updated_at":      time.Now(),
		},
	}

	_, err := m.collection.UpdateMany(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save system messages: %w", err)
	}

	return nil
}

// SaveLearningData saves learning data to MongoDB
func (m *MongoStorageProvider) SaveLearningData(ctx context.Context, data LearningData) error {
	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	learningCollection := m.database.Collection("learning_data")

	// Convert LearningData to BSON document
	learningDoc := bson.M{
		"query":      data.Query,
		"response":   data.Response,
		"tools":      data.Tools,
		"context":    data.Context,
		"timestamp":  data.Timestamp,
		"feedback":   data.Feedback,
		"created_at": time.Now(),
	}

	_, err := learningCollection.InsertOne(ctx, learningDoc)
	if err != nil {
		return fmt.Errorf("failed to save learning data: %w", err)
	}

	return nil
}

// GetLearningData retrieves learning data from MongoDB for the specified number of days
func (m *MongoStorageProvider) GetLearningData(ctx context.Context, days int) ([]LearningData, error) {
	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	learningCollection := m.database.Collection("learning_data")

	// Calculate start date
	startDate := time.Now().AddDate(0, 0, -days)

	filter := bson.M{
		"timestamp": bson.M{
			"$gte": startDate,
		},
	}

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})

	cursor, err := learningCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get learning data: %w", err)
	}
	defer cursor.Close(ctx)

	var learningData []LearningData
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue // Skip invalid documents
		}

		// Convert BSON document to LearningData
		data := LearningData{
			Query:    getString(doc, "query"),
			Response: getString(doc, "response"),
			Tools:    getStringSlice(doc, "tools"),
			Context:  getMap(doc, "context"),
			Feedback: getInt(doc, "feedback"),
		}

		// Handle timestamp conversion
		if timestamp, ok := doc["timestamp"].(primitive.DateTime); ok {
			data.Timestamp = time.Unix(int64(timestamp)/1000, 0)
		}

		learningData = append(learningData, data)
	}

	return learningData, nil
}

// SaveSecurityLog saves security log to MongoDB
func (m *MongoStorageProvider) SaveSecurityLog(ctx context.Context, log SecurityLog) error {
	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	securityCollection := m.database.Collection("security_logs")

	// Convert SecurityLog to BSON document
	securityDoc := bson.M{
		"session_id":   log.SessionID,
		"user_id":      log.UserID,
		"message_hash": log.MessageHash,
		"timestamp":    log.Timestamp,
		"spam_score":   log.SpamScore,
		"allowed":      log.Allowed,
		"error":        log.Error,
		"created_at":   time.Now(),
	}

	_, err := securityCollection.InsertOne(ctx, securityDoc)
	if err != nil {
		return fmt.Errorf("failed to save security log: %w", err)
	}

	return nil
}

// GetSecurityLogs retrieves security logs from MongoDB for a specific user and time range
func (m *MongoStorageProvider) GetSecurityLogs(ctx context.Context, userID string, startTime, endTime time.Time) ([]SecurityLog, error) {
	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	securityCollection := m.database.Collection("security_logs")

	filter := bson.M{
		"user_id": userID,
		"timestamp": bson.M{
			"$gte": startTime,
			"$lte": endTime,
		},
	}

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})

	cursor, err := securityCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get security logs: %w", err)
	}
	defer cursor.Close(ctx)

	var securityLogs []SecurityLog
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue // Skip invalid documents
		}

		// Convert BSON document to SecurityLog
		log := SecurityLog{
			SessionID:   getString(doc, "session_id"),
			UserID:      getString(doc, "user_id"),
			MessageHash: getString(doc, "message_hash"),
			SpamScore:   getFloat64(doc, "spam_score"),
			Allowed:     getBool(doc, "allowed"),
			Error:       getString(doc, "error"),
		}

		// Handle timestamp conversion
		if timestamp, ok := doc["timestamp"].(primitive.DateTime); ok {
			log.Timestamp = time.Unix(int64(timestamp)/1000, 0)
		}

		securityLogs = append(securityLogs, log)
	}

	return securityLogs, nil
}

// Close closes the MongoDB connection
func (m *MongoStorageProvider) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
	defer cancel()

	return m.client.Disconnect(ctx)
}

// HealthCheck checks if MongoDB is accessible
func (m *MongoStorageProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
	defer cancel()

	return m.client.Ping(ctx, nil)
}

// Helper function to safely extract string values from BSON documents
func getString(doc bson.M, key string) string {
	if value, ok := doc[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// Helper function to safely extract string slice values from BSON documents
func getStringSlice(doc bson.M, key string) []string {
	if value, ok := doc[key]; ok {
		if slice, ok := value.([]interface{}); ok {
			var result []string
			for _, item := range slice {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return nil
}

// Helper function to safely extract map values from BSON documents
func getMap(doc bson.M, key string) map[string]interface{} {
	if value, ok := doc[key]; ok {
		if m, ok := value.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// Helper function to safely extract int values from BSON documents
func getInt(doc bson.M, key string) int {
	if value, ok := doc[key]; ok {
		if i, ok := value.(int); ok {
			return i
		}
		if i, ok := value.(int32); ok {
			return int(i)
		}
		if i, ok := value.(int64); ok {
			return int(i)
		}
	}
	return 0
}

// Helper function to safely extract float64 values from BSON documents
func getFloat64(doc bson.M, key string) float64 {
	if value, ok := doc[key]; ok {
		if f, ok := value.(float64); ok {
			return f
		}
		if f, ok := value.(float32); ok {
			return float64(f)
		}
		if i, ok := value.(int); ok {
			return float64(i)
		}
		if i, ok := value.(int64); ok {
			return float64(i)
		}
	}
	return 0.0
}

// Helper function to safely extract bool values from BSON documents
func getBool(doc bson.M, key string) bool {
	if value, ok := doc[key]; ok {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return false
}

// GetStorageStats returns basic statistics about the MongoDB storage
func (m *MongoStorageProvider) GetStorageStats() map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
	defer cancel()

	stats := make(map[string]interface{})

	// Count sessions
	sessionCount, err := m.collection.CountDocuments(ctx, bson.M{})
	if err == nil {
		stats["total_sessions"] = sessionCount
	}

	// Count learning data
	learningCollection := m.database.Collection("learning_data")
	learningCount, err := learningCollection.CountDocuments(ctx, bson.M{})
	if err == nil {
		stats["total_learning_data"] = learningCount
	}

	// Count security logs
	securityCollection := m.database.Collection("security_logs")
	securityCount, err := securityCollection.CountDocuments(ctx, bson.M{})
	if err == nil {
		stats["total_security_logs"] = securityCount
	}

	// Get database stats
	databaseStats := m.database.RunCommand(ctx, bson.M{"dbStats": 1})
	if databaseStats.Err() == nil {
		var result bson.M
		if err := databaseStats.Decode(&result); err == nil {
			if dataSize, ok := result["dataSize"].(int64); ok {
				stats["data_size_mb"] = float64(dataSize) / (1024 * 1024)
			}
			if storageSize, ok := result["storageSize"].(int64); ok {
				stats["storage_size_mb"] = float64(storageSize) / (1024 * 1024)
			}
		}
	}

	return stats
}
