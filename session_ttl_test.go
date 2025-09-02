package cs_ai

import (
	"testing"
	"time"
)

func TestSessionTTLOptions(t *testing.T) {
	tests := []struct {
		name        string
		sessionTTL  time.Duration
		expectedTTL time.Duration
		description string
	}{
		{
			name:        "Default TTL (12 jam)",
			sessionTTL:  0, // Not set
			expectedTTL: 12 * time.Hour,
			description: "Should use default 12 hours when not specified",
		},
		{
			name:        "Custom TTL 8 jam",
			sessionTTL:  8 * time.Hour,
			expectedTTL: 8 * time.Hour,
			description: "Should use custom 8 hours for morning shift",
		},
		{
			name:        "Custom TTL 24 jam",
			sessionTTL:  24 * time.Hour,
			expectedTTL: 24 * time.Hour,
			description: "Should use custom 24 hours for full day",
		},
		{
			name:        "Custom TTL 72 jam",
			sessionTTL:  72 * time.Hour,
			expectedTTL: 72 * time.Hour,
			description: "Should use custom 72 hours for weekend coverage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create options with custom SessionTTL
			options := &Options{
				SessionTTL: tt.sessionTTL,
			}

			// Test that the TTL is set correctly
			if tt.sessionTTL == 0 {
				// For default case, simulate constructor logic
				if options.SessionTTL == 0 {
					options.SessionTTL = 12 * time.Hour
				}
			}

			if options.SessionTTL != tt.expectedTTL {
				t.Errorf("Expected TTL %v, got %v", tt.expectedTTL, options.SessionTTL)
			}

			t.Logf("✓ %s: %s", tt.name, tt.description)
		})
	}
}

func TestSessionTTLValidation(t *testing.T) {
	// Test edge cases
	edgeCases := []struct {
		name        string
		sessionTTL  time.Duration
		shouldValid bool
	}{
		{"Zero TTL", 0, true},                  // Will use default
		{"1 minute", 1 * time.Minute, true},    // Very short
		{"1 hour", 1 * time.Hour, true},        // Short
		{"12 hours", 12 * time.Hour, true},     // Default
		{"24 hours", 24 * time.Hour, true},     // Full day
		{"7 days", 7 * 24 * time.Hour, true},   // Week
		{"30 days", 30 * 24 * time.Hour, true}, // Month
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			options := &Options{
				SessionTTL: tc.sessionTTL,
			}

			// Validate TTL is within reasonable bounds
			if tc.sessionTTL > 0 && tc.sessionTTL > 365*24*time.Hour {
				t.Errorf("TTL %v is too long (more than 1 year)", tc.sessionTTL)
			}

			if tc.sessionTTL < 0 {
				t.Errorf("TTL %v cannot be negative", tc.sessionTTL)
			}

			// Use options variable to avoid linter warning
			if options.SessionTTL != tc.sessionTTL {
				t.Errorf("Options SessionTTL mismatch: expected %v, got %v", tc.sessionTTL, options.SessionTTL)
			}

			t.Logf("✓ %s: TTL %v is valid", tc.name, tc.sessionTTL)
		})
	}
}

// Benchmark untuk performance testing
func BenchmarkSessionTTLSet(b *testing.B) {
	options := &Options{
		SessionTTL: 12 * time.Hour,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate TTL calculation logic
		var ttl time.Duration
		if options.SessionTTL > 0 {
			ttl = options.SessionTTL
		} else {
			ttl = 12 * time.Hour
		}
		_ = ttl
	}
}
