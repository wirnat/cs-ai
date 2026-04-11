package cs_ai

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestJSONLStreamSink_EmitWritesSingleLineJSON(t *testing.T) {
	var buf bytes.Buffer
	sink := NewJSONLStreamSink(&buf)
	if sink == nil {
		t.Fatalf("expected sink instance")
	}

	err := sink.Emit(context.Background(), StreamEvent{
		TurnID:  "turn-1",
		Type:    "turn.started",
		Status:  "ok",
		Message: "mulai",
	})
	if err != nil {
		t.Fatalf("emit returned error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output == "" {
		t.Fatalf("expected non-empty output")
	}
	if !strings.Contains(output, `"type":"turn.started"`) {
		t.Fatalf("unexpected jsonl output: %s", output)
	}
}
