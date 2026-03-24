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

type repeatingErrorToolModel struct {
	apiURL string
}

func (m *repeatingErrorToolModel) ModelName() string { return "repeating-error-tool-model" }
func (m *repeatingErrorToolModel) ApiURL() string    { return m.apiURL }
func (m *repeatingErrorToolModel) Train() []string   { return []string{"test"} }

type repeatingErrorIntent struct{}

func (i *repeatingErrorIntent) Code() string { return "semantic_sql_readonly" }
func (i *repeatingErrorIntent) Description() []string {
	return []string{"simulate repeatable tool error"}
}
func (i *repeatingErrorIntent) Param() interface{} {
	return struct {
		SQL string `json:"sql"`
	}{}
}
func (i *repeatingErrorIntent) Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"status":  "ERROR",
		"message": "Query semantic SQL belum bisa dijalankan dengan aman.",
		"error": map[string]interface{}{
			"code":      "semantic_sql_schema_mismatch",
			"retryable": true,
		},
		"retry_hint": "Periksa ulang nama kolom dan schema ai_staff_performance.",
	}, nil
}

func TestExec_StopsRepeatedIdenticalFailingToolLoopsEarly(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

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
								"id":    "call-1",
								"type":  "function",
								"function": map[string]interface{}{
									"name":      "semantic_sql_readonly",
									"arguments": `{"sql":"SELECT sp.provider_uid FROM ai_staff_performance sp"}`,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &repeatingErrorToolModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.Add(&repeatingErrorIntent{})

	resp, err := cs.Exec(context.Background(), "repeat-error-tool-session", UserMessage{
		Message:         "siapa capster terbaik bulan ini?",
		ParticipantName: "tester",
	})

	require.NoError(t, err)
	assert.Equal(t, Assistant, resp.Role)
	assert.Equal(t, "Saya sudah mendapatkan data utamanya. Saya hentikan iterasi tambahan agar tidak berputar terlalu lama, dan saya pakai hasil terbaik yang sudah terkumpul dulu ya.", resp.Content)
	assert.Equal(t, 2, callCount)
}

func TestExec_StopsConsecutiveFailingToolLoopsEvenWhenArgumentsChange(t *testing.T) {
	callCount := 0
	sqls := []string{
		`{"sql":"SELECT sp.provider_uid FROM ai_staff_performance sp"}`,
		`{"sql":"SELECT sp.capster_uid FROM ai_staff_performance sp"}`,
		`{"sql":"SELECT sp.provider_name FROM ai_staff_performance sp"}`,
		`{"sql":"SELECT sp.capster_name FROM ai_staff_performance sp"}`,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		index := callCount - 1
		if index >= len(sqls) {
			index = len(sqls) - 1
		}

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
								"id":    "call-1",
								"type":  "function",
								"function": map[string]interface{}{
									"name":      "semantic_sql_readonly",
									"arguments": sqls[index],
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &repeatingErrorToolModel{apiURL: server.URL}
	cs.options.UseTool = true
	cs.Add(&repeatingErrorIntent{})

	resp, err := cs.Exec(context.Background(), "repeat-changing-error-tool-session", UserMessage{
		Message:         "siapa capster terbaik bulan ini?",
		ParticipantName: "tester",
	})

	require.NoError(t, err)
	assert.Equal(t, Assistant, resp.Role)
	assert.Equal(t, "Saya sudah mendapatkan data utamanya. Saya hentikan iterasi tambahan agar tidak berputar terlalu lama, dan saya pakai hasil terbaik yang sudah terkumpul dulu ya.", resp.Content)
	assert.Equal(t, 3, callCount)
}
