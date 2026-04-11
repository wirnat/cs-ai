package cs_ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type compactFollowUpModel struct {
	apiURL string
}

func (m *compactFollowUpModel) ModelName() string { return "compact-follow-up-model" }
func (m *compactFollowUpModel) ApiURL() string    { return m.apiURL }
func (m *compactFollowUpModel) Train() []string   { return []string{"test"} }

type fixedIdentifierAgent struct{}

func (a fixedIdentifierAgent) Identify(ctx context.Context, input IdentifierInput) (IdentifierOutput, error) {
	return IdentifierOutput{
		Route:            "run_tool_then_answer",
		NeedTool:         true,
		AllowedToolCodes: []string{"check-availability", "service-catalog"},
		ToolOrder:        []string{"check-availability", "service-catalog"},
		CanAnswerDirect:  false,
		Confidence:       1,
	}, nil
}

type passthroughSummaryAgent struct{}

func (a passthroughSummaryAgent) Summarize(ctx context.Context, input SummaryInput) (SummaryOutput, error) {
	return SummaryOutput{
		ConversationSummary: strings.TrimSpace(input.PreviousSummary),
	}, nil
}

type checkAvailabilityFollowUpIntent struct {
	callCount int
}

func (i *checkAvailabilityFollowUpIntent) Code() string { return "check-availability" }
func (i *checkAvailabilityFollowUpIntent) Description() []string {
	return []string{"test follow-up gate from check availability"}
}
func (i *checkAvailabilityFollowUpIntent) Param() interface{} { return struct{}{} }
func (i *checkAvailabilityFollowUpIntent) Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) {
	i.callCount++
	return map[string]interface{}{
		"status":      "ERROR",
		"message":     "service_uid 01KF3Q0HHP5MMFQKJ229BHVJ7G tidak ditemukan atau tidak aktif di outlet ini.",
		"instruction": "Panggil intent service-catalog terlebih dahulu untuk mencari service_uid yang valid, lalu ulangi check-availability.",
	}, nil
}

type serviceCatalogFollowUpIntent struct {
	callCount int
}

func (i *serviceCatalogFollowUpIntent) Code() string { return "service-catalog" }
func (i *serviceCatalogFollowUpIntent) Description() []string {
	return []string{"test service catalog follow-up"}
}
func (i *serviceCatalogFollowUpIntent) Param() interface{} { return struct{}{} }
func (i *serviceCatalogFollowUpIntent) Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) {
	i.callCount++
	return map[string]interface{}{
		"status": "SUCCESS",
		"services": []map[string]interface{}{
			{
				"uid":  "01KF3QMJJ9QSKENJR9FTR4BD9Y",
				"name": "Haircut",
			},
		},
	}, nil
}

func TestExecCompact_FollowUpGateForcesSecondToolCallBeforeFinalAnswer(t *testing.T) {
	var (
		mu       sync.Mutex
		call     int
		requests []map[string]interface{}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		body := map[string]interface{}{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		mu.Lock()
		call++
		currentCall := call
		requests = append(requests, body)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		switch currentCall {
		case 1:
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
										"name":      "check-availability",
										"arguments": `{"date":"2026-03-28","time":"09:00"}`,
									},
								},
							},
						},
					},
				},
			})
		case 2:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Maaf kak, saya tidak bisa akses jadwal.",
						},
					},
				},
			})
		case 3:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "",
							"tool_calls": []map[string]interface{}{
								{
									"index": 0,
									"id":    "call-2",
									"type":  "function",
									"function": map[string]interface{}{
										"name":      "service-catalog",
										"arguments": `{}`,
									},
								},
							},
						},
					},
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Siap kak, jam 09:00 tersedia.",
						},
					},
				},
			})
		}
	}))
	defer server.Close()

	checkAvailability := &checkAvailabilityFollowUpIntent{}
	serviceCatalog := &serviceCatalogFollowUpIntent{}

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &compactFollowUpModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.AgentRuntime = &AgentRuntimeOptions{
		Strategy:   ContextStrategyCompactBackend,
		Summary:    passthroughSummaryAgent{},
		Identifier: fixedIdentifierAgent{},
	}
	cs.Add(checkAvailability)
	cs.Add(serviceCatalog)

	resp, err := cs.Exec(context.Background(), "compact-follow-up-gate", UserMessage{
		Message:         "kalo jam 9 gmn",
		ParticipantName: "tester",
	})
	require.NoError(t, err)
	require.Equal(t, "Siap kak, jam 09:00 tersedia.", strings.TrimSpace(resp.Content))
	require.Equal(t, 1, checkAvailability.callCount)
	require.Equal(t, 1, serviceCatalog.callCount)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 4, call)
	require.Len(t, requests, 4)
	require.True(t, hasSystemMessageContaining(requests[2], "HARD GATE"), "expected third request to include follow-up hard gate instruction")
}

func hasSystemMessageContaining(body map[string]interface{}, token string) bool {
	rawMessages, ok := body["messages"].([]interface{})
	if !ok {
		return false
	}
	token = strings.ToLower(strings.TrimSpace(token))
	for _, raw := range rawMessages {
		msg, castOK := raw.(map[string]interface{})
		if !castOK {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(toString(msg["role"])))
		if role != "system" {
			continue
		}
		content := strings.ToLower(strings.TrimSpace(toString(msg["content"])))
		if strings.Contains(content, token) {
			return true
		}
	}
	return false
}

type checkAvailabilityNoProgressIntent struct {
	callCount int
}

func (i *checkAvailabilityNoProgressIntent) Code() string { return "check-availability" }
func (i *checkAvailabilityNoProgressIntent) Description() []string {
	return []string{"test no-progress guard for repeated availability checks"}
}
func (i *checkAvailabilityNoProgressIntent) Param() interface{} { return struct{}{} }
func (i *checkAvailabilityNoProgressIntent) Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) {
	i.callCount++
	return map[string]interface{}{
		"status":      "ERROR",
		"message":     "Waduh, waktu yang dipilih udah lewat kak. Pilih waktu yang lebih nanti ya",
		"next_action": "fix_input_or_retry",
	}, nil
}

func TestExecCompact_NoProgressGuardStopsRepeatedToolLoops(t *testing.T) {
	var (
		mu   sync.Mutex
		call int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		mu.Lock()
		call++
		currentCall := call
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if currentCall <= 2 {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "",
							"tool_calls": []map[string]interface{}{
								{
									"index": 0,
									"id":    "call-loop",
									"type":  "function",
									"function": map[string]interface{}{
										"name":      "check-availability",
										"arguments": `{"date":"2026-03-28","time":"14:00"}`,
									},
								},
							},
						},
					},
				},
			})
			return
		}

		t.Fatalf("model dipanggil terlalu banyak: %d", currentCall)
	}))
	defer server.Close()

	availabilityIntent := &checkAvailabilityNoProgressIntent{}

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &compactFollowUpModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.AgentRuntime = &AgentRuntimeOptions{
		Strategy:   ContextStrategyCompactBackend,
		Summary:    passthroughSummaryAgent{},
		Identifier: fixedIdentifierAgent{},
	}
	cs.Add(availabilityIntent)

	resp, err := cs.Exec(context.Background(), "compact-no-progress-guard", UserMessage{
		Message:         "jam berapa bisa bareng?",
		ParticipantName: "tester",
	})
	require.NoError(t, err)
	require.Contains(t, strings.TrimSpace(resp.Content), "Saya sudah mendapatkan data utamanya.")
	require.Equal(t, 1, availabilityIntent.callCount)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 2, call)
}

func TestExecCompact_NoProgressGuardStillPersistsToolResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]interface{}{
							{
								"index": 0,
								"id":    "call-guard",
								"type":  "function",
								"function": map[string]interface{}{
									"name":      "check-availability",
									"arguments": `{"date":"2026-03-28","time":"14:00"}`,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	intent := &checkAvailabilityNoProgressIntent{}
	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &compactFollowUpModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.Streaming = &StreamingOptions{
		Enabled: true,
		GuardPolicy: StreamGuardPolicy{
			MaxHopsPerTurn:         4,
			MaxSameSignatureRepeat: 1,
			MaxNoProgressLoops:     1,
			MaxToolErrorStreak:     2,
		},
	}
	cs.options.AgentRuntime = &AgentRuntimeOptions{
		Strategy:   ContextStrategyCompactBackend,
		Summary:    passthroughSummaryAgent{},
		Identifier: fixedIdentifierAgent{},
	}
	cs.Add(intent)

	resp, err := cs.Exec(context.Background(), "compact-no-progress-persists-tool", UserMessage{
		Message:         "jam berapa bisa bareng?",
		ParticipantName: "tester",
	})
	require.NoError(t, err)
	require.Contains(t, strings.TrimSpace(resp.Content), "Saya sudah mendapatkan data utamanya.")
	require.Equal(t, 1, intent.callCount)

	messages, err := cs.GetSessionMessages("compact-no-progress-persists-tool")
	require.NoError(t, err)
	require.Len(t, messages, 4)
	require.Equal(t, Tool, messages[2].Role)
	require.Equal(t, "ERROR", extractToolStatus(messages[2].Content))
	require.Equal(t, Assistant, messages[3].Role)
}

func TestToolStatusIndicatesProgress_RecognizesActionableStatuses(t *testing.T) {
	require.True(t, toolStatusIndicatesProgress(""))
	require.True(t, toolStatusIndicatesProgress("SUCCESS"))
	require.True(t, toolStatusIndicatesProgress("AVAILABLE"))
	require.True(t, toolStatusIndicatesProgress("BOOKED"))
	require.True(t, toolStatusIndicatesProgress("NEED_INFO"))
	require.False(t, toolStatusIndicatesProgress("ERROR"))
	require.False(t, toolStatusIndicatesProgress("FAILED"))
}

type checkAvailabilityMissingPayloadIntent struct {
	callCount int
}

func (i *checkAvailabilityMissingPayloadIntent) Code() string { return "check-availability" }
func (i *checkAvailabilityMissingPayloadIntent) Description() []string {
	return []string{"simulate confirm payload required error"}
}
func (i *checkAvailabilityMissingPayloadIntent) Param() interface{} { return struct{}{} }
func (i *checkAvailabilityMissingPayloadIntent) Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) {
	i.callCount++
	return map[string]interface{}{
		"status":      "ERROR",
		"message":     "confirm_payload wajib diisi. Panggil check-availability dulu untuk mendapatkan confirm_payload.",
		"next_action": "recheck_availability_or_input",
	}, nil
}

func TestExecCompact_DoesNotLeakInternalToolErrorWhenFinalAssistantEmpty(t *testing.T) {
	var (
		mu   sync.Mutex
		call int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		mu.Lock()
		call++
		currentCall := call
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch currentCall {
		case 1:
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
										"name":      "check-availability",
										"arguments": `{"date":"2026-03-28","time":"13:00"}`,
									},
								},
							},
						},
					},
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "",
						},
					},
				},
			})
		}
	}))
	defer server.Close()

	intent := &checkAvailabilityMissingPayloadIntent{}

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &compactFollowUpModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.AgentRuntime = &AgentRuntimeOptions{
		Strategy:   ContextStrategyCompactBackend,
		Summary:    passthroughSummaryAgent{},
		Identifier: fixedIdentifierAgent{},
	}
	cs.Add(intent)

	resp, err := cs.Exec(context.Background(), "compact-no-tool-error-leak", UserMessage{
		Message:         "lanjut booking ya",
		ParticipantName: "tester",
	})
	require.NoError(t, err)
	require.Equal(t, 1, intent.callCount)
	require.Equal(t, Assistant, resp.Role)
	require.NotEmpty(t, strings.TrimSpace(resp.Content))
	require.NotContains(t, strings.ToLower(resp.Content), "confirm_payload")
	require.NotContains(t, strings.ToLower(resp.Content), "recheck_availability_or_input")
}
