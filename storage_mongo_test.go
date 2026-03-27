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
