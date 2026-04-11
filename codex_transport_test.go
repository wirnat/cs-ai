package cs_ai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildRequestBodyByAPIMode_OpenAICodexResponses(t *testing.T) {
	roleMessages := []map[string]interface{}{
		{"role": "system", "content": "sys-1"},
		{"role": "system", "content": "sys-2"},
		{"role": "user", "content": "hello"},
		{
			"role": "assistant",
			"tool_calls": []interface{}{
				map[string]interface{}{
					"id":   "call-1",
					"type": "function",
					"function": map[string]interface{}{
						"name":      "get_time",
						"arguments": "{}",
					},
				},
			},
		},
		{"role": "tool", "tool_call_id": "call-1", "content": "{\"time\":\"10:00\"}"},
	}
	functions := []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_time",
				"description": "ambil waktu",
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []interface{}{},
				},
			},
		},
	}

	body := buildRequestBodyByAPIMode(APIModeOpenAICodexResponses, "openai-codex", "gpt-5.4", roleMessages, functions, Options{
		UseTool: true,
	}, requestBuildConfig{})

	if got := toString(body["model"]); got != "gpt-5.4" {
		t.Fatalf("unexpected model: %q", got)
	}
	if got := toString(body["instructions"]); got != "sys-1\nsys-2" {
		t.Fatalf("unexpected instructions: %q", got)
	}
	if stream, ok := body["stream"].(bool); !ok || !stream {
		t.Fatalf("expected stream=true, got: %#v", body["stream"])
	}
	if store, ok := body["store"].(bool); !ok || store {
		t.Fatalf("expected store=false, got: %#v", body["store"])
	}
	if _, exists := body["temperature"]; exists {
		t.Fatalf("temperature must not be sent in codex payload")
	}
	if _, exists := body["max_output_tokens"]; exists {
		t.Fatalf("max_output_tokens must not be sent in codex payload")
	}

	input, ok := body["input"].([]map[string]interface{})
	if !ok {
		t.Fatalf("input type mismatch: %#v", body["input"])
	}
	if len(input) != 3 {
		t.Fatalf("expected 3 input items, got %d", len(input))
	}
	if toString(input[0]["role"]) != "user" || toString(input[0]["content"]) != "hello" {
		t.Fatalf("unexpected user input item: %#v", input[0])
	}
	if toString(input[1]["type"]) != "function_call" || toString(input[1]["call_id"]) != "call-1" {
		t.Fatalf("unexpected function_call item: %#v", input[1])
	}
	if toString(input[2]["type"]) != "function_call_output" || toString(input[2]["call_id"]) != "call-1" {
		t.Fatalf("unexpected function_call_output item: %#v", input[2])
	}

	tools, ok := body["tools"].([]map[string]interface{})
	if !ok {
		t.Fatalf("tools type mismatch: %#v", body["tools"])
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if toString(tools[0]["name"]) != "get_time" || toString(tools[0]["type"]) != "function" {
		t.Fatalf("unexpected tool payload: %#v", tools[0])
	}
	params, ok := tools[0]["parameters"].(map[string]interface{})
	if !ok {
		t.Fatalf("tool parameters type mismatch: %#v", tools[0]["parameters"])
	}
	if additional, ok := params["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("expected additionalProperties=false in tool parameters, got: %#v", params["additionalProperties"])
	}
}

func TestBuildRequestBodyByAPIMode_OpenAICodexResponses_WithReasoning(t *testing.T) {
	body := buildRequestBodyByAPIMode(APIModeOpenAICodexResponses, "openai-codex", "gpt-5.4", []map[string]interface{}{
		{"role": "system", "content": "sys"},
		{"role": "user", "content": "halo"},
	}, nil, Options{}, requestBuildConfig{
		Reasoning: &ReasoningConfig{
			Effort:  ReasoningEffortMedium,
			Summary: ReasoningSummaryConcise,
		},
		PreviousResponseID: "resp_prev_123",
	})

	reasoning, ok := body["reasoning"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected reasoning payload, got %#v", body["reasoning"])
	}
	if toString(reasoning["effort"]) != "medium" {
		t.Fatalf("unexpected reasoning effort: %#v", reasoning["effort"])
	}
	if toString(reasoning["summary"]) != "concise" {
		t.Fatalf("unexpected reasoning summary: %#v", reasoning["summary"])
	}
	if toString(body["previous_response_id"]) != "resp_prev_123" {
		t.Fatalf("unexpected previous_response_id: %#v", body["previous_response_id"])
	}
}

func TestBuildRequestBodyByAPIMode_OpenAICodexResponses_StreamOverride(t *testing.T) {
	streamDisabled := false
	body := buildRequestBodyByAPIMode(APIModeOpenAICodexResponses, "openai-codex", "gpt-5.4", []map[string]interface{}{
		{"role": "system", "content": "sys"},
		{"role": "user", "content": "halo"},
	}, nil, Options{}, requestBuildConfig{
		Stream: &streamDisabled,
	})

	if stream, ok := body["stream"].(bool); !ok || stream {
		t.Fatalf("expected stream=false, got %#v", body["stream"])
	}
}

func TestBuildCodexTools_NormalizesSchemaForStrictMode(t *testing.T) {
	functions := []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name": "tenant-outlet-info",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"filters": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"city": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":       "time-now",
				"parameters": nil,
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":       "product-catalog",
				"parameters": map[string]interface{}{},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name": "appointment-booking",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]map[string]interface{}{
						"date":          {"type": "string"},
						"time":          {"type": "string"},
						"discount_uids": {"type": "array"},
					},
					"required": []string{"date", "time", "extra"},
				},
			},
			"_csai_strict": true,
		},
	}

	tools := buildCodexTools(functions)
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}

	firstParams, ok := tools[0]["parameters"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected first params type: %#v", tools[0]["parameters"])
	}
	if additional, ok := firstParams["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("expected root additionalProperties=false, got: %#v", firstParams["additionalProperties"])
	}
	props, ok := firstParams["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected properties type: %#v", firstParams["properties"])
	}
	filters, ok := props["filters"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected nested filters type: %#v", props["filters"])
	}
	if additional, ok := filters["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("expected nested additionalProperties=false, got: %#v", filters["additionalProperties"])
	}

	secondParams, ok := tools[1]["parameters"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected second params type: %#v", tools[1]["parameters"])
	}
	if toString(secondParams["type"]) != "object" {
		t.Fatalf("expected default type=object, got: %#v", secondParams["type"])
	}
	if additional, ok := secondParams["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("expected default additionalProperties=false, got: %#v", secondParams["additionalProperties"])
	}

	thirdParams, ok := tools[2]["parameters"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected third params type: %#v", tools[2]["parameters"])
	}
	if toString(thirdParams["type"]) != "object" {
		t.Fatalf("expected empty schema to become type=object, got: %#v", thirdParams["type"])
	}
	if additional, ok := thirdParams["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("expected empty schema additionalProperties=false, got: %#v", thirdParams["additionalProperties"])
	}

	if strict, ok := tools[3]["strict"].(bool); !ok || !strict {
		t.Fatalf("expected strict=true for codex tools with internal flag, got: %#v", tools[3]["strict"])
	}
	fourthParams, ok := tools[3]["parameters"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected fourth params type: %#v", tools[3]["parameters"])
	}
	fourthProps, ok := fourthParams["properties"].(map[string]interface{})
	if !ok || len(fourthProps) != 3 {
		t.Fatalf("expected typed-map properties to survive normalization, got: %#v", fourthParams["properties"])
	}
	required, ok := fourthParams["required"].([]interface{})
	if !ok {
		t.Fatalf("unexpected required type: %#v", fourthParams["required"])
	}
	if len(required) != 3 {
		t.Fatalf("expected strict schema to require all properties, got: %#v", required)
	}
	discountField, ok := fourthProps["discount_uids"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected discount_uids schema object, got: %#v", fourthProps["discount_uids"])
	}
	if _, ok := discountField["items"]; !ok {
		t.Fatalf("expected array field to include items schema, got: %#v", discountField)
	}
	discountType, ok := discountField["type"].([]interface{})
	if !ok || len(discountType) != 2 || toString(discountType[0]) != "array" || toString(discountType[1]) != "null" {
		t.Fatalf("expected optional strict field to become nullable, got: %#v", discountField["type"])
	}
	dateField, ok := fourthProps["date"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected date schema object, got: %#v", fourthProps["date"])
	}
	if toString(dateField["type"]) != "string" {
		t.Fatalf("expected required field type to stay string, got: %#v", dateField["type"])
	}
}

func TestBuildCodexTools_StrictOptionalFieldBecomesNullableAndRequired(t *testing.T) {
	functions := []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name": "service-catalog",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"provider_uid": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{},
				},
			},
			"_csai_strict": true,
		},
	}

	tools := buildCodexTools(functions)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	params, ok := tools[0]["parameters"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected params type: %#v", tools[0]["parameters"])
	}
	required, ok := params["required"].([]interface{})
	if !ok || len(required) != 1 || toString(required[0]) != "provider_uid" {
		t.Fatalf("expected provider_uid to be required in strict schema, got: %#v", params["required"])
	}

	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected properties type: %#v", params["properties"])
	}
	providerField, ok := props["provider_uid"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected provider_uid schema: %#v", props["provider_uid"])
	}
	providerType, ok := providerField["type"].([]interface{})
	if !ok || len(providerType) != 2 || toString(providerType[0]) != "string" || toString(providerType[1]) != "null" {
		t.Fatalf("expected provider_uid to be nullable under strict mode, got: %#v", providerField["type"])
	}
}

func TestParseSSEFinalResponse_Completed(t *testing.T) {
	stream := strings.Join([]string{
		"event: response.created",
		"data: {\"type\":\"response.created\"}",
		"",
		"event: response.completed",
		"data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-5.4\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"pong\"}]}]}}",
		"",
	}, "\n")

	result, body, err := parseSSEFinalResponse(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if toString(result["model"]) != "gpt-5.4" {
		t.Fatalf("unexpected parsed model: %#v", result["model"])
	}
	if len(body) == 0 {
		t.Fatalf("expected non-empty serialized body")
	}
}

func TestParseSSEFinalResponseWithObserver_EmitsIncrementalEvents(t *testing.T) {
	stream := strings.Join([]string{
		"event: response.output_text.delta",
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"po\"}",
		"",
		"event: response.output_text.delta",
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ng\"}",
		"",
		"event: response.completed",
		"data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-5.4\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"pong\"}]}]}}",
		"",
	}, "\n")

	events := make([]RequestStreamEvent, 0, 3)
	result, _, err := parseSSEFinalResponseWithObserver(strings.NewReader(stream), func(event RequestStreamEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected parsed result")
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 stream events, got %d", len(events))
	}
	if events[0].EventType != "response.output_text.delta" || toString(events[0].Payload["delta"]) != "po" {
		t.Fatalf("unexpected first event payload: %#v", events[0])
	}
	if events[2].EventType != "response.completed" {
		t.Fatalf("unexpected last event type: %#v", events[2].EventType)
	}
}

func TestParseSSEFinalResponse_ResponsesFunctionCallReconstructedFromDeltas(t *testing.T) {
	stream := strings.Join([]string{
		"event: response.output_item.added",
		"data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"id\":\"fc_123\",\"type\":\"function_call\",\"name\":\"get-provider-availability\",\"arguments\":\"\"}}",
		"",
		"event: response.function_call_arguments.delta",
		"data: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"fc_123\",\"delta\":\"{\\\"provider_name\\\":\\\"erick\\\"\"}",
		"",
		"event: response.function_call_arguments.delta",
		"data: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"fc_123\",\"delta\":\",\\\"date\\\":\\\"today\\\"}\"}",
		"",
		"event: response.completed",
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"kr/claude-sonnet-4.5\"}}",
		"",
	}, "\n")

	result, _, err := parseSSEFinalResponse(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected parsed result")
	}

	msg, err := MessageFromResponsesMap(result)
	if err != nil {
		t.Fatalf("unexpected map conversion error: %v", err)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "get-provider-availability" {
		t.Fatalf("unexpected tool call name: %q", msg.ToolCalls[0].Function.Name)
	}
	if msg.ToolCalls[0].Function.Arguments != "{\"provider_name\":\"erick\",\"date\":\"today\"}" {
		t.Fatalf("unexpected tool call arguments: %q", msg.ToolCalls[0].Function.Arguments)
	}
}

func TestRequestDetailed_StreamFailedCodexCreditsMappedToQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Codex-Credits-Has-Credits", "False")
		w.Header().Set("X-Codex-Credits-Unlimited", "False")
		_, _ = w.Write([]byte(strings.Join([]string{
			"event: response.failed",
			"data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"code\":\"server_error\",\"message\":\"generic failure\"}}}",
			"",
		}, "\n")))
	}))
	defer server.Close()

	_, statusCode, _, err := RequestDetailed(server.URL, http.MethodPost, map[string]interface{}{
		"model":  "gpt-5.4",
		"stream": true,
	}, func(req *http.Request) {})
	if err == nil {
		t.Fatalf("expected error for failed stream response")
	}

	apiErr, ok := err.(*APIRequestError)
	if !ok {
		t.Fatalf("expected APIRequestError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", apiErr.StatusCode)
	}
	if statusCode != http.StatusTooManyRequests {
		t.Fatalf("expected returned status code 429, got %d", statusCode)
	}
	lowerBody := strings.ToLower(apiErr.Body)
	if !strings.Contains(lowerBody, "insufficient_quota") && !strings.Contains(lowerBody, "credit") {
		t.Fatalf("expected quota/credit hint in body, got: %s", apiErr.Body)
	}
}
