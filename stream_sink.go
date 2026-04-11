package cs_ai

import (
	"context"
	"encoding/json"
	"io"
	"sync"
)

// JSONLStreamSink menulis event stream sebagai JSON per baris (JSONL).
// Cocok untuk observability/log collection.
type JSONLStreamSink struct {
	writer io.Writer
	mu     sync.Mutex
}

func NewJSONLStreamSink(writer io.Writer) *JSONLStreamSink {
	if writer == nil {
		return nil
	}
	return &JSONLStreamSink{writer: writer}
}

func (s *JSONLStreamSink) Emit(ctx context.Context, event StreamEvent) error {
	if s == nil || s.writer == nil {
		return nil
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.writer.Write(payload); err != nil {
		return err
	}
	_, err = s.writer.Write([]byte("\n"))
	return err
}

// MemoryStreamSink menyimpan event stream in-memory untuk testing/debug.
type MemoryStreamSink struct {
	mu     sync.Mutex
	events []StreamEvent
}

func NewMemoryStreamSink() *MemoryStreamSink {
	return &MemoryStreamSink{}
}

func (s *MemoryStreamSink) Emit(ctx context.Context, event StreamEvent) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *MemoryStreamSink) Snapshot() []StreamEvent {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]StreamEvent, len(s.events))
	copy(out, s.events)
	return out
}
