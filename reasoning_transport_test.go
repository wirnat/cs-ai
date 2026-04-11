package cs_ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type reasoningTestModel struct {
	apiURL string
}

func (m *reasoningTestModel) ModelName() string    { return "gpt-5.4" }
func (m *reasoningTestModel) ApiURL() string       { return m.apiURL }
func (m *reasoningTestModel) Train() []string      { return []string{"reasoning test"} }
func (m *reasoningTestModel) ProviderName() string { return "openai-codex" }
func (m *reasoningTestModel) APIMode() string      { return APIModeOpenAICodexResponses }

func TestSendWithModelReasoning_FallsBackWhenProviderRejectsReasoning(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		callCount++

		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed decode request: %v", err)
		}

		_, hasReasoning := req["reasoning"]
		if callCount == 1 && hasReasoning {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "unknown parameter: reasoning",
			})
			return
		}

		if _, exists := req["reasoning"]; exists {
			t.Fatalf("expected retry request without reasoning payload, got %#v", req["reasoning"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "resp_after_retry",
			"model": "gpt-5.4",
			"output": []map[string]interface{}{
				{
					"type": "message",
					"role": "assistant",
					"content": []map[string]interface{}{
						{
							"type": "output_text",
							"text": "hasil aman",
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &reasoningTestModel{apiURL: server.URL}
	cs.options.Reasoning = &ReasoningConfig{
		Effort:  ReasoningEffortMedium,
		Summary: ReasoningSummaryConcise,
	}

	msg, err := cs.sendWithModel(context.Background(), "reasoning-session", cs.Model, []map[string]interface{}{
		{"role": "system", "content": "sys"},
		{"role": "user", "content": "halo"},
	}, nil)
	if err != nil {
		t.Fatalf("sendWithModel returned error: %v", err)
	}
	if msg.Content != "hasil aman" {
		t.Fatalf("unexpected content: %q", msg.Content)
	}
	if msg.Reasoning == nil || msg.Reasoning.TransportWarning == "" {
		t.Fatalf("expected transport warning after fallback, got %#v", msg.Reasoning)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 requests, got %d", callCount)
	}

	state, err := cs.GetSessionState("reasoning-session")
	if err != nil {
		t.Fatalf("failed loading session state: %v", err)
	}
	internal, ok := state[reasoningRuntimeStateKey].(map[string]interface{})
	if !ok {
		t.Fatalf("expected reasoning runtime state, got %#v", state[reasoningRuntimeStateKey])
	}
	if internal["last_capability_warning"] == "" {
		t.Fatalf("expected persisted capability warning, got %#v", internal)
	}
}
