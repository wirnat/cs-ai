package cs_ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type getItemStockParam struct {
	Name string  `json:"name" validate:"required" description:"desc name"`
	Date *string `json:"date" description:"desc date"`
}

func TestConvertParam(t *testing.T) {
	tests := []struct {
		name     string
		param    interface{}
		expected map[string]interface{}
	}{
		{
			name: "GetItemStockParam",
			param: getItemStockParam{
				Name: "Item1",
				Date: nil,
			},
			expected: map[string]interface{}{"properties": map[string]map[string]interface{}{"date": map[string]interface{}{"type": "string", "description": "desc date", "default": nil}, "name": map[string]interface{}{"type": "string", "description": "desc name", "default": nil}}, "required": []string{"name"}, "type": "object"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertParam(tt.param)
			assert.NoError(t, err)

			assert.NoError(t, err)

			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock intent untuk testing
type mockIntent struct {
	code        string
	description []string
	param       interface{}
}

func (m *mockIntent) Code() string {
	return m.code
}

func (m *mockIntent) Description() []string {
	return m.description
}

func (m *mockIntent) Param() interface{} {
	return m.param
}

func (m *mockIntent) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func TestGenerateToolDefinitionHash(t *testing.T) {
	tests := []struct {
		name     string
		intent   Intent
		expected string
	}{
		{
			name: "Basic intent with required field",
			intent: &mockIntent{
				code:        "test_function",
				description: []string{"Test function"},
				param: getItemStockParam{
					Name: "test",
					Date: nil,
				},
			},
			expected: "", // We'll check that it's not empty
		},
		{
			name: "Intent with different required field",
			intent: &mockIntent{
				code:        "test_function",
				description: []string{"Test function"},
				param: struct {
					ID   int    `json:"id" validate:"required" description:"desc id"`
					Name string `json:"name" description:"desc name"`
				}{
					ID:   1,
					Name: "test",
				},
			},
			expected: "", // We'll check that it's not empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := generateToolDefinitionHash(tt.intent)
			assert.NoError(t, err)
			assert.NotEmpty(t, hash)
			assert.Len(t, hash, 64) // SHA256 hex string length
		})
	}
}

func TestToolDefinitionHashChanges(t *testing.T) {
	// Test that hash changes when tool definition changes
	intent1 := &mockIntent{
		code:        "test_function",
		description: []string{"Test function"},
		param: getItemStockParam{
			Name: "test",
			Date: nil,
		},
	}

	intent2 := &mockIntent{
		code:        "test_function",
		description: []string{"Test function"},
		param: struct {
			ID   int    `json:"id" validate:"required" description:"desc id"`
			Name string `json:"name" description:"desc name"`
		}{
			ID:   1,
			Name: "test",
		},
	}

	hash1, err1 := generateToolDefinitionHash(intent1)
	hash2, err2 := generateToolDefinitionHash(intent2)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotEqual(t, hash1, hash2, "Hash should be different for different tool definitions")
}
