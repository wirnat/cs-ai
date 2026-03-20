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
