package cs_ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type streamExecTestModel struct {
	apiURL string
}

func (m *streamExecTestModel) ModelName() string { return "stream-exec-test-model" }
func (m *streamExecTestModel) ApiURL() string    { return m.apiURL }
func (m *streamExecTestModel) Train() []string   { return []string{"test"} }

func TestExecStream_EmitsTurnAndStageEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"siap kak"}}]}`))
	}))
	defer server.Close()

	cs := New("test-key", &streamExecTestModel{apiURL: server.URL}, Options{
		Streaming: &StreamingOptions{
			Enabled:      true,
			EmitProgress: true,
		},
	})
	sink := NewMemoryStreamSink()

	resp, err := cs.ExecStream(context.Background(), "stream-session-1", UserMessage{
		Message:         "halo",
		ParticipantName: "Tester",
	}, sink)
	if err != nil {
		t.Fatalf("exec stream returned error: %v", err)
	}
	if resp.Content != "siap kak" {
		t.Fatalf("unexpected assistant response: %q", resp.Content)
	}

	events := sink.Snapshot()
	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(events))
	}

	types := make(map[string]int)
	var previousSeq int64
	for _, event := range events {
		types[event.Type]++
		if event.Seq <= previousSeq {
			t.Fatalf("event sequence must be increasing, got %d then %d", previousSeq, event.Seq)
		}
		previousSeq = event.Seq
		if event.TurnID == "" {
			t.Fatalf("turn_id must be populated for all events")
		}
	}

	if types["turn.started"] == 0 {
		t.Fatalf("expected turn.started event")
	}
	if types["agent.stage.started"] == 0 || types["agent.stage.completed"] == 0 {
		t.Fatalf("expected stage started/completed events, got %+v", types)
	}
	if types["turn.completed"] == 0 {
		t.Fatalf("expected turn.completed event")
	}
}

func TestExecStreamWithToolCodes_UsesStreamRunner(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"siap kak"}}]}`))
	}))
	defer server.Close()

	cs := New("test-key", &streamExecTestModel{apiURL: server.URL}, Options{
		Streaming: &StreamingOptions{
			Enabled:      true,
			EmitProgress: true,
		},
	})
	sink := NewMemoryStreamSink()

	_, err := cs.ExecStreamWithToolCodes(context.Background(), "stream-session-2", UserMessage{
		Message:         "halo",
		ParticipantName: "Tester",
	}, []string{}, sink)
	if err != nil {
		t.Fatalf("exec stream with tool codes returned error: %v", err)
	}

	events := sink.Snapshot()
	if len(events) == 0 {
		t.Fatalf("expected stream events to be emitted")
	}
}

func TestShouldStreamModelRequest_RespectsCompletionMode(t *testing.T) {
	ctx := withStageStreaming(context.Background(), AgentStageSummary, StageStreamingConfig{
		Mode:          StageModeCompletion,
		EmitTextDelta: true,
		EmitProgress:  true,
	})
	if shouldStreamModelRequest(ctx) {
		t.Fatalf("expected stream mode disabled for completion stage")
	}
}
