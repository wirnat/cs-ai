package cs_ai

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestMain runs all tests
func TestMain(m *testing.M) {
	fmt.Println("🧪 Running comprehensive CsAI security test suite...")

	// Run all tests
	m.Run()

	fmt.Println("✅ All tests completed!")
}

// TestSecurityFeatures tests security functionality
func TestSecurityFeatures(t *testing.T) {
	t.Run("RateLimiting", testRateLimiting)
	t.Run("SpamDetection", testSpamDetection)
	t.Run("SecurityLogging", testSecurityLogging)
	t.Run("Analytics", testAnalytics)
	t.Run("EndToEnd", testEndToEnd)
}

// testRateLimiting tests rate limiting functionality
func testRateLimiting(t *testing.T) {
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

// testSpamDetection tests spam detection functionality
func testSpamDetection(t *testing.T) {
	sd := NewSpamDetector(0.5)

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
			isSpam := score >= 0.5

			if isSpam != tt.expected {
				t.Errorf("Expected spam=%v for %q, got score %.2f", tt.expected, tt.message, score)
			}
		})
	}
}

// testSecurityLogging tests security logging
func testSecurityLogging(t *testing.T) {
	sm := NewSecurityManager(&SecurityOptions{
		MaxRequestsPerMinute:  10,
		MaxRequestsPerHour:    100,
		MaxRequestsPerDay:     1000,
		SpamThreshold:         0.5,
		EnableSecurityLogging: true,
		UserIDField:           "test_user",
	})

	// Test normal operation
	err := sm.CheckSecurity("user1", "session1", "normal message")
	if err != nil {
		t.Errorf("Expected no error for normal message, got: %v", err)
	}

	// Test spam detection
	err = sm.CheckSecurity("user2", "session2", "WIN FREE MONEY!!!")
	if err == nil {
		t.Error("Expected spam detection error")
	}
}

// testAnalytics tests security analytics
func testAnalytics(t *testing.T) {
	sm := NewSecurityManager(nil)
	userID := "analytics_user"
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	// Simulate requests
	for i := 0; i < 5; i++ {
		err := sm.CheckSecurity(userID, fmt.Sprintf("session%d", i), fmt.Sprintf("message %d", i))
		if err == nil {
			sm.LogSecurityEvent(SecurityLog{
				SessionID:   fmt.Sprintf("session%d", i),
				UserID:      userID,
				MessageHash: hashMessage(fmt.Sprintf("message %d", i)),
				Timestamp:   time.Now(),
				SpamScore:   0.1,
				Allowed:     true,
			})
		}
	}

	analytics := sm.GetUserAnalytics(userID, startTime, endTime)
	if analytics.TotalRequests != 5 {
		t.Errorf("Expected 5 requests, got %d", analytics.TotalRequests)
	}
}

// TestModel is a test model for testing
type TestModel struct {
	Responses map[string]Message
}

func (tm *TestModel) ModelName() string {
	return "test-model"
}

func (tm *TestModel) ApiURL() string {
	return "http://localhost:8080/test"
}

func (tm *TestModel) Train() []string {
	return []string{"You are a test assistant"}
}

// TestIntent is a test intent for testing
type TestIntent struct {
	code        string
	description []string
	param       interface{}
	handler     func(ctx context.Context, params map[string]interface{}) (interface{}, error)
}

func (ti *TestIntent) Code() string {
	return ti.code
}

func (ti *TestIntent) Description() []string {
	return ti.description
}

func (ti *TestIntent) Param() interface{} {
	return ti.param
}

func (ti *TestIntent) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return ti.handler(ctx, params)
}

// testEndToEnd tests complete functionality
func testEndToEnd(t *testing.T) {
	// Create test model
	model := &TestModel{
		Responses: map[string]Message{
			"hello": {
				Content: "Hello! How can I help you?",
				Role:    "assistant",
			},
		},
	}

	// Create CsAI instance with security
	cs := New("test-api-key", model, Options{
		SecurityOptions: &SecurityOptions{
			MaxRequestsPerMinute:  100,
			MaxRequestsPerHour:    1000,
			MaxRequestsPerDay:     10000,
			SpamThreshold:         0.8,
			EnableSecurityLogging: true,
		},
	})

	// Test normal operation
	userMessage := UserMessage{
		Message:         "hello",
		ParticipantName: "test_user",
	}

	ctx := context.Background()
	response, err := cs.Exec(ctx, "test_session", userMessage)
	if err != nil {
		t.Fatalf("End-to-end test failed: %v", err)
	}

	if response.Content == "" {
		t.Error("Expected non-empty response")
	}

	// Test security features
	// Test rate limiting
	for i := 0; i < 3; i++ {
		_, err := cs.Exec(ctx, fmt.Sprintf("session%d", i), userMessage)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

// RunTests runs all tests
func RunTests() bool {
	fmt.Println("🧪 Running comprehensive test suite...")

	// Create test runner
	tests := []struct {
		name string
		fn   func(*testing.T)
	}{
		{"RateLimiting", testRateLimiting},
		{"SpamDetection", testSpamDetection},
		{"SecurityLogging", testSecurityLogging},
		{"Analytics", testAnalytics},
		{"EndToEnd", testEndToEnd},
	}

	passed := 0
	total := len(tests)

	for _, test := range tests {
		t := &testing.T{}
		test.fn(t)
		if !t.Failed() {
			fmt.Printf("✅ %s\n", test.name)
			passed++
		} else {
			fmt.Printf("❌ %s\n", test.name)
		}
	}

	fmt.Printf("📊 Results: %d/%d tests passed\n", passed, total)
	return passed == total
}

// ExampleTestUsage demonstrates test usage
func ExampleTestUsage() {
	fmt.Println("=== CsAI Security Test Suite ===")

	// Run tests
	success := RunTests()
	if success {
		fmt.Println("🎉 All tests passed! Security features are working correctly.")
	} else {
		fmt.Println("⚠️  Some tests failed. Check implementation.")
	}
}
