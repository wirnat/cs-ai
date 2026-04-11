package cs_ai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessageFromResponsesMap_PrioritizesAssistantMessageOverTopLevelOutputText(t *testing.T) {
	raw := map[string]interface{}{
		"model":       "gpt-5.4",
		"output_text": "thinking internal\njawaban fallback",
		"output": []interface{}{
			map[string]interface{}{
				"type":    "reasoning",
				"summary": []interface{}{},
			},
			map[string]interface{}{
				"type": "message",
				"role": "assistant",
				"content": []interface{}{
					map[string]interface{}{
						"type": "output_text",
						"text": "Ini jawaban final assistant",
					},
				},
			},
		},
	}

	msg, err := MessageFromResponsesMap(raw)
	require.NoError(t, err)
	require.Equal(t, Assistant, msg.Role)
	require.Equal(t, "Ini jawaban final assistant", msg.Content)
}

func TestMessageFromResponsesMap_FallsBackToOutputTextWhenNoAssistantMessage(t *testing.T) {
	raw := map[string]interface{}{
		"model":       "gpt-5.4",
		"output_text": "Jawaban final dari output_text",
		"output": []interface{}{
			map[string]interface{}{
				"type": "reasoning",
			},
		},
	}

	msg, err := MessageFromResponsesMap(raw)
	require.NoError(t, err)
	require.Equal(t, "Jawaban final dari output_text", msg.Content)
}

func TestMessageFromResponsesMap_StripsThinkTagFromAssistantContent(t *testing.T) {
	raw := map[string]interface{}{
		"model": "gpt-5.4",
		"output": []interface{}{
			map[string]interface{}{
				"type": "message",
				"role": "assistant",
				"content": []interface{}{
					map[string]interface{}{
						"type": "output_text",
						"text": "<think>analisa internal</think>\nJawaban final aman",
					},
				},
			},
		},
	}

	msg, err := MessageFromResponsesMap(raw)
	require.NoError(t, err)
	require.Equal(t, "Jawaban final aman", msg.Content)
}

func TestMessageFromResponsesMap_ParsesReasoningMetadata(t *testing.T) {
	raw := map[string]interface{}{
		"id":                   "resp_123",
		"model":                "gpt-5.4",
		"previous_response_id": "resp_prev",
		"usage": map[string]interface{}{
			"input_tokens":  100,
			"output_tokens": 20,
			"total_tokens":  120,
			"input_tokens_details": map[string]interface{}{
				"cached_tokens": 15,
			},
			"output_tokens_details": map[string]interface{}{
				"reasoning_tokens": 9,
			},
		},
		"output": []interface{}{
			map[string]interface{}{
				"type":   "reasoning",
				"effort": "medium",
				"summary": []interface{}{
					map[string]interface{}{"text": "Langkah validasi tool dipilih dulu."},
				},
			},
			map[string]interface{}{
				"type": "message",
				"role": "assistant",
				"content": []interface{}{
					map[string]interface{}{
						"type": "output_text",
						"text": "Jawaban final assistant",
					},
				},
			},
		},
	}

	msg, err := MessageFromResponsesMap(raw)
	require.NoError(t, err)
	require.Equal(t, "resp_123", msg.ResponseID)
	require.NotNil(t, msg.Reasoning)
	require.Equal(t, "medium", msg.Reasoning.EffortUsed)
	require.Equal(t, "resp_prev", msg.Reasoning.PreviousResponseID)
	require.Equal(t, "Langkah validasi tool dipilih dulu.", msg.Reasoning.SummaryText)
	require.True(t, msg.Reasoning.ItemsPresent)
	require.EqualValues(t, 9, msg.Reasoning.Usage.Tokens)
	require.NotNil(t, msg.Usage)
	require.EqualValues(t, 15, msg.Usage.Reasoning.CachedContextTokens)
}

func TestMessageFromMap_StripsThinkTagFromAssistantContent(t *testing.T) {
	raw := map[string]interface{}{
		"model": "deepseek-chat",
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "<think>analisa internal</think>\nFinal answer",
				},
			},
		},
	}

	msg, err := MessageFromMap(raw)
	require.NoError(t, err)
	require.Equal(t, "Final answer", msg.Content)
}
