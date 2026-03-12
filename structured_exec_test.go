package cs_ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecStructured_PersistsEnvelopeAndUsesSelector(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "ringkasan terstruktur",
					},
				},
			},
		})
	}))
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &bootstrapModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.Add(&runtimeIntentStub{code: "tool-a"})
	cs.Add(&runtimeIntentStub{code: "tool-b"})

	result, err := cs.ExecStructured(context.Background(), "structured-1", UserMessage{
		Message:         "halo",
		ParticipantName: "tester",
	}, StructuredExecOptions{
		ToolSelector: RuntimeToolSelectorFunc(func(ctx context.Context, sessionID string, userMessage UserMessage, availableIntents []Intent, opts StructuredExecOptions) ([]string, error) {
			return []string{"tool-b"}, nil
		}),
	})
	require.NoError(t, err)
	require.Equal(t, "ringkasan terstruktur", result.AssistantMessage)
	require.NotNil(t, result.Decision)
	require.Equal(t, []string{"tool-b"}, result.Decision.AllowedTools)

	require.NotNil(t, capturedBody)
	tools, ok := capturedBody["tools"].([]interface{})
	require.True(t, ok)
	require.Len(t, tools, 1)

	toolItem := tools[0].(map[string]interface{})
	functionItem := toolItem["function"].(map[string]interface{})
	require.Equal(t, "tool-b", functionItem["name"])

	messages, err := cs.GetSessionMessages("structured-1")
	require.NoError(t, err)
	require.Len(t, messages, 2)

	lastMessage := messages[len(messages)-1]
	require.Equal(t, Assistant, lastMessage.Role)
	require.Equal(t, "ringkasan terstruktur", lastMessage.Content)

	contentMap, ok := lastMessage.ContentMap.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "ringkasan terstruktur", contentMap["assistant_message"])
}

func TestExecStructured_PassesSessionMessagesToSelector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "ok",
					},
				},
			},
		})
	}))
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &bootstrapModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.Add(&runtimeIntentStub{code: "tool-a"})

	_, err := cs.SaveSessionMessages("structured-history", []Message{
		{Role: User, Content: "fitur Karna apa saja?"},
		{Role: Assistant, Content: "Saya bisa bantu jelaskan modul Karna."},
	})
	require.NoError(t, err)

	_, err = cs.ExecStructured(context.Background(), "structured-history", UserMessage{
		Message:         "yang inventory gimana?",
		ParticipantName: "tester",
	}, StructuredExecOptions{
		ToolSelector: RuntimeToolSelectorFunc(func(ctx context.Context, sessionID string, userMessage UserMessage, availableIntents []Intent, opts StructuredExecOptions) ([]string, error) {
			require.Len(t, opts.SessionMessages, 2)
			require.Equal(t, "fitur Karna apa saja?", opts.SessionMessages[0].Content)
			require.Equal(t, "Saya bisa bantu jelaskan modul Karna.", opts.SessionMessages[1].Content)
			return []string{"tool-a"}, nil
		}),
	})
	require.NoError(t, err)
}
