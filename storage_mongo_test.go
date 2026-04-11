package cs_ai

import (
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestBuildMongoSessionMessagesDocument_WithConversationPayload(t *testing.T) {
	messages := []Message{
		{Content: "halo", Role: User},
		{
			Role:    Assistant,
			Content: "",
			ToolCalls: []ToolCall{{
				Index: 0,
				Id:    "call-1",
				Type:  "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: "service-catalog", Arguments: `{"provider_uid":null}`},
			}},
		},
		{
			Role:       Tool,
			ToolCallID: "call-1",
			Content:    `{"data":{"count":1,"services":[{"uid":"svc-1","name":"Haircut"}]},"status":"SUCCESS"}`,
		},
		{
			Role:    Assistant,
			Content: "Siang kak, untuk cukur hari ini bisa jam berapa?",
		},
	}

	doc, err := buildMongoSessionMessagesDocument("session-1", messages, time.Hour)
	if err != nil {
		t.Fatalf("expected mongo document to be encodable, got error: %v", err)
	}

	stored, ok := doc["messages"].([]Message)
	if !ok {
		t.Fatalf("messages type mismatch: %T", doc["messages"])
	}
	if len(stored) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(stored))
	}
	if stored[0].ID != 1 || stored[3].ID != 4 {
		t.Fatalf("expected auto-increment IDs 1..4, got first=%d last=%d", stored[0].ID, stored[3].ID)
	}
	if stored[2].ContentMap == nil {
		t.Fatalf("expected tool JSON content to populate content_map")
	}
}

func TestBuildMongoSessionMessagesDocument_PreservesJSONContentMapShape(t *testing.T) {
	messages := []Message{{
		Role:    Tool,
		Content: `{"nested":{"items":[{"name":"Haircut","price":110000}]},"status":"SUCCESS"}`,
	}}

	doc, err := buildMongoSessionMessagesDocument("session-2", messages, time.Hour)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal bson: %v", err)
	}

	decoded := map[string]interface{}{}
	if err := bson.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal bson: %v", err)
	}

	normalizedMessages, ok := NormalizeMongoDocumentForComparison(decoded["messages"]).([]interface{})
	if !ok || len(normalizedMessages) != 1 {
		t.Fatalf("decoded messages mismatch: %#v", decoded["messages"])
	}

	decodedMessage, ok := normalizedMessages[0].(map[string]interface{})
	if !ok {
		t.Fatalf("decoded message mismatch: %#v", normalizedMessages[0])
	}

	expectedContentMap := map[string]interface{}{
		"nested": map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"name": "Haircut", "price": float64(110000)},
			},
		},
		"status": "SUCCESS",
	}

	if !reflect.DeepEqual(
		NormalizeMongoDocumentForComparison(decodedMessage["content_map"]),
		NormalizeMongoDocumentForComparison(expectedContentMap),
	) {
		t.Fatalf("content_map mismatch\nwant=%#v\ngot=%#v", expectedContentMap, decodedMessage["content_map"])
	}
}

func TestBuildMongoSessionMessagesDocument_SetsSessionTotalUsage(t *testing.T) {
	messages := []Message{
		{
			Role:  Assistant,
			Usage: &DeepSeekUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		},
		{
			Role:            Assistant,
			Usage:           &DeepSeekUsage{PromptTokens: 999, CompletionTokens: 999, TotalTokens: 1998},
			AggregatedUsage: &DeepSeekUsage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30, PromptCacheHitTokens: 5},
		},
	}

	doc, err := buildMongoSessionMessagesDocument("session-usage", messages, time.Hour)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	rawTotal, ok := doc["session_total_usage"]
	if !ok {
		t.Fatalf("expected session_total_usage field to exist")
	}

	sessionTotal, ok := rawTotal.(DeepSeekUsage)
	if !ok {
		t.Fatalf("session_total_usage type mismatch: %T", rawTotal)
	}

	expected := DeepSeekUsage{}.
		Add(*messages[0].Usage).
		Add(*messages[1].AggregatedUsage)

	if !reflect.DeepEqual(sessionTotal, expected) {
		t.Fatalf("session_total_usage mismatch\nwant=%+v\ngot=%+v", expected, sessionTotal)
	}

	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal bson: %v", err)
	}

	decoded := map[string]interface{}{}
	if err := bson.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal bson: %v", err)
	}

	if _, exists := decoded["session_total_usage"]; !exists {
		t.Fatalf("expected session_total_usage to be persisted in bson document")
	}
}

func TestBuildMongoSessionMessagesDocument_StripsInternalResponseMetadataFromMessages(t *testing.T) {
	messages := []Message{
		{
			Role:       Assistant,
			Content:    `{"status":"BOOKED","booking_code":"INV-1"}`,
			ResponseID: "resp_internal_123",
			Usage:      &DeepSeekUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			AggregatedUsage: &DeepSeekUsage{
				PromptTokens: 30, CompletionTokens: 7, TotalTokens: 37,
			},
			Reasoning: &ResponseReasoningMetadata{
				SummaryText: "internal summary",
				EffortUsed:  "medium",
			},
		},
	}

	doc, err := buildMongoSessionMessagesDocument("session-redacted", messages, time.Hour)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	stored, ok := doc["messages"].([]Message)
	if !ok || len(stored) != 1 {
		t.Fatalf("messages type mismatch: %#v", doc["messages"])
	}

	if stored[0].ResponseID != "" {
		t.Fatalf("expected response_id stripped from stored message, got %q", stored[0].ResponseID)
	}
	if stored[0].AggregatedUsage != nil {
		t.Fatalf("expected aggregated_usage stripped from stored message, got %#v", stored[0].AggregatedUsage)
	}
	if stored[0].Reasoning != nil {
		t.Fatalf("expected reasoning stripped from stored message, got %#v", stored[0].Reasoning)
	}
	if stored[0].ContentMap == nil {
		t.Fatalf("expected content_map to remain populated for JSON content")
	}

	sessionTotal, ok := doc["session_total_usage"].(DeepSeekUsage)
	if !ok {
		t.Fatalf("session_total_usage type mismatch: %T", doc["session_total_usage"])
	}
	if sessionTotal.TotalTokens != 37 {
		t.Fatalf("expected session_total_usage to keep aggregate accounting, got %+v", sessionTotal)
	}
}
