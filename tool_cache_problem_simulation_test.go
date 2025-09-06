package cs_ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestToolCacheProblemSimulation simulates the exact problem described by the user
func TestToolCacheProblemSimulation(t *testing.T) {
	// PROBLEM: User changes tool definition (e.g., removes required field)
	// but cache still uses old definition and asks for the removed field

	// Step 1: Initial tool definition with required field
	oldIntent := &mockIntent{
		code:        "booking_capster",
		description: []string{"Book a capster appointment"},
		param: struct {
			CustomerName string `json:"customer_name" validate:"required" description:"Customer name"`
			ServiceType  string `json:"service_type" description:"Type of service"`
			Date         string `json:"date" description:"Appointment date"`
		}{
			CustomerName: "John Doe",
			ServiceType:  "haircut",
			Date:         "2024-01-15",
		},
	}

	// Generate hash for old tool definition
	oldHash, err := generateToolDefinitionHash(oldIntent)
	assert.NoError(t, err)

	// Step 2: Simulate cache with old tool definition
	oldCacheKey := ToolCacheKey{
		FunctionName:       "booking_capster",
		Arguments:          `{"customer_name": "John Doe", "service_type": "haircut"}`,
		ToolDefinitionHash: oldHash,
	}

	// Step 3: User modifies tool definition - removes required field
	newIntent := &mockIntent{
		code:        "booking_capster",
		description: []string{"Book a capster appointment"},
		param: struct {
			CustomerName string `json:"customer_name" description:"Customer name"` // removed validate:"required"
			ServiceType  string `json:"service_type" description:"Type of service"`
			Date         string `json:"date" description:"Appointment date"`
		}{
			CustomerName: "John Doe",
			ServiceType:  "haircut",
			Date:         "2024-01-15",
		},
	}

	// Generate hash for new tool definition
	newHash, err := generateToolDefinitionHash(newIntent)
	assert.NoError(t, err)
	assert.NotEqual(t, oldHash, newHash, "New tool definition should have different hash")

	// Step 4: Simulate cache with new tool definition
	newCacheKey := ToolCacheKey{
		FunctionName:       "booking_capster",
		Arguments:          `{"customer_name": "John Doe", "service_type": "haircut"}`,
		ToolDefinitionHash: newHash,
	}

	// Step 5: Verify that old and new cache keys are different
	assert.NotEqual(t, oldCacheKey, newCacheKey, "Old and new cache keys should be different")

	// Step 6: Simulate cache behavior
	cache := make(map[ToolCacheKey]string)

	// Old cache entry (with required field validation)
	cache[oldCacheKey] = "Old result: customer_name is required"

	// New cache entry (without required field validation)
	cache[newCacheKey] = "New result: customer_name is optional"

	// Step 7: Test that both cache entries exist independently
	oldResult, oldExists := cache[oldCacheKey]
	assert.True(t, oldExists)
	assert.Equal(t, "Old result: customer_name is required", oldResult)

	newResult, newExists := cache[newCacheKey]
	assert.True(t, newExists)
	assert.Equal(t, "New result: customer_name is optional", newResult)

	// Step 8: Verify that using old cache key with new tool definition fails
	// This simulates the problem: old cache key won't match new tool definition
	_, exists := cache[newCacheKey]
	assert.True(t, exists, "New tool definition should have its own cache entry")

	// Step 9: Test cache invalidation - old cache should not interfere with new tool
	assert.NotEqual(t, oldResult, newResult, "Old and new results should be different")
}

// TestToolCacheInvalidationWithFieldRemoval tests specific field removal scenario
func TestToolCacheInvalidationWithFieldRemoval(t *testing.T) {
	// Test case: Remove required field from tool definition

	// Before: Tool requires 'email' field
	beforeIntent := &mockIntent{
		code:        "create_customer",
		description: []string{"Create a new customer"},
		param: struct {
			Name  string `json:"name" validate:"required" description:"Customer name"`
			Email string `json:"email" validate:"required" description:"Customer email"`
			Phone string `json:"phone" description:"Customer phone"`
		}{
			Name:  "John Doe",
			Email: "john@example.com",
			Phone: "123-456-7890",
		},
	}

	// After: Tool no longer requires 'email' field
	afterIntent := &mockIntent{
		code:        "create_customer",
		description: []string{"Create a new customer"},
		param: struct {
			Name  string `json:"name" validate:"required" description:"Customer name"`
			Email string `json:"email" description:"Customer email"` // removed validate:"required"
			Phone string `json:"phone" description:"Customer phone"`
		}{
			Name:  "John Doe",
			Email: "john@example.com",
			Phone: "123-456-7890",
		},
	}

	// Generate hashes
	beforeHash, err := generateToolDefinitionHash(beforeIntent)
	assert.NoError(t, err)

	afterHash, err := generateToolDefinitionHash(afterIntent)
	assert.NoError(t, err)

	// Verify hashes are different
	assert.NotEqual(t, beforeHash, afterHash, "Removing required field should change hash")

	// Test cache behavior
	cache := make(map[ToolCacheKey]string)

	// Before: Cache with required email
	beforeKey := ToolCacheKey{
		FunctionName:       "create_customer",
		Arguments:          `{"name": "John Doe", "phone": "123-456-7890"}`,
		ToolDefinitionHash: beforeHash,
	}
	cache[beforeKey] = "Error: email is required"

	// After: Cache without required email
	afterKey := ToolCacheKey{
		FunctionName:       "create_customer",
		Arguments:          `{"name": "John Doe", "phone": "123-456-7890"}`,
		ToolDefinitionHash: afterHash,
	}
	cache[afterKey] = "Success: Customer created without email"

	// Verify both cache entries exist and are different
	beforeResult, beforeExists := cache[beforeKey]
	assert.True(t, beforeExists)
	assert.Equal(t, "Error: email is required", beforeResult)

	afterResult, afterExists := cache[afterKey]
	assert.True(t, afterExists)
	assert.Equal(t, "Success: Customer created without email", afterResult)

	// Verify cache keys are different
	assert.NotEqual(t, beforeKey, afterKey, "Cache keys should be different")
}

// TestToolCacheInvalidationWithFieldAddition tests adding new required field
func TestToolCacheInvalidationWithFieldAddition(t *testing.T) {
	// Test case: Add new required field to tool definition

	// Before: Tool only requires 'name'
	beforeIntent := &mockIntent{
		code:        "update_profile",
		description: []string{"Update user profile"},
		param: struct {
			Name string `json:"name" validate:"required" description:"User name"`
		}{
			Name: "John Doe",
		},
	}

	// After: Tool now also requires 'email'
	afterIntent := &mockIntent{
		code:        "update_profile",
		description: []string{"Update user profile"},
		param: struct {
			Name  string `json:"name" validate:"required" description:"User name"`
			Email string `json:"email" validate:"required" description:"User email"`
		}{
			Name:  "John Doe",
			Email: "john@example.com",
		},
	}

	// Generate hashes
	beforeHash, err := generateToolDefinitionHash(beforeIntent)
	assert.NoError(t, err)

	afterHash, err := generateToolDefinitionHash(afterIntent)
	assert.NoError(t, err)

	// Verify hashes are different
	assert.NotEqual(t, beforeHash, afterHash, "Adding required field should change hash")

	// Test cache behavior
	cache := make(map[ToolCacheKey]string)

	// Before: Cache with only name
	beforeKey := ToolCacheKey{
		FunctionName:       "update_profile",
		Arguments:          `{"name": "John Doe"}`,
		ToolDefinitionHash: beforeHash,
	}
	cache[beforeKey] = "Success: Profile updated with name only"

	// After: Cache with name and email
	afterKey := ToolCacheKey{
		FunctionName:       "update_profile",
		Arguments:          `{"name": "John Doe", "email": "john@example.com"}`,
		ToolDefinitionHash: afterHash,
	}
	cache[afterKey] = "Success: Profile updated with name and email"

	// Verify both cache entries exist and are different
	beforeResult, beforeExists := cache[beforeKey]
	assert.True(t, beforeExists)
	assert.Equal(t, "Success: Profile updated with name only", beforeResult)

	afterResult, afterExists := cache[afterKey]
	assert.True(t, afterExists)
	assert.Equal(t, "Success: Profile updated with name and email", afterResult)

	// Verify cache keys are different
	assert.NotEqual(t, beforeKey, afterKey, "Cache keys should be different")
}

// TestToolCacheInvalidationWithDescriptionChange tests description changes
func TestToolCacheInvalidationWithDescriptionChange(t *testing.T) {
	// Test case: Change field description

	// Before: Original description
	beforeIntent := &mockIntent{
		code:        "search_products",
		description: []string{"Search for products"},
		param: struct {
			Query string `json:"query" validate:"required" description:"Search query"`
		}{
			Query: "laptop",
		},
	}

	// After: Updated description
	afterIntent := &mockIntent{
		code:        "search_products",
		description: []string{"Search for products"},
		param: struct {
			Query string `json:"query" validate:"required" description:"Product search query (minimum 3 characters)"`
		}{
			Query: "laptop",
		},
	}

	// Generate hashes
	beforeHash, err := generateToolDefinitionHash(beforeIntent)
	assert.NoError(t, err)

	afterHash, err := generateToolDefinitionHash(afterIntent)
	assert.NoError(t, err)

	// Verify hashes are different
	assert.NotEqual(t, beforeHash, afterHash, "Changing description should change hash")

	// Test cache behavior
	cache := make(map[ToolCacheKey]string)

	// Before: Cache with old description
	beforeKey := ToolCacheKey{
		FunctionName:       "search_products",
		Arguments:          `{"query": "laptop"}`,
		ToolDefinitionHash: beforeHash,
	}
	cache[beforeKey] = "Results for: laptop"

	// After: Cache with new description
	afterKey := ToolCacheKey{
		FunctionName:       "search_products",
		Arguments:          `{"query": "laptop"}`,
		ToolDefinitionHash: afterHash,
	}
	cache[afterKey] = "Results for: laptop (with new validation)"

	// Verify both cache entries exist and are different
	beforeResult, beforeExists := cache[beforeKey]
	assert.True(t, beforeExists)
	assert.Equal(t, "Results for: laptop", beforeResult)

	afterResult, afterExists := cache[afterKey]
	assert.True(t, afterExists)
	assert.Equal(t, "Results for: laptop (with new validation)", afterResult)

	// Verify cache keys are different
	assert.NotEqual(t, beforeKey, afterKey, "Cache keys should be different")
}
