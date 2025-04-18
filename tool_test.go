package cs_ai

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
			expected: map[string]interface{}{"properties": map[string]map[string]string{"date": map[string]string{"type": "string", "description": "desc date"}, "name": map[string]string{"type": "string", "description": "desc name"}}, "required": []string{"name"}, "type": "object"},
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
