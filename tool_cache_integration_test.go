package cs_ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestToolCacheInvalidation tests the complete tool cache invalidation flow
func TestToolCacheInvalidation(t *testing.T) {
	// Create mock intents with different parameter structures
	intent1 := &mockIntent{
		code:        "test_function",
		description: []string{"Test function with required field"},
		param: struct {
			Name string  `json:"name" validate:"required" description:"desc name"`
			Date *string `json:"date" description:"desc date"`
		}{
			Name: "test",
			Date: nil,
		},
	}

	intent2 := &mockIntent{
		code:        "test_function",
		description: []string{"Test function without required field"},
		param: struct {
			Name string  `json:"name" description:"desc name"`
			Date *string `json:"date" description:"desc date"`
		}{
			Name: "test",
			Date: nil,
		},
	}

	// Test that different tool definitions generate different hashes
	hash1, err1 := generateToolDefinitionHash(intent1)
	hash2, err2 := generateToolDefinitionHash(intent2)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotEqual(t, hash1, hash2, "Different tool definitions should generate different hashes")

	// Test that same tool definition generates same hash
	hash1Again, err3 := generateToolDefinitionHash(intent1)
	assert.NoError(t, err3)
	assert.Equal(t, hash1, hash1Again, "Same tool definition should generate same hash")
}

// TestToolCacheKeyConsistency tests that cache keys are generated consistently
func TestToolCacheKeyConsistency(t *testing.T) {
	intent := &mockIntent{
		code:        "test_function",
		description: []string{"Test function"},
		param: struct {
			Name string `json:"name" validate:"required" description:"desc name"`
		}{
			Name: "test",
		},
	}

	hash, err := generateToolDefinitionHash(intent)
	assert.NoError(t, err)

	// Create cache keys with same parameters
	key1 := ToolCacheKey{
		FunctionName:       "test_function",
		Arguments:          `{"name": "test"}`,
		ToolDefinitionHash: hash,
	}

	key2 := ToolCacheKey{
		FunctionName:       "test_function",
		Arguments:          `{"name": "test"}`,
		ToolDefinitionHash: hash,
	}

	// Test that identical cache keys are equal
	assert.Equal(t, key1, key2, "Identical cache keys should be equal")

	// Test that different hashes create different cache keys
	key3 := ToolCacheKey{
		FunctionName:       "test_function",
		Arguments:          `{"name": "test"}`,
		ToolDefinitionHash: "different_hash",
	}

	assert.NotEqual(t, key1, key3, "Different hashes should create different cache keys")
}

// TestToolCacheKeyWithDifferentArguments tests cache key behavior with different arguments
func TestToolCacheKeyWithDifferentArguments(t *testing.T) {
	intent := &mockIntent{
		code:        "test_function",
		description: []string{"Test function"},
		param: struct {
			Name string `json:"name" validate:"required" description:"desc name"`
		}{
			Name: "test",
		},
	}

	hash, err := generateToolDefinitionHash(intent)
	assert.NoError(t, err)

	// Same function, same hash, different arguments
	key1 := ToolCacheKey{
		FunctionName:       "test_function",
		Arguments:          `{"name": "test1"}`,
		ToolDefinitionHash: hash,
	}

	key2 := ToolCacheKey{
		FunctionName:       "test_function",
		Arguments:          `{"name": "test2"}`,
		ToolDefinitionHash: hash,
	}

	// Different arguments should create different cache keys
	assert.NotEqual(t, key1, key2, "Different arguments should create different cache keys")
}

// TestToolDefinitionHashStability tests that hash generation is stable and deterministic
func TestToolDefinitionHashStability(t *testing.T) {
	intent := &mockIntent{
		code:        "stable_function",
		description: []string{"Stable function"},
		param: struct {
			ID   int    `json:"id" validate:"required" description:"desc id"`
			Name string `json:"name" description:"desc name"`
		}{
			ID:   1,
			Name: "test",
		},
	}

	// Generate hash multiple times
	hashes := make([]string, 10)
	for i := 0; i < 10; i++ {
		hash, err := generateToolDefinitionHash(intent)
		assert.NoError(t, err)
		hashes[i] = hash
	}

	// All hashes should be identical
	for i := 1; i < len(hashes); i++ {
		assert.Equal(t, hashes[0], hashes[i], "Hash generation should be deterministic")
	}
}

// TestToolCacheKeyAsMapKey tests that ToolCacheKey can be used as map key
func TestToolCacheKeyAsMapKey(t *testing.T) {
	intent := &mockIntent{
		code:        "map_key_test",
		description: []string{"Map key test"},
		param: struct {
			Value string `json:"value" validate:"required" description:"desc value"`
		}{
			Value: "test",
		},
	}

	hash, err := generateToolDefinitionHash(intent)
	assert.NoError(t, err)

	// Create cache map
	cache := make(map[ToolCacheKey]string)

	key1 := ToolCacheKey{
		FunctionName:       "map_key_test",
		Arguments:          `{"value": "test1"}`,
		ToolDefinitionHash: hash,
	}

	key2 := ToolCacheKey{
		FunctionName:       "map_key_test",
		Arguments:          `{"value": "test2"}`,
		ToolDefinitionHash: hash,
	}

	// Store values
	cache[key1] = "result1"
	cache[key2] = "result2"

	// Retrieve values
	assert.Equal(t, "result1", cache[key1])
	assert.Equal(t, "result2", cache[key2])

	// Test that we can't retrieve with different hash
	key3 := ToolCacheKey{
		FunctionName:       "map_key_test",
		Arguments:          `{"value": "test1"}`,
		ToolDefinitionHash: "different_hash",
	}

	_, exists := cache[key3]
	assert.False(t, exists, "Should not find cache entry with different hash")
}

// TestToolDefinitionHashChangesWithParameterModifications tests various parameter changes
func TestToolDefinitionHashChangesWithParameterModifications(t *testing.T) {
	baseIntent := &mockIntent{
		code:        "modification_test",
		description: []string{"Modification test"},
		param: struct {
			Name string `json:"name" validate:"required" description:"desc name"`
		}{
			Name: "test",
		},
	}

	baseHash, err := generateToolDefinitionHash(baseIntent)
	assert.NoError(t, err)

	// Test 1: Change required field to optional
	intent1 := &mockIntent{
		code:        "modification_test",
		description: []string{"Modification test"},
		param: struct {
			Name string `json:"name" description:"desc name"` // removed validate:"required"
		}{
			Name: "test",
		},
	}

	hash1, err := generateToolDefinitionHash(intent1)
	assert.NoError(t, err)
	assert.NotEqual(t, baseHash, hash1, "Removing required field should change hash")

	// Test 2: Change description
	intent2 := &mockIntent{
		code:        "modification_test",
		description: []string{"Modification test"},
		param: struct {
			Name string `json:"name" validate:"required" description:"different description"`
		}{
			Name: "test",
		},
	}

	hash2, err := generateToolDefinitionHash(intent2)
	assert.NoError(t, err)
	assert.NotEqual(t, baseHash, hash2, "Changing description should change hash")

	// Test 3: Add new field
	intent3 := &mockIntent{
		code:        "modification_test",
		description: []string{"Modification test"},
		param: struct {
			Name  string `json:"name" validate:"required" description:"desc name"`
			Email string `json:"email" description:"desc email"`
		}{
			Name:  "test",
			Email: "test@example.com",
		},
	}

	hash3, err := generateToolDefinitionHash(intent3)
	assert.NoError(t, err)
	assert.NotEqual(t, baseHash, hash3, "Adding new field should change hash")

	// Test 4: Change function name
	intent4 := &mockIntent{
		code:        "different_function_name",
		description: []string{"Modification test"},
		param: struct {
			Name string `json:"name" validate:"required" description:"desc name"`
		}{
			Name: "test",
		},
	}

	hash4, err := generateToolDefinitionHash(intent4)
	assert.NoError(t, err)
	assert.NotEqual(t, baseHash, hash4, "Changing function name should change hash")

	// Test 5: Change description array
	intent5 := &mockIntent{
		code:        "modification_test",
		description: []string{"Different description"},
		param: struct {
			Name string `json:"name" validate:"required" description:"desc name"`
		}{
			Name: "test",
		},
	}

	hash5, err := generateToolDefinitionHash(intent5)
	assert.NoError(t, err)
	assert.NotEqual(t, baseHash, hash5, "Changing description array should change hash")
}

// BenchmarkToolDefinitionHash benchmarks hash generation performance
func BenchmarkToolDefinitionHash(b *testing.B) {
	intent := &mockIntent{
		code:        "benchmark_test",
		description: []string{"Benchmark test"},
		param: struct {
			ID       int    `json:"id" validate:"required" description:"desc id"`
			Name     string `json:"name" validate:"required" description:"desc name"`
			Email    string `json:"email" description:"desc email"`
			Category string `json:"category" description:"desc category"`
		}{
			ID:       1,
			Name:     "test",
			Email:    "test@example.com",
			Category: "test",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generateToolDefinitionHash(intent)
		if err != nil {
			b.Fatal(err)
		}
	}
}
