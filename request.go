package cs_ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

type APIRequestError struct {
	StatusCode int
	Body       string
	Err        error
}

type RequestStreamEvent struct {
	EventType string
	Payload   map[string]interface{}
	Raw       []byte
}

type RequestStreamObserver func(RequestStreamEvent)

func (e *APIRequestError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		if e.StatusCode > 0 {
			return fmt.Sprintf("request failed with status %d: %v", e.StatusCode, e.Err)
		}
		return e.Err.Error()
	}
	if e.StatusCode > 0 {
		if e.Body != "" {
			return fmt.Sprintf("request failed with status %d: %s", e.StatusCode, e.Body)
		}
		return fmt.Sprintf("request failed with status %d", e.StatusCode)
	}
	if e.Body != "" {
		return e.Body
	}
	return "request failed"
}

func (e *APIRequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *APIRequestError) IsTimeout() bool {
	if e == nil {
		return false
	}
	if e.StatusCode == http.StatusRequestTimeout || e.StatusCode == http.StatusGatewayTimeout {
		return true
	}
	var netErr net.Error
	return errors.As(e.Err, &netErr) && netErr.Timeout()
}

func Request(url string, method string, reqBody map[string]interface{}, setHeader func(*http.Request)) (result map[string]interface{}, err error) {
	result, _, _, err = RequestDetailed(url, method, reqBody, setHeader)
	return result, err
}

func RequestDetailed(url string, method string, reqBody map[string]interface{}, setHeader func(*http.Request)) (result map[string]interface{}, statusCode int, responseHeaders map[string]string, err error) {
	return requestDetailed(context.Background(), url, method, reqBody, setHeader, nil)
}

func RequestDetailedWithContext(ctx context.Context, url string, method string, reqBody map[string]interface{}, setHeader func(*http.Request)) (result map[string]interface{}, statusCode int, responseHeaders map[string]string, err error) {
	return requestDetailed(ctx, url, method, reqBody, setHeader, nil)
}

func RequestDetailedWithObserver(
	url string,
	method string,
	reqBody map[string]interface{},
	setHeader func(*http.Request),
	observer RequestStreamObserver,
) (result map[string]interface{}, statusCode int, responseHeaders map[string]string, err error) {
	return requestDetailed(context.Background(), url, method, reqBody, setHeader, observer)
}

func RequestDetailedWithContextAndObserver(
	ctx context.Context,
	url string,
	method string,
	reqBody map[string]interface{},
	setHeader func(*http.Request),
	observer RequestStreamObserver,
) (result map[string]interface{}, statusCode int, responseHeaders map[string]string, err error) {
	return requestDetailed(ctx, url, method, reqBody, setHeader, observer)
}

func requestDetailed(
	ctx context.Context,
	url string,
	method string,
	reqBody map[string]interface{},
	setHeader func(*http.Request),
	observer RequestStreamObserver,
) (result map[string]interface{}, statusCode int, responseHeaders map[string]string, err error) {
	httpLog := GetHTTPLogger()
	logMeta := resolveHTTPLogMetadata(ctx)
	streamRequested := false
	if rawStream, ok := reqBody["stream"].(bool); ok {
		streamRequested = rawStream
	}

	jsonData, err := json.MarshalIndent(reqBody, "", "  ")
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setHeader(req)

	// Extract model/provider from request body for logging context.
	modelName, _ := reqBody["model"].(string)

	// Capture headers for logging.
	reqHeaders := make(map[string]string, len(req.Header))
	for k, v := range req.Header {
		reqHeaders[k] = strings.Join(v, ", ")
	}

	httpLog.LogRequestWithMetadata(method, url, reqHeaders, jsonData, modelName, "", logMeta)

	// Compact stdout summary (one line).
	fmt.Printf("[cs-ai] %s %s model=%s\n", method, url, modelName)

	startTime := time.Now()
	client := &http.Client{}
	resp, err := client.Do(req)
	durationMs := time.Since(startTime).Milliseconds()
	if err != nil {
		httpLog.LogResponseWithMetadata(0, nil, nil, durationMs, err.Error(), logMeta)
		httpLog.FlushPair()
		return nil, 0, nil, &APIRequestError{Err: fmt.Errorf("request failed: %w", err)}
	}
	defer resp.Body.Close()

	statusCode = resp.StatusCode
	responseHeaders = make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		responseHeaders[k] = strings.Join(v, ", ")
	}

	// Stream mode (SSE), used by OpenAI Codex transport and compatible providers.
	if streamRequested && statusCode < 400 {
		// Some providers/mocks may still return JSON (or omit SSE content-type)
		// despite stream=true. Fallback to plain JSON parsing in that case.
		contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
		if !strings.Contains(contentType, "text/event-stream") && !isCodexCreditExhausted(resp.Header) {
			bodyBytes, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				httpLog.LogResponseWithMetadata(statusCode, nil, nil, durationMs, readErr.Error(), logMeta)
				httpLog.FlushPair()
				return nil, statusCode, responseHeaders, &APIRequestError{StatusCode: statusCode, Err: fmt.Errorf("failed to read response body: %w", readErr)}
			}

			httpLog.LogResponseWithMetadata(statusCode, responseHeaders, bodyBytes, durationMs, "", logMeta)
			httpLog.FlushPair()

			if len(bodyBytes) > 0 {
				if unmarshalErr := json.Unmarshal(bodyBytes, &result); unmarshalErr != nil {
					return nil, statusCode, responseHeaders, &APIRequestError{StatusCode: statusCode, Body: string(bodyBytes), Err: fmt.Errorf("failed to parse JSON: %w", unmarshalErr)}
				}
			}
			if result == nil {
				result = map[string]interface{}{}
			}
			return result, statusCode, responseHeaders, nil
		}

		finalResponse, streamBody, streamErr := parseSSEFinalResponseWithObserver(resp.Body, observer)

		errMsg := ""
		if streamErr != nil {
			errMsg = streamErr.Error()
		}
		httpLog.LogResponseWithMetadata(statusCode, responseHeaders, streamBody, durationMs, errMsg, logMeta)
		httpLog.FlushPair()

		if streamErr != nil {
			errBody := strings.TrimSpace(string(streamBody))
			errStatus := statusCode
			if errStatus < 400 {
				errStatus = http.StatusBadGateway
			}
			if isCodexCreditExhausted(resp.Header) {
				errStatus = http.StatusTooManyRequests
				if errBody == "" {
					errBody = "insufficient_quota: codex credits exhausted"
				} else {
					lowerBody := strings.ToLower(errBody)
					if !strings.Contains(lowerBody, "insufficient_quota") &&
						!strings.Contains(lowerBody, "quota") &&
						!strings.Contains(lowerBody, "credit") &&
						!strings.Contains(lowerBody, "billing") {
						errBody = errBody + " | insufficient_quota: codex credits exhausted"
					}
				}
			}
			return nil, errStatus, responseHeaders, &APIRequestError{
				StatusCode: errStatus,
				Body:       errBody,
				Err:        streamErr,
			}
		}

		fmt.Printf("[cs-ai] <- %d (%dms) stream completed\n", statusCode, durationMs)
		return finalResponse, statusCode, responseHeaders, nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		httpLog.LogResponseWithMetadata(statusCode, nil, nil, durationMs, err.Error(), logMeta)
		httpLog.FlushPair()
		return nil, statusCode, responseHeaders, &APIRequestError{StatusCode: statusCode, Err: fmt.Errorf("failed to read response body: %w", err)}
	}

	errMsg := ""
	if statusCode >= 400 {
		errMsg = fmt.Sprintf("HTTP %d", statusCode)
	}
	httpLog.LogResponseWithMetadata(statusCode, responseHeaders, bodyBytes, durationMs, errMsg, logMeta)
	httpLog.FlushPair()

	// Compact stdout summary.
	fmt.Printf("[cs-ai] <- %d (%dms) %s\n", statusCode, durationMs, truncateForLog(string(bodyBytes), 200))

	if len(bodyBytes) > 0 {
		if err = json.Unmarshal(bodyBytes, &result); err != nil {
			if statusCode >= 400 {
				return nil, statusCode, responseHeaders, &APIRequestError{StatusCode: statusCode, Body: string(bodyBytes), Err: fmt.Errorf("failed to parse JSON: %w", err)}
			}
			return nil, statusCode, responseHeaders, fmt.Errorf("failed to parse JSON: %v, response: %s", err, string(bodyBytes))
		}
	}

	if statusCode >= 400 {
		bodyText := strings.TrimSpace(string(bodyBytes))
		if bodyText == "" && result != nil {
			if raw, marshalErr := json.Marshal(result); marshalErr == nil {
				bodyText = string(raw)
			}
		}
		return nil, statusCode, responseHeaders, &APIRequestError{StatusCode: statusCode, Body: bodyText}
	}

	if result == nil {
		result = map[string]interface{}{}
	}
	return result, statusCode, responseHeaders, nil
}

func parseSSEFinalResponse(reader io.Reader) (map[string]interface{}, []byte, error) {
	return parseSSEFinalResponseWithObserver(reader, nil)
}

func parseSSEFinalResponseWithObserver(reader io.Reader, observer RequestStreamObserver) (map[string]interface{}, []byte, error) {
	scanner := bufio.NewScanner(reader)
	// Increase scanner buffer for long SSE payload lines.
	scanner.Buffer(make([]byte, 0, 128*1024), 4*1024*1024)

	currentEvent := ""
	var completedResponse map[string]interface{}
	var failedPayload map[string]interface{}
	var lastPayload map[string]interface{}
	var chatCompletionAccumulator *chatCompletionStreamAccumulator
	responsesAccumulator := newResponsesStreamAccumulator()
	var responsesTextDelta strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		dataText := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if dataText == "" || dataText == "[DONE]" {
			continue
		}

		payload := map[string]interface{}{}
		if err := json.Unmarshal([]byte(dataText), &payload); err != nil {
			continue
		}
		lastPayload = payload

		eventType := currentEvent
		if typ, ok := payload["type"].(string); ok && strings.TrimSpace(typ) != "" {
			eventType = strings.TrimSpace(typ)
		}
		if eventType == "" {
			if objectType, ok := payload["object"].(string); ok && strings.TrimSpace(objectType) != "" {
				eventType = strings.TrimSpace(objectType)
			}
		}

		if isChatCompletionChunkPayload(payload) {
			if chatCompletionAccumulator == nil {
				chatCompletionAccumulator = newChatCompletionStreamAccumulator()
			}
			chatCompletionAccumulator.Absorb(payload)
		}
		if responsesAccumulator != nil {
			responsesAccumulator.Absorb(eventType, payload)
		}

		switch eventType {
		case "response.completed":
			if responseMap, ok := payload["response"].(map[string]interface{}); ok {
				completedResponse = responseMap
			}
		case "response.output_text.delta":
			if delta := toString(payload["delta"]); delta != "" {
				responsesTextDelta.WriteString(delta)
			}
		case "response.failed", "error":
			failedPayload = payload
		}

		if observer != nil {
			rawPayload, _ := json.Marshal(payload)
			observer(RequestStreamEvent{
				EventType: strings.TrimSpace(eventType),
				Payload:   payload,
				Raw:       rawPayload,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, []byte(err.Error()), err
	}

	if completedResponse != nil {
		if responsesAccumulator != nil {
			completedResponse = responsesAccumulator.ApplyTo(completedResponse)
		}
		if strings.TrimSpace(toString(completedResponse["output_text"])) == "" {
			if text := strings.TrimSpace(responsesTextDelta.String()); text != "" {
				completedResponse["output_text"] = text
			}
		}
		body, _ := json.Marshal(completedResponse)
		return completedResponse, body, nil
	}

	if failedPayload != nil {
		body, _ := json.Marshal(failedPayload)
		return nil, body, fmt.Errorf("stream response failed")
	}

	if chatCompletionAccumulator != nil && chatCompletionAccumulator.HasData() {
		finalResponse := chatCompletionAccumulator.FinalResponse()
		body, _ := json.Marshal(finalResponse)
		return finalResponse, body, nil
	}

	if responsesAccumulator != nil && responsesAccumulator.HasData() {
		finalResponse := responsesAccumulator.FinalResponse()
		if strings.TrimSpace(toString(finalResponse["output_text"])) == "" {
			if text := strings.TrimSpace(responsesTextDelta.String()); text != "" {
				finalResponse["output_text"] = text
			}
		}
		body, _ := json.Marshal(finalResponse)
		return finalResponse, body, nil
	}

	if lastPayload != nil {
		if responseMap, ok := lastPayload["response"].(map[string]interface{}); ok {
			body, _ := json.Marshal(responseMap)
			return responseMap, body, nil
		}
		body, _ := json.Marshal(lastPayload)
		return nil, body, fmt.Errorf("stream response missing completed payload")
	}

	return nil, nil, fmt.Errorf("stream response empty")
}

type chatCompletionToolCallState struct {
	ID        string
	Type      string
	Name      string
	Arguments strings.Builder
}

type responsesFunctionCallState struct {
	ID          string
	CallID      string
	Name        string
	OutputIndex int
	HasIndex    bool
	Arguments   strings.Builder
	inOrder     bool
}

type responsesStreamAccumulator struct {
	ResponseID            string
	Model                 string
	OutputText            strings.Builder
	functionCallByItemID  map[string]*responsesFunctionCallState
	functionCallByCallID  map[string]*responsesFunctionCallState
	functionCallByIndex   map[int]*responsesFunctionCallState
	functionCallOrder     []*responsesFunctionCallState
	lastFunctionCallState *responsesFunctionCallState
}

func newResponsesStreamAccumulator() *responsesStreamAccumulator {
	return &responsesStreamAccumulator{
		functionCallByItemID: make(map[string]*responsesFunctionCallState),
		functionCallByCallID: make(map[string]*responsesFunctionCallState),
		functionCallByIndex:  make(map[int]*responsesFunctionCallState),
	}
}

func (a *responsesStreamAccumulator) Absorb(eventType string, payload map[string]interface{}) {
	if a == nil || payload == nil {
		return
	}

	if responseMap, ok := payload["response"].(map[string]interface{}); ok && responseMap != nil {
		a.absorbResponseMetadata(responseMap)
	}
	if model := strings.TrimSpace(toString(payload["model"])); model != "" {
		a.Model = model
	}

	switch eventType {
	case "response.output_text.delta":
		if delta := toString(payload["delta"]); delta != "" {
			a.OutputText.WriteString(delta)
		}
	case "response.output_item.added":
		item, _ := payload["item"].(map[string]interface{})
		if item == nil {
			return
		}
		if strings.ToLower(strings.TrimSpace(toString(item["type"]))) != "function_call" {
			return
		}
		state := a.resolveFunctionCallState(payload, item)
		if state == nil {
			return
		}
		if id := strings.TrimSpace(toString(item["id"])); id != "" {
			state.ID = id
			a.functionCallByItemID[id] = state
		}
		if callID := strings.TrimSpace(toString(item["call_id"])); callID != "" {
			state.CallID = callID
			a.functionCallByCallID[callID] = state
		}
		if name := strings.TrimSpace(firstNonEmptyString(toString(item["name"]), toString(item["function"]))); name != "" {
			state.Name = name
		}
		if args := toString(item["arguments"]); args != "" {
			state.Arguments.WriteString(args)
		}
		a.lastFunctionCallState = state
	case "response.function_call_arguments.delta":
		delta := toString(payload["delta"])
		if delta == "" {
			return
		}
		state := a.resolveFunctionCallState(payload, nil)
		if state == nil {
			return
		}
		state.Arguments.WriteString(delta)
		a.lastFunctionCallState = state
	case "response.function_call_arguments.done":
		finalArgs := toString(payload["arguments"])
		if finalArgs == "" {
			return
		}
		state := a.resolveFunctionCallState(payload, nil)
		if state == nil {
			return
		}
		state.Arguments.Reset()
		state.Arguments.WriteString(finalArgs)
		a.lastFunctionCallState = state
	}
}

func (a *responsesStreamAccumulator) ApplyTo(response map[string]interface{}) map[string]interface{} {
	if a == nil {
		return response
	}
	if response == nil {
		response = map[string]interface{}{}
	}

	a.absorbResponseMetadata(response)

	if strings.TrimSpace(toString(response["id"])) == "" && strings.TrimSpace(a.ResponseID) != "" {
		response["id"] = a.ResponseID
	}
	if strings.TrimSpace(toString(response["model"])) == "" && strings.TrimSpace(a.Model) != "" {
		response["model"] = a.Model
	}

	if output, ok := response["output"].([]interface{}); !ok || len(output) == 0 {
		if finalOutput := a.finalOutput(); len(finalOutput) > 0 {
			response["output"] = finalOutput
		}
	}
	if strings.TrimSpace(toString(response["output_text"])) == "" {
		if text := strings.TrimSpace(a.OutputText.String()); text != "" {
			response["output_text"] = text
		}
	}

	return response
}

func (a *responsesStreamAccumulator) HasData() bool {
	if a == nil {
		return false
	}
	if strings.TrimSpace(a.ResponseID) != "" || strings.TrimSpace(a.Model) != "" {
		return true
	}
	if strings.TrimSpace(a.OutputText.String()) != "" {
		return true
	}
	return len(a.functionCallOrder) > 0
}

func (a *responsesStreamAccumulator) FinalResponse() map[string]interface{} {
	result := map[string]interface{}{}
	if strings.TrimSpace(a.ResponseID) != "" {
		result["id"] = a.ResponseID
	}
	if strings.TrimSpace(a.Model) != "" {
		result["model"] = a.Model
	}
	if finalOutput := a.finalOutput(); len(finalOutput) > 0 {
		result["output"] = finalOutput
	}
	if text := strings.TrimSpace(a.OutputText.String()); text != "" {
		result["output_text"] = text
	}
	return result
}

func (a *responsesStreamAccumulator) absorbResponseMetadata(response map[string]interface{}) {
	if a == nil || response == nil {
		return
	}
	if id := strings.TrimSpace(toString(response["id"])); id != "" {
		a.ResponseID = id
	}
	if model := strings.TrimSpace(toString(response["model"])); model != "" {
		a.Model = model
	}
}

func (a *responsesStreamAccumulator) resolveFunctionCallState(payload map[string]interface{}, item map[string]interface{}) *responsesFunctionCallState {
	if a == nil {
		return nil
	}

	itemID := ""
	callID := ""
	if item != nil {
		itemID = strings.TrimSpace(toString(item["id"]))
		callID = strings.TrimSpace(toString(item["call_id"]))
	}
	if itemID == "" {
		itemID = strings.TrimSpace(toString(payload["item_id"]))
	}
	if callID == "" {
		callID = strings.TrimSpace(toString(payload["call_id"]))
	}

	if itemID != "" {
		if state := a.functionCallByItemID[itemID]; state != nil {
			return state
		}
	}
	if callID != "" {
		if state := a.functionCallByCallID[callID]; state != nil {
			return state
		}
	}
	outputIndex, hasOutputIndex := parseSSEOutputIndex(payload["output_index"])
	if hasOutputIndex {
		if state := a.functionCallByIndex[outputIndex]; state != nil {
			return state
		}
	}
	if itemID == "" && callID == "" && !hasOutputIndex && a.lastFunctionCallState != nil {
		return a.lastFunctionCallState
	}

	state := &responsesFunctionCallState{}
	if !state.inOrder {
		state.inOrder = true
		a.functionCallOrder = append(a.functionCallOrder, state)
	}
	if itemID != "" {
		state.ID = itemID
		a.functionCallByItemID[itemID] = state
	}
	if callID != "" {
		state.CallID = callID
		a.functionCallByCallID[callID] = state
	}
	if hasOutputIndex {
		state.OutputIndex = outputIndex
		state.HasIndex = true
		a.functionCallByIndex[outputIndex] = state
	}
	return state
}

func parseSSEOutputIndex(raw interface{}) (int, bool) {
	switch idx := raw.(type) {
	case int:
		return idx, true
	case int32:
		return int(idx), true
	case int64:
		return int(idx), true
	case float64:
		return int(idx), true
	default:
		return 0, false
	}
}

func (a *responsesStreamAccumulator) finalOutput() []interface{} {
	if a == nil || len(a.functionCallOrder) == 0 {
		return nil
	}
	items := make([]interface{}, 0, len(a.functionCallOrder))
	for _, state := range a.functionCallOrder {
		if state == nil {
			continue
		}
		name := strings.TrimSpace(state.Name)
		if name == "" {
			continue
		}
		entry := map[string]interface{}{
			"type":      "function_call",
			"id":        firstNonEmptyString(strings.TrimSpace(state.ID), randomID("fc")),
			"call_id":   strings.TrimSpace(state.CallID),
			"name":      name,
			"arguments": state.Arguments.String(),
		}
		items = append(items, entry)
	}
	return items
}

type chatCompletionStreamAccumulator struct {
	ID       string
	Model    string
	Content  strings.Builder
	Usage    map[string]interface{}
	ToolCall map[int]*chatCompletionToolCallState
}

func newChatCompletionStreamAccumulator() *chatCompletionStreamAccumulator {
	return &chatCompletionStreamAccumulator{
		ToolCall: make(map[int]*chatCompletionToolCallState),
	}
}

func isChatCompletionChunkPayload(payload map[string]interface{}) bool {
	if payload == nil {
		return false
	}
	objectType := strings.TrimSpace(toString(payload["object"]))
	return objectType == "chat.completion.chunk"
}

func (a *chatCompletionStreamAccumulator) Absorb(payload map[string]interface{}) {
	if payload == nil {
		return
	}
	if id := strings.TrimSpace(toString(payload["id"])); id != "" {
		a.ID = id
	}
	if model := strings.TrimSpace(toString(payload["model"])); model != "" {
		a.Model = model
	}
	if usage, ok := payload["usage"].(map[string]interface{}); ok && usage != nil {
		a.Usage = usage
	}

	choices, _ := payload["choices"].([]interface{})
	for _, rawChoice := range choices {
		choiceMap, _ := rawChoice.(map[string]interface{})
		if choiceMap == nil {
			continue
		}
		deltaMap, _ := choiceMap["delta"].(map[string]interface{})
		if deltaMap == nil {
			continue
		}

		if content := toString(deltaMap["content"]); content != "" {
			a.Content.WriteString(content)
		}

		toolCalls, _ := deltaMap["tool_calls"].([]interface{})
		for _, rawToolCall := range toolCalls {
			toolCallMap, _ := rawToolCall.(map[string]interface{})
			if toolCallMap == nil {
				continue
			}

			index := 0
			switch idx := toolCallMap["index"].(type) {
			case int:
				index = idx
			case int32:
				index = int(idx)
			case int64:
				index = int(idx)
			case float64:
				index = int(idx)
			}

			state, exists := a.ToolCall[index]
			if !exists {
				state = &chatCompletionToolCallState{Type: "function"}
				a.ToolCall[index] = state
			}

			if id := strings.TrimSpace(toString(toolCallMap["id"])); id != "" {
				state.ID = id
			}
			if toolType := strings.TrimSpace(toString(toolCallMap["type"])); toolType != "" {
				state.Type = toolType
			}

			fnMap, _ := toolCallMap["function"].(map[string]interface{})
			if fnMap != nil {
				if name := strings.TrimSpace(toString(fnMap["name"])); name != "" {
					state.Name = name
				}
				if args := toString(fnMap["arguments"]); args != "" {
					state.Arguments.WriteString(args)
				}
			}
		}
	}
}

func (a *chatCompletionStreamAccumulator) HasData() bool {
	if a == nil {
		return false
	}
	if strings.TrimSpace(a.Content.String()) != "" {
		return true
	}
	if len(a.ToolCall) > 0 {
		return true
	}
	if strings.TrimSpace(a.ID) != "" || strings.TrimSpace(a.Model) != "" {
		return true
	}
	return a.Usage != nil
}

func (a *chatCompletionStreamAccumulator) FinalResponse() map[string]interface{} {
	message := map[string]interface{}{
		"role":    "assistant",
		"content": a.Content.String(),
	}

	toolCallEntries := a.finalToolCalls()
	finishReason := "stop"
	if len(toolCallEntries) > 0 {
		message["tool_calls"] = toolCallEntries
		finishReason = "tool_calls"
	}

	choice := map[string]interface{}{
		"index":         0,
		"finish_reason": finishReason,
		"message":       message,
	}

	response := map[string]interface{}{
		"choices": []interface{}{choice},
	}
	if strings.TrimSpace(a.ID) != "" {
		response["id"] = a.ID
	}
	if strings.TrimSpace(a.Model) != "" {
		response["model"] = a.Model
	}
	if a.Usage != nil {
		response["usage"] = a.Usage
	}
	return response
}

func (a *chatCompletionStreamAccumulator) finalToolCalls() []interface{} {
	if len(a.ToolCall) == 0 {
		return nil
	}

	indices := make([]int, 0, len(a.ToolCall))
	for idx := range a.ToolCall {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	entries := make([]interface{}, 0, len(indices))
	for _, idx := range indices {
		state := a.ToolCall[idx]
		if state == nil {
			continue
		}
		entry := map[string]interface{}{
			"id":   firstNonEmptyString(strings.TrimSpace(state.ID), randomID("call")),
			"type": firstNonEmptyString(strings.TrimSpace(state.Type), "function"),
			"function": map[string]interface{}{
				"name":      strings.TrimSpace(state.Name),
				"arguments": state.Arguments.String(),
			},
		}
		entries = append(entries, entry)
	}
	return entries
}

func isCodexCreditExhausted(headers http.Header) bool {
	if headers == nil {
		return false
	}
	hasCredits := strings.ToLower(strings.TrimSpace(headers.Get("X-Codex-Credits-Has-Credits")))
	unlimited := strings.ToLower(strings.TrimSpace(headers.Get("X-Codex-Credits-Unlimited")))
	return hasCredits == "false" && unlimited != "true"
}

// truncateForLog trims a string for compact stdout display.
func truncateForLog(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
