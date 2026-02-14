package cs_ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessageFromMap_ParsesDeepSeekUsage(t *testing.T) {
	raw := map[string]interface{}{
		"model": "deepseek-chat",
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "ok",
				},
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(100),
			"completion_tokens": float64(10),
			"total_tokens":      float64(110),
			"prompt_tokens_details": map[string]interface{}{
				"cached_tokens": float64(25),
			},
			"prompt_cache_hit_tokens":  float64(25),
			"prompt_cache_miss_tokens": float64(75),
		},
	}

	msg, err := MessageFromMap(raw)
	require.NoError(t, err)
	require.Equal(t, "deepseek-chat", msg.Model)
	require.NotNil(t, msg.Usage)
	require.EqualValues(t, 100, msg.Usage.PromptTokens)
	require.EqualValues(t, 10, msg.Usage.CompletionTokens)
	require.EqualValues(t, 110, msg.Usage.TotalTokens)
	require.EqualValues(t, 25, msg.Usage.PromptTokensDetails.CachedTokens)
	require.EqualValues(t, 25, msg.Usage.PromptCacheHitTokens)
	require.EqualValues(t, 75, msg.Usage.PromptCacheMissTokens)
}

func TestExec_AggregatesUsageAndDoesNotLeakUsageInRequestMessages(t *testing.T) {
	callCount := 0
	var secondRequest map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		callCount++

		var req map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		if callCount == 2 {
			secondRequest = req
		}

		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"model": "deepseek-chat",
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "bukan-json",
						},
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":            100,
					"completion_tokens":        20,
					"total_tokens":             120,
					"prompt_cache_hit_tokens":  30,
					"prompt_cache_miss_tokens": 70,
				},
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "deepseek-chat",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "{\"status\":\"ok\"}",
					},
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     200,
				"completion_tokens": 10,
				"total_tokens":      210,
				"prompt_tokens_details": map[string]interface{}{
					"cached_tokens": 40,
				},
				"prompt_cache_hit_tokens": 40,
			},
		})
	}))
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &bootstrapModel{apiURL: server.URL}
	cs.options.ResponseType = ResponseTypeJSON

	resp, err := cs.Exec(context.Background(), "usage-agg-session", UserMessage{
		Message:         "halo",
		ParticipantName: "tester",
	})
	require.NoError(t, err)
	require.Equal(t, 2, callCount)
	require.NotNil(t, resp.Usage)
	require.NotNil(t, resp.AggregatedUsage)

	require.EqualValues(t, 200, resp.Usage.PromptTokens)
	require.EqualValues(t, 10, resp.Usage.CompletionTokens)
	require.EqualValues(t, 210, resp.Usage.TotalTokens)
	require.EqualValues(t, 40, resp.Usage.PromptCacheHitTokens)
	require.EqualValues(t, 160, resp.Usage.PromptCacheMissTokens)

	require.EqualValues(t, 300, resp.AggregatedUsage.PromptTokens)
	require.EqualValues(t, 30, resp.AggregatedUsage.CompletionTokens)
	require.EqualValues(t, 330, resp.AggregatedUsage.TotalTokens)
	require.EqualValues(t, 70, resp.AggregatedUsage.PromptCacheHitTokens)
	require.EqualValues(t, 230, resp.AggregatedUsage.PromptCacheMissTokens)

	require.NotNil(t, secondRequest)
	reqMessages, ok := secondRequest["messages"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, reqMessages)

	hasUsageLeak := false
	for _, rawMsg := range reqMessages {
		msgMap, castOK := rawMsg.(map[string]interface{})
		if !castOK {
			continue
		}
		if _, exists := msgMap["usage"]; exists {
			hasUsageLeak = true
		}
		if _, exists := msgMap["aggregated_usage"]; exists {
			hasUsageLeak = true
		}
	}
	require.False(t, hasUsageLeak, "usage field should not be sent back to API messages")
}
