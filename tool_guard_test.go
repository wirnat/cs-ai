package cs_ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type guardTestModel struct{}

func (m *guardTestModel) ModelName() string {
	return "guard-test-model"
}

func (m *guardTestModel) ApiURL() string {
	return "http://localhost/guard-test"
}

func (m *guardTestModel) Train() []string {
	return []string{"base system prompt"}
}

type guardTestIntent struct {
	code string
}

func (i *guardTestIntent) Code() string {
	return i.code
}

func (i *guardTestIntent) Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{}, nil
}

func (i *guardTestIntent) Description() []string {
	return []string{"guard test intent"}
}

func (i *guardTestIntent) Param() interface{} {
	return struct{}{}
}

func TestBuildToolErrorMessage(t *testing.T) {
	msg := buildToolErrorMessage("call_1", "tool_not_found", "fake_tool", []string{"tool_a", "tool_b"})

	assert.Equal(t, Tool, msg.Role)
	assert.Equal(t, "call_1", msg.ToolCallID)
	assert.NotEmpty(t, msg.Content)

	var payload map[string]interface{}
	err := json.Unmarshal([]byte(msg.Content), &payload)
	require.NoError(t, err)

	errorData, ok := payload["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "tool_not_found", errorData["code"])
	assert.Equal(t, "fake_tool", errorData["requested_tool"])

	availableTools, ok := errorData["available_tools"].([]interface{})
	require.True(t, ok)
	assert.Len(t, availableTools, 2)
	assert.Contains(t, availableTools, "tool_a")
	assert.Contains(t, availableTools, "tool_b")
}

func TestBuildToolSafetyFallbackMessage(t *testing.T) {
	msg := buildToolSafetyFallbackMessage("tester")

	assert.Equal(t, Assistant, msg.Role)
	assert.Equal(t, "tester", msg.Name)
	assert.NotEmpty(t, msg.Content)
}

func TestGetModelMessageIncludesWhitelistedTools(t *testing.T) {
	cs := New("test-key", &guardTestModel{}, Options{})
	cs.Add(&guardTestIntent{code: "z_tool"})
	cs.Add(&guardTestIntent{code: "a_tool"})

	messages := cs.getModelMessage()

	found := false
	for _, msg := range messages {
		if msg.Role == System && strings.Contains(msg.Content, "Daftar tool yang tersedia:") {
			found = true
			assert.Contains(t, msg.Content, "a_tool, z_tool")
			break
		}
	}

	assert.True(t, found, "expected tool whitelist instruction in system messages")
}
