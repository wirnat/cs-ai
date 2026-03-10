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

	body := buildRequestBodyByAPIMode(APIModeOpenAICodexResponses, "gpt-5.4", roleMessages, functions, Options{
		UseTool: true,
	})

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
							"date": {"type": "string"},
							"time": {"type": "string"},
							"discount_uids": {"type": "array"},
						},
						"required": []string{"date", "time", "extra"},
					},
				},
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

	if strict, ok := tools[3]["strict"].(bool); !ok || strict {
		t.Fatalf("expected strict=false for codex tools, got: %#v", tools[3]["strict"])
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
	if len(required) != 2 {
		t.Fatalf("expected invalid required keys filtered out, got: %#v", required)
	}
	discountField, ok := fourthProps["discount_uids"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected discount_uids schema object, got: %#v", fourthProps["discount_uids"])
	}
	if _, ok := discountField["items"]; !ok {
		t.Fatalf("expected array field to include items schema, got: %#v", discountField)
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

	_, statusCode, err := RequestDetailed(server.URL, http.MethodPost, map[string]interface{}{
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
