package cs_ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRealisticToolCacheScenario tests a realistic scenario where tool definition changes
func TestRealisticToolCacheScenario(t *testing.T) {
	// Scenario: User modifies a tool by removing a required field
	// Before: Tool requires 'name' field
	// After: Tool no longer requires 'name' field (but still accepts it)

	// Step 1: Create initial tool definition with required field
	initialIntent := &mockIntent{
		code:        "get_user_info",
		description: []string{"Get user information"},
		param: struct {
			Name string `json:"name" validate:"required" description:"User name"`
			Age  int    `json:"age" description:"User age"`
		}{
			Name: "John",
			Age:  25,
		},
	}

	// Step 2: Generate hash for initial tool
	initialHash, err := generateToolDefinitionHash(initialIntent)
	assert.NoError(t, err)
	assert.NotEmpty(t, initialHash)

	// Step 3: Simulate tool execution with cache
	cache := make(map[ToolCacheKey]string)

	// First execution - should not be cached
	key1 := ToolCacheKey{
		FunctionName:       "get_user_info",
		Arguments:          `{"name": "John", "age": 25}`,
		ToolDefinitionHash: initialHash,
	}

	// Simulate cache miss and store result
	cache[key1] = "User: John, Age: 25"

	// Verify cache hit
	result1, exists := cache[key1]
	assert.True(t, exists)
	assert.Equal(t, "User: John, Age: 25", result1)

	// Step 4: User modifies tool definition - removes required field
	modifiedIntent := &mockIntent{
		code:        "get_user_info",
		description: []string{"Get user information"},
		param: struct {
			Name string `json:"name" description:"User name"` // removed validate:"required"
			Age  int    `json:"age" description:"User age"`
		}{
			Name: "John",
			Age:  25,
		},
	}

	// Step 5: Generate hash for modified tool
	modifiedHash, err := generateToolDefinitionHash(modifiedIntent)
	assert.NoError(t, err)
	assert.NotEqual(t, initialHash, modifiedHash, "Modified tool should have different hash")

	// Step 6: Same arguments but different tool definition hash
	key2 := ToolCacheKey{
		FunctionName:       "get_user_info",
		Arguments:          `{"name": "John", "age": 25}`,
		ToolDefinitionHash: modifiedHash,
	}

	// Step 7: Verify cache miss for modified tool (different hash)
	_, exists = cache[key2]
	assert.False(t, exists, "Modified tool should not hit old cache")

	// Step 8: Simulate new execution with modified tool
	cache[key2] = "User: John, Age: 25 (optional name)"

	// Step 9: Verify both cache entries exist independently
	result1, exists = cache[key1]
	assert.True(t, exists)
	assert.Equal(t, "User: John, Age: 25", result1)

	result2, exists := cache[key2]
	assert.True(t, exists)
	assert.Equal(t, "User: John, Age: 25 (optional name)", result2)

	// Step 10: Test that old cache is not affected by new tool definition
	assert.NotEqual(t, result1, result2, "Old and new cache should be independent")
}

// TestToolCacheWithMultipleParameterChanges tests multiple parameter changes
func TestToolCacheWithMultipleParameterChanges(t *testing.T) {
	// Test various parameter changes that should invalidate cache

	testCases := []struct {
		name        string
		description string
		param       interface{}
	}{
		{
			name:        "add_required_field",
			description: "Add required field",
			param: struct {
				Name  string `json:"name" validate:"required" description:"User name"`
				Email string `json:"email" validate:"required" description:"User email"`
			}{
				Name:  "John",
				Email: "john@example.com",
			},
		},
		{
			name:        "remove_required_field",
			description: "Remove required field",
			param: struct {
				Name string `json:"name" description:"User name"`
			}{
				Name: "John",
			},
		},
		{
			name:        "change_field_type",
			description: "Change field type",
			param: struct {
				Age string `json:"age" description:"User age as string"`
			}{
				Age: "25",
			},
		},
		{
			name:        "add_optional_field",
			description: "Add optional field",
			param: struct {
				Name    string `json:"name" validate:"required" description:"User name"`
				Address string `json:"address" description:"User address"`
			}{
				Name:    "John",
				Address: "123 Main St",
			},
		},
	}

	// Generate hashes for all test cases
	hashes := make([]string, len(testCases))
	for i, tc := range testCases {
		intent := &mockIntent{
			code:        "test_function",
			description: []string{tc.description},
			param:       tc.param,
		}

		hash, err := generateToolDefinitionHash(intent)
		assert.NoError(t, err)
		hashes[i] = hash
	}

	// Verify all hashes are different
	for i := 0; i < len(hashes); i++ {
		for j := i + 1; j < len(hashes); j++ {
			assert.NotEqual(t, hashes[i], hashes[j],
				"Hash for %s should be different from %s",
				testCases[i].name, testCases[j].name)
		}
	}
}

// TestToolCacheKeyUniqueness tests that cache keys are unique for different scenarios
func TestToolCacheKeyUniqueness(t *testing.T) {
	// Create a base intent
	baseIntent := &mockIntent{
		code:        "unique_test",
		description: []string{"Unique test"},
		param: struct {
			Value string `json:"value" validate:"required" description:"Test value"`
		}{
			Value: "test",
		},
	}

	baseHash, err := generateToolDefinitionHash(baseIntent)
	assert.NoError(t, err)

	// Test different scenarios that should create unique cache keys
	scenarios := []struct {
		name         string
		functionName string
		arguments    string
		hash         string
	}{
		{
			name:         "base_scenario",
			functionName: "unique_test",
			arguments:    `{"value": "test1"}`,
			hash:         baseHash,
		},
		{
			name:         "different_arguments",
			functionName: "unique_test",
			arguments:    `{"value": "test2"}`,
			hash:         baseHash,
		},
		{
			name:         "different_function",
			functionName: "different_test",
			arguments:    `{"value": "test1"}`,
			hash:         baseHash,
		},
		{
			name:         "different_hash",
			functionName: "unique_test",
			arguments:    `{"value": "test1"}`,
			hash:         "different_hash",
		},
	}

	// Create cache keys for all scenarios
	keys := make([]ToolCacheKey, len(scenarios))
	for i, scenario := range scenarios {
		keys[i] = ToolCacheKey{
			FunctionName:       scenario.functionName,
			Arguments:          scenario.arguments,
			ToolDefinitionHash: scenario.hash,
		}
	}

	// Verify all keys are unique
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			assert.NotEqual(t, keys[i], keys[j],
				"Cache key for %s should be different from %s",
				scenarios[i].name, scenarios[j].name)
		}
	}
}

// TestToolCachePerformance tests cache performance with many entries
func TestToolCachePerformance(t *testing.T) {
	// Create a large cache to test performance
	cache := make(map[ToolCacheKey]string)

	// Create many cache entries
	numEntries := 1000
	for i := 0; i < numEntries; i++ {
		key := ToolCacheKey{
			FunctionName:       "perf_test",
			Arguments:          `{"id": ` + string(rune(i)) + `}`,
			ToolDefinitionHash: "test_hash",
		}
		cache[key] = "result_" + string(rune(i))
	}

	// Test cache retrieval performance
	for i := 0; i < numEntries; i++ {
		key := ToolCacheKey{
			FunctionName:       "perf_test",
			Arguments:          `{"id": ` + string(rune(i)) + `}`,
			ToolDefinitionHash: "test_hash",
		}
		result, exists := cache[key]
		assert.True(t, exists)
		assert.Equal(t, "result_"+string(rune(i)), result)
	}

	// Test cache size
	assert.Equal(t, numEntries, len(cache))
}
