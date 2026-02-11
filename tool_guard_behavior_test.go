package cs_ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type guardBehaviorModel struct {
	apiURL string
}

func (m *guardBehaviorModel) ModelName() string { return "guard-behavior-model" }
func (m *guardBehaviorModel) ApiURL() string    { return m.apiURL }
func (m *guardBehaviorModel) Train() []string   { return []string{"test"} }

type boolCoercionIntent struct {
	seenConfirm interface{}
}

func (i *boolCoercionIntent) Code() string { return "booking-change" }
func (i *boolCoercionIntent) Description() []string {
	return []string{"test bool coercion on confirm field"}
}
func (i *boolCoercionIntent) Param() interface{} {
	return struct {
		Confirm bool `json:"confirm"`
	}{}
}
func (i *boolCoercionIntent) Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) {
	i.seenConfirm = req["confirm"]
	return map[string]interface{}{"ok": true}, nil
}

type confirmationRequiredIntent struct{}

func (i *confirmationRequiredIntent) Code() string { return "booking-change" }
func (i *confirmationRequiredIntent) Description() []string {
	return []string{"test confirmation required status handling"}
}
func (i *confirmationRequiredIntent) Param() interface{} { return nil }
func (i *confirmationRequiredIntent) Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"status":  "CONFIRMATION_REQUIRED",
		"message": "Mau aku ubah booking jadi besok jam 17:00 ya kak?",
	}, nil
}

func TestExec_CoercesBooleanToolArgument(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "",
							"tool_calls": []map[string]interface{}{
								{
									"index": 0,
									"id":    "call-1",
									"type":  "function",
									"function": map[string]interface{}{
										"name":      "booking-change",
										"arguments": `{"confirm":"true"}`,
									},
								},
							},
						},
					},
				},
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Selesai kak.",
					},
				},
			},
		})
	}))
	defer server.Close()

	intent := &boolCoercionIntent{}
	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &guardBehaviorModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.Add(intent)

	_, err := cs.Exec(context.Background(), "coerce-bool-session", UserMessage{
		Message:         "tolong proses",
		ParticipantName: "tester",
	})

	require.NoError(t, err)
	require.Equal(t, 2, callCount)
	assert.IsType(t, true, intent.seenConfirm)
	assert.Equal(t, true, intent.seenConfirm)
}

func TestExec_UsesToolConfirmationMessageWhenStatusStillPending(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "",
							"tool_calls": []map[string]interface{}{
								{
									"index": 0,
									"id":    "call-1",
									"type":  "function",
									"function": map[string]interface{}{
										"name":      "booking-change",
										"arguments": `{}`,
									},
								},
							},
						},
					},
				},
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Oke kak, booking sudah berhasil direschedule.",
					},
				},
			},
		})
	}))
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &guardBehaviorModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.Add(&confirmationRequiredIntent{})

	resp, err := cs.Exec(context.Background(), "confirmation-guard-session", UserMessage{
		Message:         "reschedule ya",
		ParticipantName: "tester",
	})

	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
	assert.Equal(t, Assistant, resp.Role)
	assert.Equal(t, "Mau aku ubah booking jadi besok jam 17:00 ya kak?", resp.Content)
}
