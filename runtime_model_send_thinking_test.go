package cs_ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildRequestBodyByAPIMode_ChatCompletions_AttachesAnthropicThinking(t *testing.T) {
	body := buildRequestBodyByAPIMode(
		APIModeChatCompletions,
		"omniroute",
		"kr/claude-sonnet-4.5",
		[]map[string]interface{}{
			{"role": "user", "content": "halo"},
		},
		nil,
		Options{},
		requestBuildConfig{
			Reasoning: &ReasoningConfig{
				Effort: ReasoningEffortMedium,
			},
		},
	)

	thinking, ok := body["thinking"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected thinking payload, got %#v", body["thinking"])
	}
	if toString(thinking["type"]) != "enabled" {
		t.Fatalf("unexpected thinking type: %#v", thinking["type"])
	}
	if budget, ok := thinking["budget_tokens"].(int); !ok || budget <= 0 {
		t.Fatalf("unexpected thinking budget: %#v", thinking["budget_tokens"])
	}
}

func TestSendWithModel_DisablesThinkingWhenProviderRejectsThinkingParameter(t *testing.T) {
	reqCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		defer r.Body.Close()
		body := map[string]interface{}{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if reqCount == 1 {
			if _, exists := body["thinking"]; !exists {
				t.Fatalf("expected first request includes thinking payload")
			}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unknown parameter: thinking"}`))
			return
		}
		if _, exists := body["thinking"]; exists {
			t.Fatalf("expected retry request without thinking payload")
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &fallbackTestModel{
		name:     "kr/claude-sonnet-4.5",
		apiURL:   server.URL,
		provider: "omniroute",
		apiMode:  APIModeChatCompletions,
	}
	cs.options.Reasoning = &ReasoningConfig{
		Effort: ReasoningEffortMedium,
	}

	msg, err := cs.sendWithModel(
		withStageStreaming(context.Background(), AgentStageAnswer, StageStreamingConfig{Mode: StageModeStream}),
		"thinking-fallback",
		cs.Model,
		[]map[string]interface{}{{"role": "user", "content": "cek"}},
		nil,
	)
	if err != nil {
		t.Fatalf("expected retry success, got error: %v", err)
	}
	if msg.Content != "ok" {
		t.Fatalf("unexpected message content: %q", msg.Content)
	}
	if reqCount != 2 {
		t.Fatalf("expected exactly 2 requests, got %d", reqCount)
	}
}
