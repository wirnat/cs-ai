package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type bootstrapModel struct {
	apiURL string
}

func (m *bootstrapModel) ModelName() string { return "bootstrap-test-model" }
func (m *bootstrapModel) ApiURL() string    { return m.apiURL }
func (m *bootstrapModel) Train() []string   { return []string{"bootstrap test"} }

type bootstrapIntentStub struct {
	code      string
	payload   interface{}
	handleErr error
	callCount int
}

func (i *bootstrapIntentStub) Code() string {
	return i.code
}

func (i *bootstrapIntentStub) Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) {
	i.callCount++
	if i.handleErr != nil {
		return nil, i.handleErr
	}
	if i.payload != nil {
		return i.payload, nil
	}
	return map[string]interface{}{
		"status":  "SUCCESS",
		"message": fmt.Sprintf("intent %s ok", i.code),
	}, nil
}

func (i *bootstrapIntentStub) Description() []string {
	return []string{"bootstrap intent stub"}
}

func (i *bootstrapIntentStub) Param() interface{} {
	return struct{}{}
}

func newBootstrapServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "siap kak",
					},
				},
			},
		})
	}))
	return server, &callCount
}

func newBootstrapConfig(intentCodes ...string) *FirstTurnBootstrapOptions {
	calls := make([]BootstrapIntentCall, 0, len(intentCodes))
	for _, code := range intentCodes {
		calls = append(calls, BootstrapIntentCall{
			IntentCode: code,
			Params:     map[string]interface{}{},
		})
	}

	return &FirstTurnBootstrapOptions{
		IntentCalls:      calls,
		FailurePolicy:    BootstrapFailureBestEffort,
		ToolCallIDPrefix: "bootstrap-fc",
		RequireSessionID: true,
		Compaction: BootstrapCompactionOptions{
			MaxDepth:        4,
			MaxArrayItems:   20,
			MaxObjectKeys:   30,
			MaxStringChars:  280,
			MaxPayloadBytes: 12000,
		},
	}
}

func TestExec_FirstTurnBootstrap_InsertsAssistantToolSequenceBeforeFirstUser(t *testing.T) {
	server, _ := newBootstrapServer(t)
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &bootstrapModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.FirstTurnBootstrap = newBootstrapConfig(
		"tenant-outlet-info",
		"service-catalog",
		"provider-schedule",
	)

	intent1 := &bootstrapIntentStub{code: "tenant-outlet-info"}
	intent2 := &bootstrapIntentStub{code: "service-catalog"}
	intent3 := &bootstrapIntentStub{code: "provider-schedule"}
	cs.Add(intent1)
	cs.Add(intent2)
	cs.Add(intent3)

	_, err := cs.Exec(context.Background(), "session-1", UserMessage{
		Message:         "halo",
		ParticipantName: "tester",
	})
	require.NoError(t, err)

	messages, err := cs.GetSessionMessages("session-1")
	require.NoError(t, err)
	require.Len(t, messages, 6)
	require.Equal(t, Assistant, messages[0].Role)
	require.Len(t, messages[0].ToolCalls, 3)
	require.Equal(t, Tool, messages[1].Role)
	require.Equal(t, Tool, messages[2].Role)
	require.Equal(t, Tool, messages[3].Role)
	require.Equal(t, User, messages[4].Role)
	require.Equal(t, Assistant, messages[5].Role)

	require.Equal(t, "tenant-outlet-info", messages[0].ToolCalls[0].Function.Name)
	require.Equal(t, "service-catalog", messages[0].ToolCalls[1].Function.Name)
	require.Equal(t, "provider-schedule", messages[0].ToolCalls[2].Function.Name)

	require.Equal(t, messages[0].ToolCalls[0].Id, messages[1].ToolCallID)
	require.Equal(t, messages[0].ToolCalls[1].Id, messages[2].ToolCallID)
	require.Equal(t, messages[0].ToolCalls[2].Id, messages[3].ToolCallID)
}

func TestExec_FirstTurnBootstrap_RunsOnlyOncePerSession(t *testing.T) {
	server, _ := newBootstrapServer(t)
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &bootstrapModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.FirstTurnBootstrap = newBootstrapConfig(
		"tenant-outlet-info",
		"service-catalog",
		"provider-schedule",
	)

	intent1 := &bootstrapIntentStub{code: "tenant-outlet-info"}
	intent2 := &bootstrapIntentStub{code: "service-catalog"}
	intent3 := &bootstrapIntentStub{code: "provider-schedule"}
	cs.Add(intent1)
	cs.Add(intent2)
	cs.Add(intent3)

	_, err := cs.Exec(context.Background(), "session-2", UserMessage{
		Message:         "halo",
		ParticipantName: "tester",
	})
	require.NoError(t, err)

	_, err = cs.Exec(context.Background(), "session-2", UserMessage{
		Message:         "lanjut",
		ParticipantName: "tester",
	})
	require.NoError(t, err)

	require.Equal(t, 1, intent1.callCount)
	require.Equal(t, 1, intent2.callCount)
	require.Equal(t, 1, intent3.callCount)

	messages, err := cs.GetSessionMessages("session-2")
	require.NoError(t, err)
	withToolCalls := 0
	for _, msg := range messages {
		if msg.Role == Assistant && len(msg.ToolCalls) > 0 {
			withToolCalls++
		}
	}
	require.Equal(t, 1, withToolCalls)
}

func TestExec_FirstTurnBootstrap_BestEffort_PreservesCompleteToolResponses(t *testing.T) {
	server, _ := newBootstrapServer(t)
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &bootstrapModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.FirstTurnBootstrap = &FirstTurnBootstrapOptions{
		IntentCalls: []BootstrapIntentCall{
			{IntentCode: "tenant-outlet-info", Params: map[string]interface{}{}},
			{IntentCode: "service-catalog", Params: map[string]interface{}{}},
			{IntentCode: "provider-schedule", Params: map[string]interface{}{}},
		},
		FailurePolicy:    BootstrapFailureBestEffort,
		ToolCallIDPrefix: "bootstrap-fc",
		RequireSessionID: true,
	}

	intent1 := &bootstrapIntentStub{code: "tenant-outlet-info"}
	intent2 := &bootstrapIntentStub{code: "service-catalog", handleErr: fmt.Errorf("forced error")}
	intent3 := &bootstrapIntentStub{code: "provider-schedule"}
	cs.Add(intent1)
	cs.Add(intent2)
	cs.Add(intent3)

	_, err := cs.Exec(context.Background(), "session-3", UserMessage{
		Message:         "halo",
		ParticipantName: "tester",
	})
	require.NoError(t, err)

	messages, err := cs.GetSessionMessages("session-3")
	require.NoError(t, err)
	require.Len(t, messages, 6)

	var payload2 map[string]interface{}
	var payload3 map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[2].Content), &payload2))
	require.NoError(t, json.Unmarshal([]byte(messages[3].Content), &payload3))

	require.Equal(t, "ERROR", payload2["status"])
	require.NotNil(t, payload2["error"])
	require.Equal(t, "SUCCESS", payload3["status"])
}

func TestExec_FirstTurnBootstrap_StrictMode_NoPartialMutation(t *testing.T) {
	server, _ := newBootstrapServer(t)
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &bootstrapModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.FirstTurnBootstrap = &FirstTurnBootstrapOptions{
		IntentCalls: []BootstrapIntentCall{
			{IntentCode: "tenant-outlet-info", Params: map[string]interface{}{}},
			{IntentCode: "service-catalog", Params: map[string]interface{}{}},
			{IntentCode: "provider-schedule", Params: map[string]interface{}{}},
		},
		FailurePolicy:    BootstrapFailureStrict,
		ToolCallIDPrefix: "bootstrap-fc",
		RequireSessionID: true,
	}

	intent1 := &bootstrapIntentStub{code: "tenant-outlet-info"}
	intent2 := &bootstrapIntentStub{code: "service-catalog", handleErr: fmt.Errorf("forced error")}
	intent3 := &bootstrapIntentStub{code: "provider-schedule"}
	cs.Add(intent1)
	cs.Add(intent2)
	cs.Add(intent3)

	_, err := cs.Exec(context.Background(), "session-4", UserMessage{
		Message:         "halo",
		ParticipantName: "tester",
	})
	require.Error(t, err)

	messages, getErr := cs.GetSessionMessages("session-4")
	require.NoError(t, getErr)
	require.Len(t, messages, 0)
}

func TestExec_FirstTurnBootstrap_SkipWhenSessionIDEmpty(t *testing.T) {
	server, _ := newBootstrapServer(t)
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &bootstrapModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.FirstTurnBootstrap = newBootstrapConfig(
		"tenant-outlet-info",
		"service-catalog",
		"provider-schedule",
	)

	intent1 := &bootstrapIntentStub{code: "tenant-outlet-info"}
	intent2 := &bootstrapIntentStub{code: "service-catalog"}
	intent3 := &bootstrapIntentStub{code: "provider-schedule"}
	cs.Add(intent1)
	cs.Add(intent2)
	cs.Add(intent3)

	_, err := cs.Exec(context.Background(), "", UserMessage{
		Message:         "halo",
		ParticipantName: "tester",
	})
	require.NoError(t, err)

	require.Equal(t, 0, intent1.callCount)
	require.Equal(t, 0, intent2.callCount)
	require.Equal(t, 0, intent3.callCount)

	messages, getErr := cs.GetSessionMessages("")
	require.NoError(t, getErr)
	withToolCalls := 0
	for _, msg := range messages {
		if msg.Role == Assistant && len(msg.ToolCalls) > 0 {
			withToolCalls++
		}
	}
	require.Equal(t, 0, withToolCalls)
}

func TestExec_FirstTurnBootstrap_CompactsLargePayload(t *testing.T) {
	server, _ := newBootstrapServer(t)
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &bootstrapModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.FirstTurnBootstrap = &FirstTurnBootstrapOptions{
		IntentCalls: []BootstrapIntentCall{
			{IntentCode: "service-catalog", Params: map[string]interface{}{}},
		},
		FailurePolicy:    BootstrapFailureBestEffort,
		ToolCallIDPrefix: "bootstrap-fc",
		RequireSessionID: true,
		Compaction: BootstrapCompactionOptions{
			MaxDepth:        4,
			MaxArrayItems:   2,
			MaxObjectKeys:   3,
			MaxStringChars:  20,
			MaxPayloadBytes: 500,
		},
	}

	intent := &bootstrapIntentStub{
		code: "service-catalog",
		payload: map[string]interface{}{
			"message": "ok",
			"data": map[string]interface{}{
				"long_text": strings.Repeat("x", 120),
				"items":     []interface{}{1, 2, 3, 4, 5},
				"extra":     "ignored",
			},
		},
	}
	cs.Add(intent)

	_, err := cs.Exec(context.Background(), "session-6", UserMessage{
		Message:         "halo",
		ParticipantName: "tester",
	})
	require.NoError(t, err)

	messages, getErr := cs.GetSessionMessages("session-6")
	require.NoError(t, getErr)
	require.Len(t, messages, 4)

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(messages[1].Content), &payload))
	require.Equal(t, "SUCCESS", payload["status"])

	data, ok := payload["data"].(map[string]interface{})
	require.True(t, ok)
	compactedData, ok := data["data"].(map[string]interface{})
	require.True(t, ok)
	longText, ok := compactedData["long_text"].(string)
	require.True(t, ok)
	require.LessOrEqual(t, len(longText), 20)
	items, ok := compactedData["items"].([]interface{})
	require.True(t, ok)
	require.Len(t, items, 3)
}

func TestExec_FirstTurnBootstrap_SkipWhenUserHistoryExists(t *testing.T) {
	server, _ := newBootstrapServer(t)
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &bootstrapModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.FirstTurnBootstrap = newBootstrapConfig(
		"tenant-outlet-info",
		"service-catalog",
		"provider-schedule",
	)

	intent1 := &bootstrapIntentStub{code: "tenant-outlet-info"}
	intent2 := &bootstrapIntentStub{code: "service-catalog"}
	intent3 := &bootstrapIntentStub{code: "provider-schedule"}
	cs.Add(intent1)
	cs.Add(intent2)
	cs.Add(intent3)

	_, err := cs.SaveSessionMessages("session-7", []Message{
		{Role: User, Content: "existing user message"},
	})
	require.NoError(t, err)

	_, err = cs.Exec(context.Background(), "session-7", UserMessage{
		Message:         "halo lagi",
		ParticipantName: "tester",
	})
	require.NoError(t, err)

	require.Equal(t, 0, intent1.callCount)
	require.Equal(t, 0, intent2.callCount)
	require.Equal(t, 0, intent3.callCount)

	messages, getErr := cs.GetSessionMessages("session-7")
	require.NoError(t, getErr)
	withToolCalls := 0
	for _, msg := range messages {
		if msg.Role == Assistant && len(msg.ToolCalls) > 0 {
			withToolCalls++
		}
	}
	require.Equal(t, 0, withToolCalls)
}
