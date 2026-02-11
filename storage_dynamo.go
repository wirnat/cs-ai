package cs_ai

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoStorageProvider implements StorageProvider for DynamoDB
type DynamoStorageProvider struct {
	client *dynamodb.Client
	config StorageConfig
}

// DynamoDBSession represents a session document in DynamoDB
type DynamoDBSession struct {
	SessionID      string    `dynamodbav:"session_id"`
	Messages       []Message `dynamodbav:"messages"`
	SystemMessages []Message `dynamodbav:"system_messages"` // Pre-chat/default messages
	ExpiresAt      int64     `dynamodbav:"expires_at"`
	UpdatedAt      int64     `dynamodbav:"updated_at"`
}

// DynamoDBLearningData represents learning data document in DynamoDB
type DynamoDBLearningData struct {
	ID        string                 `dynamodbav:"id"`
	Query     string                 `dynamodbav:"query"`
	Response  string                 `dynamodbav:"response"`
	Tools     []string               `dynamodbav:"tools"`
	Context   map[string]interface{} `dynamodbav:"context"`
	Timestamp int64                  `dynamodbav:"timestamp"`
	Feedback  int                    `dynamodbav:"feedback"`
	CreatedAt int64                  `dynamodbav:"created_at"`
}

// DynamoDBSecurityLog represents security log document in DynamoDB
type DynamoDBSecurityLog struct {
	ID          string  `dynamodbav:"id"`
	SessionID   string  `dynamodbav:"session_id"`
	UserID      string  `dynamodbav:"user_id"`
	MessageHash string  `dynamodbav:"message_hash"`
	Timestamp   int64   `dynamodbav:"timestamp"`
	SpamScore   float64 `dynamodbav:"spam_score"`
	Allowed     bool    `dynamodbav:"allowed"`
	Error       string  `dynamodbav:"error"`
	CreatedAt   int64   `dynamodbav:"created_at"`
}

// NewDynamoStorageProvider creates a new DynamoDB storage provider
func NewDynamoStorageProvider(config StorageConfig) (*DynamoStorageProvider, error) {
	if config.AWSRegion == "" {
		config.AWSRegion = "us-east-1"
	}
	if config.DynamoTable == "" {
		config.DynamoTable = "cs_ai_sessions"
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	// Load AWS configuration
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(config.AWSRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	// Test connection by describing the table
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	_, err = client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(config.DynamoTable),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to DynamoDB table %s: %w", config.DynamoTable, err)
	}

	return &DynamoStorageProvider{
		client: client,
		config: config,
	}, nil
}

// GetSessionMessages retrieves session messages from DynamoDB
func (d *DynamoStorageProvider) GetSessionMessages(ctx context.Context, sessionID string) ([]Message, error) {
	ctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	result, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.config.DynamoTable),
		Key: map[string]types.AttributeValue{
			"session_id": &types.AttributeValueMemberS{Value: sessionID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get session from DynamoDB: %w", err)
	}

	if result.Item == nil {
		return nil, nil // Session not found
	}

	var session DynamoDBSession
	err = attributevalue.UnmarshalMap(result.Item, &session)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Check if session has expired
	if time.Now().Unix() > session.ExpiresAt {
		// Delete expired session
		_ = d.DeleteSession(ctx, sessionID)
		return nil, nil
	}

	return session.Messages, nil
}

// SaveSessionMessages saves session messages to DynamoDB with TTL
func (d *DynamoStorageProvider) SaveSessionMessages(ctx context.Context, sessionID string, messages []Message, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	EnsureAutoIncrementMessageIDs(messages)

	// Prepare messages for storage (populate ContentMap for JSON content)
	for i := range messages {
		messages[i].PrepareForStorage()
	}

	expiresAt := time.Now().Add(ttl).Unix()
	updatedAt := time.Now().Unix()

	session := DynamoDBSession{
		SessionID: sessionID,
		Messages:  messages,
		ExpiresAt: expiresAt,
		UpdatedAt: updatedAt,
	}

	item, err := attributevalue.MarshalMap(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.config.DynamoTable),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to save session to DynamoDB: %w", err)
	}

	return nil
}

// DeleteSession deletes a session from DynamoDB
func (d *DynamoStorageProvider) DeleteSession(ctx context.Context, sessionID string) error {
	ctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	_, err := d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(d.config.DynamoTable),
		Key: map[string]types.AttributeValue{
			"session_id": &types.AttributeValueMemberS{Value: sessionID},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete session from DynamoDB: %w", err)
	}

	return nil
}

// GetSystemMessages retrieves system messages from DynamoDB
func (d *DynamoStorageProvider) GetSystemMessages(ctx context.Context, sessionID string) ([]Message, error) {
	ctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	result, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.config.DynamoTable),
		Key: map[string]types.AttributeValue{
			"session_id": &types.AttributeValueMemberS{Value: sessionID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get session from DynamoDB: %w", err)
	}

	if result.Item == nil {
		return nil, nil // Session not found
	}

	var session DynamoDBSession
	err = attributevalue.UnmarshalMap(result.Item, &session)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Check if session has expired
	if time.Now().Unix() > session.ExpiresAt {
		return nil, nil
	}

	return session.SystemMessages, nil
}

// SaveSystemMessages saves system messages to DynamoDB with TTL
func (d *DynamoStorageProvider) SaveSystemMessages(ctx context.Context, sessionID string, messages []Message, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	expiresAt := time.Now().Add(ttl).Unix()
	updatedAt := time.Now().Unix()

	// Get existing session to preserve other fields
	existing, _ := d.GetSessionMessages(ctx, sessionID)

	session := DynamoDBSession{
		SessionID:      sessionID,
		Messages:       existing,
		SystemMessages: messages,
		ExpiresAt:      expiresAt,
		UpdatedAt:      updatedAt,
	}

	item, err := attributevalue.MarshalMap(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.config.DynamoTable),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to save system messages to DynamoDB: %w", err)
	}

	return nil
}

// SaveLearningData saves learning data to DynamoDB
func (d *DynamoStorageProvider) SaveLearningData(ctx context.Context, data LearningData) error {
	ctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	learningTable := d.config.DynamoTable + "_learning"

	learningDoc := DynamoDBLearningData{
		ID:        fmt.Sprintf("learning_%d", time.Now().UnixNano()),
		Query:     data.Query,
		Response:  data.Response,
		Tools:     data.Tools,
		Context:   data.Context,
		Timestamp: data.Timestamp.Unix(),
		Feedback:  data.Feedback,
		CreatedAt: time.Now().Unix(),
	}

	item, err := attributevalue.MarshalMap(learningDoc)
	if err != nil {
		return fmt.Errorf("failed to marshal learning data: %w", err)
	}

	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(learningTable),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to save learning data to DynamoDB: %w", err)
	}

	return nil
}

// GetLearningData retrieves learning data from DynamoDB for the specified number of days
func (d *DynamoStorageProvider) GetLearningData(ctx context.Context, days int) ([]LearningData, error) {
	ctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	learningTable := d.config.DynamoTable + "_learning"

	// Calculate start timestamp
	startTime := time.Now().AddDate(0, 0, -days).Unix()

	// Scan the learning table for recent data
	result, err := d.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(learningTable),
		FilterExpression: aws.String("#timestamp >= :start_time"),
		ExpressionAttributeNames: map[string]string{
			"#timestamp": "timestamp",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":start_time": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", startTime)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan learning data from DynamoDB: %w", err)
	}

	var learningData []LearningData
	for _, item := range result.Items {
		var doc DynamoDBLearningData
		err := attributevalue.UnmarshalMap(item, &doc)
		if err != nil {
			continue // Skip invalid documents
		}

		data := LearningData{
			Query:     doc.Query,
			Response:  doc.Response,
			Tools:     doc.Tools,
			Context:   doc.Context,
			Timestamp: time.Unix(doc.Timestamp, 0),
			Feedback:  doc.Feedback,
		}

		learningData = append(learningData, data)
	}

	return learningData, nil
}

// SaveSecurityLog saves security log to DynamoDB
func (d *DynamoStorageProvider) SaveSecurityLog(ctx context.Context, log SecurityLog) error {
	ctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	securityTable := d.config.DynamoTable + "_security"

	securityDoc := DynamoDBSecurityLog{
		ID:          fmt.Sprintf("security_%d", time.Now().UnixNano()),
		SessionID:   log.SessionID,
		UserID:      log.UserID,
		MessageHash: log.MessageHash,
		Timestamp:   log.Timestamp.Unix(),
		SpamScore:   log.SpamScore,
		Allowed:     log.Allowed,
		Error:       log.Error,
		CreatedAt:   time.Now().Unix(),
	}

	item, err := attributevalue.MarshalMap(securityDoc)
	if err != nil {
		return fmt.Errorf("failed to marshal security log: %w", err)
	}

	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(securityTable),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to save security log to DynamoDB: %w", err)
	}

	return nil
}

// GetSecurityLogs retrieves security logs from DynamoDB for a specific user and time range
func (d *DynamoStorageProvider) GetSecurityLogs(ctx context.Context, userID string, startTime, endTime time.Time) ([]SecurityLog, error) {
	ctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	securityTable := d.config.DynamoTable + "_security"

	// Scan the security table for user logs in time range
	result, err := d.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(securityTable),
		FilterExpression: aws.String("#user_id = :user_id AND #timestamp BETWEEN :start_time AND :end_time"),
		ExpressionAttributeNames: map[string]string{
			"#user_id":   "user_id",
			"#timestamp": "timestamp",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":user_id":    &types.AttributeValueMemberS{Value: userID},
			":start_time": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", startTime.Unix())},
			":end_time":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", endTime.Unix())},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan security logs from DynamoDB: %w", err)
	}

	var securityLogs []SecurityLog
	for _, item := range result.Items {
		var doc DynamoDBSecurityLog
		err := attributevalue.UnmarshalMap(item, &doc)
		if err != nil {
			continue // Skip invalid documents
		}

		log := SecurityLog{
			SessionID:   doc.SessionID,
			UserID:      doc.UserID,
			MessageHash: doc.MessageHash,
			Timestamp:   time.Unix(doc.Timestamp, 0),
			SpamScore:   doc.SpamScore,
			Allowed:     doc.Allowed,
			Error:       doc.Error,
		}

		securityLogs = append(securityLogs, log)
	}

	return securityLogs, nil
}

// Close closes the DynamoDB connection
func (d *DynamoStorageProvider) Close() error {
	// DynamoDB client doesn't need explicit closing
	return nil
}

// HealthCheck checks if DynamoDB is accessible
func (d *DynamoStorageProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), d.config.Timeout)
	defer cancel()

	_, err := d.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(d.config.DynamoTable),
	})
	return err
}

// GetStorageStats returns basic statistics about the DynamoDB storage
func (d *DynamoStorageProvider) GetStorageStats() map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), d.config.Timeout)
	defer cancel()

	stats := make(map[string]interface{})

	// Get table description for main sessions table
	tableDesc, err := d.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(d.config.DynamoTable),
	})
	if err == nil {
		stats["table_name"] = d.config.DynamoTable
		stats["table_status"] = tableDesc.Table.TableStatus
		if tableDesc.Table.ItemCount != nil {
			stats["item_count"] = *tableDesc.Table.ItemCount
		}
		if tableDesc.Table.TableSizeBytes != nil {
			stats["table_size_mb"] = float64(*tableDesc.Table.TableSizeBytes) / (1024 * 1024)
		}
	}

	// Try to get learning table stats
	learningTable := d.config.DynamoTable + "_learning"
	learningDesc, err := d.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(learningTable),
	})
	if err == nil {
		stats["learning_table_status"] = learningDesc.Table.TableStatus
		if learningDesc.Table.ItemCount != nil {
			stats["learning_item_count"] = *learningDesc.Table.ItemCount
		}
	}

	// Try to get security table stats
	securityTable := d.config.DynamoTable + "_security"
	securityDesc, err := d.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(securityTable),
	})
	if err == nil {
		stats["security_table_status"] = securityDesc.Table.TableStatus
		if securityDesc.Table.ItemCount != nil {
			stats["security_item_count"] = *securityDesc.Table.ItemCount
		}
	}

	return stats
}
