package cs_ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type codexStreamModeTestModel struct {
	apiURL string
}

func (m *codexStreamModeTestModel) ModelName() string    { return "gpt-5.2" }
func (m *codexStreamModeTestModel) ApiURL() string       { return m.apiURL }
func (m *codexStreamModeTestModel) Train() []string      { return []string{"test"} }
func (m *codexStreamModeTestModel) ProviderName() string { return "openai-codex" }
func (m *codexStreamModeTestModel) APIMode() string      { return APIModeOpenAICodexResponses }

func TestSendWithModelReasoning_CodexAlwaysUsesStreamTrueEvenCompletionStage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := map[string]interface{}{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		stream, ok := req["stream"].(bool)
		if !ok || !stream {
			t.Fatalf("expected stream=true for codex responses, got %#v", req["stream"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "resp_stream_mode_test",
			"model": "gpt-5.2",
			"output": []map[string]interface{}{
				{
					"type": "message",
					"role": "assistant",
					"content": []map[string]interface{}{
						{"type": "output_text", "text": "ok"},
					},
				},
			},
		})
	}))
	defer server.Close()

	cs := New("test-key", &codexStreamModeTestModel{apiURL: server.URL}, Options{
		UseTool: true,
		Streaming: &StreamingOptions{
			Enabled: true,
		},
	})

	ctx := withStageStreaming(context.Background(), AgentStageSummary, StageStreamingConfig{
		Mode:          StageModeCompletion,
		EmitTextDelta: false,
		EmitProgress:  true,
	})

	msg, err := cs.sendWithModelReasoning(ctx, "", cs.Model, []map[string]interface{}{
		{"role": "system", "content": "sys"},
		{"role": "user", "content": "halo"},
	}, nil, nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Content != "ok" {
		t.Fatalf("unexpected response content: %q", msg.Content)
	}
}
