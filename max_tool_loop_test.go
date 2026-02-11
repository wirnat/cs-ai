package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type loopingToolCallModel struct {
	apiURL string
}

func (m *loopingToolCallModel) ModelName() string { return "looping-tool-model" }
func (m *loopingToolCallModel) ApiURL() string    { return m.apiURL }
func (m *loopingToolCallModel) Train() []string   { return []string{"test"} }

type loopingToolIntent struct{}

func (i *loopingToolIntent) Code() string { return "loop_tool" }

func (i *loopingToolIntent) Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func (i *loopingToolIntent) Description() []string { return []string{"loop tool test intent"} }
func (i *loopingToolIntent) Param() interface{}    { return nil }

func TestExec_ReturnsErrorWhenToolCallExceedsMaxLoop(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]interface{}{
							{
								"index": 0,
								"id":    fmt.Sprintf("call-%d", callCount),
								"type":  "function",
								"function": map[string]interface{}{
									"name":      "loop_tool",
									"arguments": `{"count":1}`,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &loopingToolCallModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.Add(&loopingToolIntent{})

	resp, err := cs.Exec(context.Background(), "max-loop-session", UserMessage{
		Message:         "please run the loop tool",
		ParticipantName: "tester",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "max tool-call loop reached (10)")
	assert.Equal(t, Message{}, resp)
	assert.Equal(t, 11, callCount)
}
