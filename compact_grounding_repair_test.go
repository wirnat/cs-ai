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

type compactGroundingRepairModel struct {
	apiURL string
}

func (m *compactGroundingRepairModel) ModelName() string { return "kr/claude-sonnet-4.5" }
func (m *compactGroundingRepairModel) ApiURL() string    { return m.apiURL }
func (m *compactGroundingRepairModel) Train() []string   { return []string{"test"} }
func (m *compactGroundingRepairModel) ProviderName() string {
	return "omniroute"
}
func (m *compactGroundingRepairModel) APIMode() string {
	return APIModeChatCompletions
}

type fixedGroundingIdentifierAgent struct{}

func (a fixedGroundingIdentifierAgent) Identify(ctx context.Context, input IdentifierInput) (IdentifierOutput, error) {
	return IdentifierOutput{
		Route:            "run_tool_then_answer",
		NeedTool:         true,
		AllowedToolCodes: []string{"service-catalog"},
		ToolOrder:        []string{"service-catalog"},
		CanAnswerDirect:  false,
		Confidence:       1,
	}, nil
}

type noopGroundingSummaryAgent struct{}

func (a noopGroundingSummaryAgent) Summarize(ctx context.Context, input SummaryInput) (SummaryOutput, error) {
	return SummaryOutput{ConversationSummary: strings.TrimSpace(input.PreviousSummary)}, nil
}

type serviceCatalogGroundingIntent struct {
	callCount int
}

func (i *serviceCatalogGroundingIntent) Code() string { return "service-catalog" }
func (i *serviceCatalogGroundingIntent) Description() []string {
	return []string{"service catalog facts"}
}
func (i *serviceCatalogGroundingIntent) Param() interface{} { return struct{}{} }
func (i *serviceCatalogGroundingIntent) Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) {
	i.callCount++
	return map[string]interface{}{
		"status":      "success",
		"result_kind": "service_menu",
		"display":     "Haircut Rp 35.000 - Rp 50.000",
		"facts": map[string]interface{}{
			"services": []map[string]interface{}{
				{
					"name":          "Haircut",
					"price_display": "Rp 35.000 - Rp 50.000",
				},
			},
		},
		"artifacts": map[string]interface{}{},
		"warnings":  []string{},
	}, nil
}

func TestExecCompact_GroundingRepairTriggersToolCallAfterUngroundedDirectAnswer(t *testing.T) {
	var (
		mu       sync.Mutex
		requests []map[string]interface{}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body := map[string]interface{}{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		mu.Lock()
		requests = append(requests, body)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		switch {
		case hasSystemMessageToken(body, "grounding verifier internal"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": `{"needs_repair":true,"reason":"price_without_tool_evidence"}`,
						},
					},
				},
			})
		case hasSystemMessageToken(body, "GROUNDING REPAIR PASS"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "",
							"tool_calls": []map[string]interface{}{
								{
									"index": 0,
									"id":    "repair-call-1",
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
		case hasToolMessage(body, "repair-call-1"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Price list terbaru: Haircut Rp 35.000 - Rp 50.000.",
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
							"content": "Price list: Haircut Rp 30.000 - Rp 40.000.",
						},
					},
				},
			})
		}
	}))
	defer server.Close()

	serviceCatalog := &serviceCatalogGroundingIntent{}

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &compactGroundingRepairModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.options.GroundingRepair = &GroundingRepairOptions{
		Enabled:     true,
		MaxAttempts: 1,
	}
	cs.options.AgentRuntime = &AgentRuntimeOptions{
		Strategy:   ContextStrategyCompactBackend,
		Summary:    noopGroundingSummaryAgent{},
		Identifier: fixedGroundingIdentifierAgent{},
	}
	cs.Add(serviceCatalog)

	resp, err := cs.Exec(context.Background(), "compact-grounding-repair", UserMessage{
		Message:         "bisa minta price listnya?",
		ParticipantName: "tester",
	})
	require.NoError(t, err)
	require.Equal(t, "Price list terbaru: Haircut Rp 35.000 - Rp 50.000.", strings.TrimSpace(resp.Content))
	require.Equal(t, 1, serviceCatalog.callCount)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, requests, 4)
}

func hasSystemMessageToken(body map[string]interface{}, token string) bool {
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

func hasToolMessage(body map[string]interface{}, toolCallID string) bool {
	rawMessages, ok := body["messages"].([]interface{})
	if !ok {
		return false
	}
	for _, raw := range rawMessages {
		msg, castOK := raw.(map[string]interface{})
		if !castOK {
			continue
		}
		if strings.ToLower(strings.TrimSpace(toString(msg["role"]))) != "tool" {
			continue
		}
		if strings.TrimSpace(toString(msg["tool_call_id"])) == strings.TrimSpace(toolCallID) {
			return true
		}
	}
	return false
}
