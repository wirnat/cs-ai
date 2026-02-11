package cs_ai

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// InMemoryStorageProvider implements StorageProvider using in-memory storage
type InMemoryStorageProvider struct {
	mu           sync.RWMutex
	sessions     map[string]*MemorySession
	learningData map[string][]LearningData
	securityLogs map[string][]SecurityLog
	config       StorageConfig
}

// MemorySession represents a session in memory
type MemorySession struct {
	SessionID      string    `json:"session_id"`
	Messages       []Message `json:"messages"`
	SystemMessages []Message `json:"system_messages"` // Pre-chat/default messages
	TTL            time.Time `json:"ttl"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// NewInMemoryStorageProvider creates a new in-memory storage provider
func NewInMemoryStorageProvider(config StorageConfig) (StorageProvider, error) {
	provider := &InMemoryStorageProvider{
		sessions:     make(map[string]*MemorySession),
		learningData: make(map[string][]LearningData),
		securityLogs: make(map[string][]SecurityLog),
		config:       config,
	}

	// Start cleanup goroutine for expired sessions
	go provider.cleanupExpiredSessions()

	return provider, nil
}

func (m *InMemoryStorageProvider) GetSessionMessages(ctx context.Context, sessionID string) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, nil // Session not found
	}

	// Check if session is expired
	if time.Now().After(session.TTL) {
		delete(m.sessions, sessionID)
		return nil, nil
	}

	return session.Messages, nil
}

func (m *InMemoryStorageProvider) SaveSessionMessages(ctx context.Context, sessionID string, messages []Message, ttl time.Duration) error {
	if ttl == 0 {
		ttl = m.config.SessionTTL
	}
	if ttl == 0 {
		ttl = 12 * time.Hour // Default fallback
	}

	EnsureAutoIncrementMessageIDs(messages)

	// Prepare messages for storage (populate ContentMap for JSON content)
	for i := range messages {
		messages[i].PrepareForStorage()
	}

	now := time.Now()
	session := &MemorySession{
		SessionID: sessionID,
		Messages:  messages,
		TTL:       now.Add(ttl),
		UpdatedAt: now,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, exists := m.sessions[sessionID]; exists {
		session.CreatedAt = existing.CreatedAt
	} else {
		session.CreatedAt = now
	}

	m.sessions[sessionID] = session
	return nil
}

func (m *InMemoryStorageProvider) DeleteSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	return nil
}

func (m *InMemoryStorageProvider) GetSystemMessages(ctx context.Context, sessionID string) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, nil // Session not found
	}

	// Check if session is expired
	if time.Now().After(session.TTL) {
		delete(m.sessions, sessionID)
		return nil, nil
	}

	return session.SystemMessages, nil
}

func (m *InMemoryStorageProvider) SaveSystemMessages(ctx context.Context, sessionID string, messages []Message, ttl time.Duration) error {
	if ttl == 0 {
		ttl = m.config.SessionTTL
	}
	if ttl == 0 {
		ttl = 12 * time.Hour // Default fallback
	}

	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, exists := m.sessions[sessionID]; exists {
		// Update existing session with system messages
		existing.SystemMessages = messages
		existing.UpdatedAt = now
		if now.Add(ttl).After(existing.TTL) {
			existing.TTL = now.Add(ttl)
		}
	} else {
		// Create new session with only system messages
		m.sessions[sessionID] = &MemorySession{
			SessionID:      sessionID,
			Messages:       []Message{},
			SystemMessages: messages,
			TTL:            now.Add(ttl),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
	}

	return nil
}

func (m *InMemoryStorageProvider) SaveLearningData(ctx context.Context, data LearningData) error {
	dateKey := data.Timestamp.Format("2006-01-02")

	m.mu.Lock()
	defer m.mu.Unlock()

	m.learningData[dateKey] = append(m.learningData[dateKey], data)
	return nil
}

func (m *InMemoryStorageProvider) GetLearningData(ctx context.Context, days int) ([]LearningData, error) {
	var allData []LearningData

	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i)
		dateKey := date.Format("2006-01-02")

		if data, exists := m.learningData[dateKey]; exists {
			allData = append(allData, data...)
		}
	}

	return allData, nil
}

func (m *InMemoryStorageProvider) SaveSecurityLog(ctx context.Context, log SecurityLog) error {
	dateKey := log.Timestamp.Format("2006-01-02")
	userKey := fmt.Sprintf("%s:%s", log.UserID, dateKey)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.securityLogs[userKey] = append(m.securityLogs[userKey], log)
	return nil
}

func (m *InMemoryStorageProvider) GetSecurityLogs(ctx context.Context, userID string, startTime, endTime time.Time) ([]SecurityLog, error) {
	var logs []SecurityLog

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get logs for each day in the range
	for d := startTime; d.Before(endTime) || d.Equal(endTime); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")
		userKey := fmt.Sprintf("%s:%s", userID, dateKey)

		if userLogs, exists := m.securityLogs[userKey]; exists {
			for _, log := range userLogs {
				if log.Timestamp.After(startTime) && log.Timestamp.Before(endTime) {
					logs = append(logs, log)
				}
			}
		}
	}

	return logs, nil
}

func (m *InMemoryStorageProvider) Close() error {
	// Nothing to close for in-memory storage
	return nil
}

func (m *InMemoryStorageProvider) HealthCheck() error {
	// In-memory storage is always healthy
	return nil
}

// cleanupExpiredSessions periodically removes expired sessions
func (m *InMemoryStorageProvider) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		m.mu.Lock()
		for sessionID, session := range m.sessions {
			if now.After(session.TTL) {
				delete(m.sessions, sessionID)
			}
		}
		m.mu.Unlock()
	}
}

// GetStorageStats returns statistics about the in-memory storage
func (m *InMemoryStorageProvider) GetStorageStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Count total sessions
	totalSessions := len(m.sessions)

	// Count total learning data entries
	totalLearningData := 0
	for _, data := range m.learningData {
		totalLearningData += len(data)
	}

	// Count total security logs
	totalSecurityLogs := 0
	for _, logs := range m.securityLogs {
		totalSecurityLogs += len(logs)
	}

	return map[string]interface{}{
		"storage_type":        "in_memory",
		"total_sessions":      totalSessions,
		"total_learning_data": totalLearningData,
		"total_security_logs": totalSecurityLogs,
		"memory_usage_mb":     m.estimateMemoryUsage(),
	}
}

// estimateMemoryUsage estimates memory usage in MB
func (m *InMemoryStorageProvider) estimateMemoryUsage() float64 {
	// Rough estimation: each session ~1KB, learning data ~500B, security log ~200B
	sessionSize := len(m.sessions) * 1024
	learningSize := 0
	for _, data := range m.learningData {
		learningSize += len(data) * 512
	}
	securitySize := 0
	for _, logs := range m.securityLogs {
		securitySize += len(logs) * 256
	}

	totalBytes := sessionSize + learningSize + securitySize
	return float64(totalBytes) / (1024 * 1024) // Convert to MB
}
