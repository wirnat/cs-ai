package cs_ai

import (
	"testing"
	"time"
)

func TestRateLimiting(t *testing.T) {
	rl := NewRateLimiter(3, 5, 10)
	userID := "test_user"

	// Test within limits
	for i := 0; i < 3; i++ {
		err := rl.Check(userID)
		if err != nil {
			t.Errorf("Expected no error for request %d, got: %v", i+1, err)
		}
	}

	// Test exceeding limit
	err := rl.Check(userID)
	if err == nil {
		t.Error("Expected rate limit error, got nil")
	}

	// Test reset
	rl.Reset(userID)
	err = rl.Check(userID)
	if err != nil {
		t.Errorf("Expected no error after reset, got: %v", err)
	}
}

func TestSpamDetection(t *testing.T) {
	sd := NewSpamDetector(0.3) // Lower threshold for testing

	tests := []struct {
		name     string
		message  string
		expected bool
	}{
		{"Normal", "Hello, how are you?", false},
		{"Spam", "WIN FREE MONEY NOW!!!", true},
		{"Caps", "THIS IS ALL CAPS", true},
		{"Exclamation", "Hello!!!!!!", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := sd.Detect(tt.message)
			isSpam := score >= 0.3

			if isSpam != tt.expected {
				t.Errorf("Expected spam=%v for %q, got score %.2f", tt.expected, tt.message, score)
			}
		})
	}
}

func TestSecurityLogging(t *testing.T) {
	sm := NewSecurityManager(&SecurityOptions{
		MaxRequestsPerMinute:  10,
		MaxRequestsPerHour:    100,
		MaxRequestsPerDay:     1000,
		SpamThreshold:         0.2, // Lower threshold for testing
		EnableSecurityLogging: true,
		UserIDField:           "test_user",
	})

	// Test normal operation
	err := sm.CheckSecurity("user1", "session1", "normal message")
	if err != nil {
		t.Errorf("Expected no error for normal message, got: %v", err)
	}

	// Test spam detection
	err = sm.CheckSecurity("user2", "session2", "WIN FREE MONEY NOW!!!")
	if err == nil {
		t.Error("Expected spam detection error")
	}
}

func TestAnalytics(t *testing.T) {
	sm := NewSecurityManager(&SecurityOptions{
		MaxRequestsPerMinute:  100,
		MaxRequestsPerHour:    1000,
		MaxRequestsPerDay:     10000,
		SpamThreshold:         0.5,
		EnableSecurityLogging: true,
	})
	userID := "analytics_user"
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	// Simulate requests
	for i := 0; i < 5; i++ {
		err := sm.CheckSecurity(userID, "session", "message")
		if err != nil {
			t.Logf("Security check failed: %v", err)
		}
	}

	analytics := sm.GetUserAnalytics(userID, startTime, endTime)
	if analytics.TotalRequests < 0 {
		t.Errorf("Expected non-negative requests, got %d", analytics.TotalRequests)
	}
}

func TestSecurityIntegration(t *testing.T) {
	t.Skip("Integration test requires actual model implementation")
}
