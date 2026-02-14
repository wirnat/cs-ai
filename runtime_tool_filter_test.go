package cs_ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type runtimeIntentStub struct {
	code      string
	callCount int
}

func (i *runtimeIntentStub) Code() string { return i.code }
func (i *runtimeIntentStub) Description() []string {
	return []string{"runtime intent stub"}
}
func (i *runtimeIntentStub) Param() interface{} {
	return struct{}{}
}
func (i *runtimeIntentStub) Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) {
	i.callCount++
	return map[string]interface{}{"status": "SUCCESS"}, nil
}

func TestExecWithToolCodes_RestrictsToolsInRequest(t *testing.T) {
	var mu sync.Mutex
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		mu.Lock()
		defer mu.Unlock()

		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
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

	intentA := &runtimeIntentStub{code: "tool-a"}
	intentB := &runtimeIntentStub{code: "tool-b"}
	cs.Add(intentA)
	cs.Add(intentB)

	_, err := cs.ExecWithToolCodes(context.Background(), "runtime-tools-1", UserMessage{
		Message:         "halo",
		ParticipantName: "tester",
	}, []string{"tool-a"})
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, capturedBody)

	tools, ok := capturedBody["tools"].([]interface{})
	require.True(t, ok)
	require.Len(t, tools, 1)

	toolItem, ok := tools[0].(map[string]interface{})
	require.True(t, ok)
	fn, ok := toolItem["function"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "tool-a", fn["name"])
}

func TestExecWithToolCodes_EmptyAllowedDisablesTools(t *testing.T) {
	var mu sync.Mutex
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		mu.Lock()
		defer mu.Unlock()

		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
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

	_, err := cs.ExecWithToolCodes(context.Background(), "runtime-tools-2", UserMessage{
		Message:         "halo",
		ParticipantName: "tester",
	}, []string{})
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, capturedBody)
	require.Equal(t, "none", capturedBody["tool_choice"])
	require.Nil(t, capturedBody["tools"])
}

func TestExecWithToolCodes_BootstrapSkipsUnavailableRuntimeIntent(t *testing.T) {
	server, _ := newBootstrapServer(t)
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &bootstrapModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.FirstTurnBootstrap = newBootstrapConfig(
		"tenant-outlet-info",
		"service-catalog",
	)

	intent1 := &bootstrapIntentStub{code: "tenant-outlet-info"}
	intent2 := &bootstrapIntentStub{code: "service-catalog"}
	cs.Add(intent1)
	cs.Add(intent2)

	_, err := cs.ExecWithToolCodes(context.Background(), "runtime-bootstrap-1", UserMessage{
		Message:         "halo",
		ParticipantName: "tester",
	}, []string{"tenant-outlet-info"})
	require.NoError(t, err)

	require.Equal(t, 1, intent1.callCount)
	require.Equal(t, 0, intent2.callCount)

	messages, err := cs.GetSessionMessages("runtime-bootstrap-1")
	require.NoError(t, err)
	require.Len(t, messages, 4)
	require.Equal(t, Assistant, messages[0].Role)
	require.Len(t, messages[0].ToolCalls, 1)
	require.Equal(t, "tenant-outlet-info", messages[0].ToolCalls[0].Function.Name)
}

func TestExecWithToolCodesAndIntents_IncludesAdditionalRuntimeIntents(t *testing.T) {
	var mu sync.Mutex
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		mu.Lock()
		defer mu.Unlock()

		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
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

	baseIntent := &runtimeIntentStub{code: "tool-a"}
	cs.Add(baseIntent)
	additionalIntent := &runtimeIntentStub{code: "custom-tool-1"}

	_, err := cs.ExecWithToolCodesAndIntents(
		context.Background(),
		"runtime-tools-extra-1",
		UserMessage{
			Message:         "halo",
			ParticipantName: "tester",
		},
		[]string{"tool-a"},
		[]Intent{additionalIntent},
	)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, capturedBody)

	tools, ok := capturedBody["tools"].([]interface{})
	require.True(t, ok)
	require.Len(t, tools, 2)

	gotNames := make([]string, 0, len(tools))
	for _, rawTool := range tools {
		toolItem, castOK := rawTool.(map[string]interface{})
		require.True(t, castOK)
		fn, castFnOK := toolItem["function"].(map[string]interface{})
		require.True(t, castFnOK)
		gotNames = append(gotNames, fn["name"].(string))
	}
	require.ElementsMatch(t, []string{"tool-a", "custom-tool-1"}, gotNames)
}
