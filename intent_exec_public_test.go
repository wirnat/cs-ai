package cs_ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecuteIntent_ReturnsProcessedToolMessage(t *testing.T) {
	cs := newTestCsAIWithInMemoryStorage(t)
	intent := &runtimeIntentStub{code: "booking-history"}
	cs.Add(intent)

	result, err := cs.ExecuteIntent(context.Background(), "deterministic-1", UserMessage{
		Message:         "saya mau ubah booking",
		ParticipantName: "tester",
	}, "booking-history", map[string]interface{}{
		"filter_status": "open",
	}, IntentExecutionOptions{
		AllowedToolCodes: []string{"booking-history"},
	})
	require.NoError(t, err)
	require.Equal(t, "booking-history", result.ToolCode)
	require.Equal(t, "open", result.Parameters["filter_status"])
	require.Equal(t, Tool, result.ToolMessage.Role)
	require.Contains(t, result.ToolMessage.Content, "SUCCESS")
	require.Equal(t, 1, intent.callCount)
}

func TestExecuteIntent_PersistsToolMessageWhenRequested(t *testing.T) {
	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Add(&runtimeIntentStub{code: "booking-history"})

	_, err := cs.ExecuteIntent(context.Background(), "deterministic-2", UserMessage{
		Message:         "cek booking saya",
		ParticipantName: "tester",
	}, "booking-history", nil, IntentExecutionOptions{
		AllowedToolCodes:   []string{"booking-history"},
		PersistToolMessage: true,
	})
	require.NoError(t, err)

	messages, err := cs.GetSessionMessages("deterministic-2")
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, Tool, messages[0].Role)
	require.NotEmpty(t, messages[0].ToolCallID)
}

func TestExecuteIntent_HonorsAdditionalRuntimeIntents(t *testing.T) {
	cs := newTestCsAIWithInMemoryStorage(t)

	_, err := cs.ExecuteIntent(context.Background(), "deterministic-3", UserMessage{
		Message:         "jalankan tool custom",
		ParticipantName: "tester",
	}, "custom-tool", nil, IntentExecutionOptions{
		AllowedToolCodes: []string{},
		AdditionalIntents: []Intent{
			&runtimeIntentStub{code: "custom-tool"},
		},
	})
	require.NoError(t, err)
}
