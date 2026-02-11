package cs_ai

import (
	"testing"
	"time"
)

type noOpModel struct{}

func (m *noOpModel) ModelName() string { return "test-model" }
func (m *noOpModel) ApiURL() string    { return "http://localhost/test" }
func (m *noOpModel) Train() []string   { return []string{"test"} }

func newTestCsAIWithInMemoryStorage(t *testing.T) *CsAI {
	t.Helper()

	provider, err := NewInMemoryStorageProvider(StorageConfig{
		Type:       StorageTypeInMemory,
		SessionTTL: 1 * time.Hour,
		Timeout:    1 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create in-memory provider: %v", err)
	}

	return New("test-api-key", &noOpModel{}, Options{
		StorageProvider: provider,
		SessionTTL:      1 * time.Hour,
	})
}

func TestDeleteMessageFromSession_TruncatesAndPersists(t *testing.T) {
	cs := newTestCsAIWithInMemoryStorage(t)
	sessionID := "delete-truncate"

	original := []Message{
		{Role: User, Content: "user-1"},
		{Role: Assistant, Content: "assistant-1"},
		{Role: User, Content: "user-2"},
		{Role: Assistant, Content: "assistant-2"},
	}

	if _, err := cs.SaveSessionMessages(sessionID, original); err != nil {
		t.Fatalf("failed to seed session messages: %v", err)
	}

	// Delete message with ID #3. Messages ID #3 and above should be removed.
	if err := cs.DeleteMessageFromSession(sessionID, 3); err != nil {
		t.Fatalf("DeleteMessageFromSession returned error: %v", err)
	}

	got, err := cs.GetSessionMessages(sessionID)
	if err != nil {
		t.Fatalf("failed to get session messages: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 messages after truncation, got %d", len(got))
	}
	if got[0].Content != "user-1" || got[1].Content != "assistant-1" {
		t.Fatalf("unexpected messages after truncation: %+v", got)
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("expected remaining IDs to be [1,2], got [%d,%d]", got[0].ID, got[1].ID)
	}
}

func TestDeleteMessageFromSession_RemovesUnsafeTrailingToolContext(t *testing.T) {
	cs := newTestCsAIWithInMemoryStorage(t)
	sessionID := "delete-tool-boundary"

	toolCall := ToolCall{Id: "call-1", Type: "function"}
	toolCall.Function.Name = "get_data"
	toolCall.Function.Arguments = `{"id":1}`

	original := []Message{
		{Role: User, Content: "show data"},
		{Role: Assistant, ToolCalls: []ToolCall{toolCall}},
		{Role: Tool, Content: `{"ok":true}`, ToolCallID: "call-1"},
		{Role: Assistant, Content: "done"},
	}

	if _, err := cs.SaveSessionMessages(sessionID, original); err != nil {
		t.Fatalf("failed to seed session messages: %v", err)
	}

	// Delete message with ID #4. Raw truncation would end at a tool response (#3),
	// so delete logic must also remove trailing tool + assistant(tool_calls).
	if err := cs.DeleteMessageFromSession(sessionID, 4); err != nil {
		t.Fatalf("DeleteMessageFromSession returned error: %v", err)
	}

	got, err := cs.GetSessionMessages(sessionID)
	if err != nil {
		t.Fatalf("failed to get session messages: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 safe message after cleanup, got %d", len(got))
	}
	if got[0].Role != User || got[0].Content != "show data" {
		t.Fatalf("unexpected remaining message: %+v", got[0])
	}
	if got[0].ID != 1 {
		t.Fatalf("expected remaining message ID to be 1, got %d", got[0].ID)
	}
}

func TestDeleteMessageFromSession_InvalidID(t *testing.T) {
	cs := newTestCsAIWithInMemoryStorage(t)
	sessionID := "delete-invalid-id"

	if _, err := cs.SaveSessionMessages(sessionID, []Message{{Role: User, Content: "hello"}}); err != nil {
		t.Fatalf("failed to seed session messages: %v", err)
	}

	if err := cs.DeleteMessageFromSession(sessionID, 0); err == nil {
		t.Fatal("expected error for id 0, got nil")
	}
	if err := cs.DeleteMessageFromSession(sessionID, 2); err == nil {
		t.Fatal("expected out-of-range error, got nil")
	}
}

func TestSaveSessionMessages_AssignsAutoIncrementIDs(t *testing.T) {
	cs := newTestCsAIWithInMemoryStorage(t)
	sessionID := "auto-increment-ids"

	seed := []Message{
		{Role: User, Content: "first", ID: 9},
		{Role: Assistant, Content: "second", ID: 99},
		{Role: User, Content: "third"},
	}

	if _, err := cs.SaveSessionMessages(sessionID, seed); err != nil {
		t.Fatalf("failed to save session messages: %v", err)
	}

	got, err := cs.GetSessionMessages(sessionID)
	if err != nil {
		t.Fatalf("failed to get session messages: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(got))
	}

	for i := range got {
		expectedID := i + 1
		if got[i].ID != expectedID {
			t.Fatalf("expected message #%d to have ID %d, got %d", i, expectedID, got[i].ID)
		}
	}
}
