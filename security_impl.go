package cs_ai

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SecurityManager manages security features
type SecurityManager struct {
	options   *SecurityOptions
	rateLimit *RateLimiter
	spam      *SpamDetector
	mu        sync.RWMutex
	requests  map[string][]SecurityLog
}

// SecurityLog stores security event information
type SecurityLog struct {
	SessionID   string
	UserID      string
	MessageHash string
	Timestamp   time.Time
	SpamScore   float64
	Allowed     bool
	Error       string
}

// UserAnalytics holds analytics data for a user
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

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	maxPerMinute int
	maxPerHour   int
	maxPerDay    int

	mu       sync.RWMutex
	requests map[string][]time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxPerMinute, maxPerHour, maxPerDay int) *RateLimiter {
	return &RateLimiter{
		maxPerMinute: maxPerMinute,
		maxPerHour:   maxPerHour,
		maxPerDay:    maxPerDay,
		requests:     make(map[string][]time.Time),
	}
}

// Check checks if user has exceeded rate limits
func (rl *RateLimiter) Check(userID string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Clean old requests
	cutoffDay := now.Add(-24 * time.Hour)
	cutoffHour := now.Add(-1 * time.Hour)
	cutoffMinute := now.Add(-1 * time.Minute)

	requests := rl.requests[userID]
	validRequests := make([]time.Time, 0)

	for _, req := range requests {
		if req.After(cutoffDay) {
			validRequests = append(validRequests, req)
		}
	}

	rl.requests[userID] = validRequests

	// Check daily limit
	if len(validRequests) >= rl.maxPerDay {
		return fmt.Errorf("daily limit exceeded (%d requests)", rl.maxPerDay)
	}

	// Check hourly limit
	hourlyCount := 0
	for _, req := range validRequests {
		if req.After(cutoffHour) {
			hourlyCount++
		}
	}
	if hourlyCount >= rl.maxPerHour {
		return fmt.Errorf("hourly limit exceeded (%d requests)", rl.maxPerHour)
	}

	// Check minute limit
	minuteCount := 0
	for _, req := range validRequests {
		if req.After(cutoffMinute) {
			minuteCount++
		}
	}
	if minuteCount >= rl.maxPerMinute {
		return fmt.Errorf("per-minute limit exceeded (%d requests)", rl.maxPerMinute)
	}

	// Add current request
	rl.requests[userID] = append(rl.requests[userID], now)

	return nil
}

// Reset resets rate limits for a specific user
func (rl *RateLimiter) Reset(userID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.requests, userID)
}

// SpamDetector provides spam detection functionality
type SpamDetector struct {
	threshold float64
}

// NewSpamDetector creates a new spam detector
func NewSpamDetector(threshold float64) *SpamDetector {
	return &SpamDetector{threshold: threshold}
}

// Detect checks if message is spam
func (sd *SpamDetector) Detect(message string) float64 {
	score := 0.0

	// Check for excessive punctuation
	if strings.Count(message, "!") > 5 {
		score += 0.3
	}

	// Check for all caps
	upperCount := 0
	for _, char := range message {
		if char >= 'A' && char <= 'Z' {
			upperCount++
		}
	}
	if len(message) > 0 && float64(upperCount)/float64(len(message)) > 0.8 {
		score += 0.4
	}

	// Check for excessive length
	if len(message) > 1000 {
		score += 0.2
	}

	// Check for repetitive characters
	if len(message) > 10 {
		repeated := 0
		for i := 2; i < len(message); i++ {
			if message[i] == message[i-1] && message[i] == message[i-2] {
				repeated++
			}
		}
		if float64(repeated)/float64(len(message)) > 0.1 {
			score += 0.3
		}
	}

	// Check for common spam patterns
	spamPatterns := []string{"free", "win", "click", "money", "urgent", "limited"}
	messageLower := strings.ToLower(message)
	for _, pattern := range spamPatterns {
		if strings.Contains(messageLower, pattern) {
			score += 0.1
		}
	}

	return score
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(options *SecurityOptions) *SecurityManager {
	if options == nil {
		options = &SecurityOptions{
			MaxRequestsPerMinute:  10,
			MaxRequestsPerHour:    100,
			MaxRequestsPerDay:     1000,
			SpamThreshold:         0.5,
			EnableSecurityLogging: true,
			UserIDField:           "ParticipantName",
		}
	}

	return &SecurityManager{
		options:   options,
		rateLimit: NewRateLimiter(options.MaxRequestsPerMinute, options.MaxRequestsPerHour, options.MaxRequestsPerDay),
		spam:      NewSpamDetector(options.SpamThreshold),
		requests:  make(map[string][]SecurityLog),
	}
}

// CheckSecurity checks security rules for a request
func (sm *SecurityManager) CheckSecurity(userID, sessionID, message string) error {
	// Check rate limits
	if err := sm.rateLimit.Check(userID); err != nil {
		return fmt.Errorf("rate limit: %v", err)
	}

	// Check spam
	spamScore := sm.spam.Detect(message)
	if spamScore >= sm.options.SpamThreshold {
		return fmt.Errorf("spam detected (score: %.2f)", spamScore)
	}

	return nil
}

// LogSecurityEvent logs security events
func (sm *SecurityManager) LogSecurityEvent(log SecurityLog) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.requests[log.UserID] = append(sm.requests[log.UserID], log)

	// Keep only last 1000 requests per user
	if len(sm.requests[log.UserID]) > 1000 {
		sm.requests[log.UserID] = sm.requests[log.UserID][len(sm.requests[log.UserID])-1000:]
	}

	if sm.options.EnableSecurityLogging {
		fmt.Printf("[SECURITY] user=%s session=%s spam_score=%.2f allowed=%v error=%s\n",
			log.UserID, log.SessionID, log.SpamScore, log.Allowed, log.Error)
	}
}

// GetUserAnalytics returns analytics for a specific user
func (sm *SecurityManager) GetUserAnalytics(userID string, startTime, endTime time.Time) UserAnalytics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	requests := sm.requests[userID]
	analytics := UserAnalytics{
		UserID:    userID,
		StartTime: startTime,
		EndTime:   endTime,
	}

	for _, req := range requests {
		if req.Timestamp.After(startTime) && req.Timestamp.Before(endTime) {
			analytics.TotalRequests++
			if req.Allowed {
				analytics.AllowedRequests++
			} else {
				analytics.BlockedRequests++
			}
			analytics.TotalSpamScore += req.SpamScore
		}
	}

	if analytics.TotalRequests > 0 {
		analytics.AverageSpamScore = analytics.TotalSpamScore / float64(analytics.TotalRequests)
	}

	return analytics
}

// GetAllAnalytics returns analytics for all users
func (sm *SecurityManager) GetAllAnalytics(startTime, endTime time.Time) []UserAnalytics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	userAnalytics := make([]UserAnalytics, 0)
	processedUsers := make(map[string]bool)

	for userID := range sm.requests {
		if !processedUsers[userID] {
			analytics := sm.GetUserAnalytics(userID, startTime, endTime)
			userAnalytics = append(userAnalytics, analytics)
			processedUsers[userID] = true
		}
	}

	return userAnalytics
}

// ResetUserLimits resets rate limits for a specific user
func (sm *SecurityManager) ResetUserLimits(userID string) {
	sm.rateLimit.Reset(userID)
}

// GetSecurityStats returns current security statistics
func (sm *SecurityManager) GetSecurityStats() map[string]interface{} {
	return map[string]interface{}{
		"rate_limiting_enabled":   true,
		"spam_detection_enabled":  true,
		"logging_enabled":         sm.options.EnableSecurityLogging,
		"max_requests_per_minute": sm.options.MaxRequestsPerMinute,
		"max_requests_per_hour":   sm.options.MaxRequestsPerHour,
		"max_requests_per_day":    sm.options.MaxRequestsPerDay,
		"spam_threshold":          sm.options.SpamThreshold,
	}
}

// hashMessage creates a SHA256 hash of the message for privacy
func hashMessage(message string) string {
	hash := sha256.Sum256([]byte(message))
	return hex.EncodeToString(hash[:])
}
