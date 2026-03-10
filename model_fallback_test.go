package cs_ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type fallbackTestModel struct {
	name     string
	apiURL   string
	provider string
	apiMode  string
}

func (m *fallbackTestModel) ModelName() string    { return m.name }
func (m *fallbackTestModel) ApiURL() string       { return m.apiURL }
func (m *fallbackTestModel) Train() []string      { return []string{"test"} }
func (m *fallbackTestModel) ProviderName() string { return m.provider }
func (m *fallbackTestModel) APIMode() string      { return m.apiMode }

func TestSend_FallbackToSecondaryModelWhenPrimaryRateLimited(t *testing.T) {
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "rate limit"},
		})
	}))
	defer primaryServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "fallback-ok",
					},
				},
			},
			"model": "deepseek-chat",
		})
	}))
	defer fallbackServer.Close()

	primaryModel := &fallbackTestModel{
		name:     "gpt-5.4",
		apiURL:   primaryServer.URL,
		provider: "openai-codex",
		apiMode:  APIModeOpenAICodexResponses,
	}
	secondaryModel := &fallbackTestModel{
		name:     "deepseek-chat",
		apiURL:   fallbackServer.URL,
		provider: "deepseek",
		apiMode:  APIModeChatCompletions,
	}

	cs := New("test-token", primaryModel, Options{
		UseTool:        false,
		ModelFallbacks: []Modeler{secondaryModel},
	})

	resp, err := cs.Send(Messages{{Role: User, Content: "Halo"}})
	require.NoError(t, err)
	require.Equal(t, "fallback-ok", resp.Content)
	require.Equal(t, "deepseek-chat", resp.Model)
}
