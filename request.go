package cs_ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type APIRequestError struct {
	StatusCode int
	Body       string
	Err        error
}

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
	result, _, err = RequestDetailed(url, method, reqBody, setHeader)
	return result, err
}

func RequestDetailed(url string, method string, reqBody map[string]interface{}, setHeader func(*http.Request)) (result map[string]interface{}, statusCode int, err error) {
	httpLog := GetHTTPLogger()
	streamRequested := false
	if rawStream, ok := reqBody["stream"].(bool); ok {
		streamRequested = rawStream
	}

	jsonData, err := json.MarshalIndent(reqBody, "", "  ")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request body: %v", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %v", err)
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

	httpLog.LogRequest(method, url, reqHeaders, jsonData, modelName, "")

	// Compact stdout summary (one line).
	fmt.Printf("[cs-ai] %s %s model=%s\n", method, url, modelName)

	startTime := time.Now()
	client := &http.Client{}
	resp, err := client.Do(req)
	durationMs := time.Since(startTime).Milliseconds()
	if err != nil {
		httpLog.LogResponse(0, nil, nil, durationMs, err.Error())
		httpLog.FlushPair()
		return nil, 0, &APIRequestError{Err: fmt.Errorf("request failed: %w", err)}
	}
	defer resp.Body.Close()

	statusCode = resp.StatusCode

	// Stream mode (SSE), used by OpenAI Codex transport on chatgpt.com.
	if streamRequested && statusCode < 400 {
		// Some providers might still return JSON despite stream=true.
		contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
		if strings.Contains(contentType, "application/json") {
			bodyBytes, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				httpLog.LogResponse(statusCode, nil, nil, durationMs, readErr.Error())
				httpLog.FlushPair()
				return nil, statusCode, &APIRequestError{StatusCode: statusCode, Err: fmt.Errorf("failed to read response body: %w", readErr)}
			}

			respHeaders := make(map[string]string, len(resp.Header))
			for k, v := range resp.Header {
				respHeaders[k] = strings.Join(v, ", ")
			}
			httpLog.LogResponse(statusCode, respHeaders, bodyBytes, durationMs, "")
			httpLog.FlushPair()

			if len(bodyBytes) > 0 {
				if unmarshalErr := json.Unmarshal(bodyBytes, &result); unmarshalErr != nil {
					return nil, statusCode, &APIRequestError{StatusCode: statusCode, Body: string(bodyBytes), Err: fmt.Errorf("failed to parse JSON: %w", unmarshalErr)}
				}
			}
			if result == nil {
				result = map[string]interface{}{}
			}
			return result, statusCode, nil
		}

		finalResponse, streamBody, streamErr := parseSSEFinalResponse(resp.Body)

		respHeaders := make(map[string]string, len(resp.Header))
		for k, v := range resp.Header {
			respHeaders[k] = strings.Join(v, ", ")
		}

		errMsg := ""
		if streamErr != nil {
			errMsg = streamErr.Error()
		}
		httpLog.LogResponse(statusCode, respHeaders, streamBody, durationMs, errMsg)
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
			return nil, errStatus, &APIRequestError{
				StatusCode: errStatus,
				Body:       errBody,
				Err:        streamErr,
			}
		}

		fmt.Printf("[cs-ai] <- %d (%dms) stream completed\n", statusCode, durationMs)
		return finalResponse, statusCode, nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		httpLog.LogResponse(statusCode, nil, nil, durationMs, err.Error())
		httpLog.FlushPair()
		return nil, statusCode, &APIRequestError{StatusCode: statusCode, Err: fmt.Errorf("failed to read response body: %w", err)}
	}

	// Capture response headers for logging.
	respHeaders := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		respHeaders[k] = strings.Join(v, ", ")
	}

	errMsg := ""
	if statusCode >= 400 {
		errMsg = fmt.Sprintf("HTTP %d", statusCode)
	}
	httpLog.LogResponse(statusCode, respHeaders, bodyBytes, durationMs, errMsg)
	httpLog.FlushPair()

	// Compact stdout summary.
	fmt.Printf("[cs-ai] <- %d (%dms) %s\n", statusCode, durationMs, truncateForLog(string(bodyBytes), 200))

	if len(bodyBytes) > 0 {
		if err = json.Unmarshal(bodyBytes, &result); err != nil {
			if statusCode >= 400 {
				return nil, statusCode, &APIRequestError{StatusCode: statusCode, Body: string(bodyBytes), Err: fmt.Errorf("failed to parse JSON: %w", err)}
			}
			return nil, statusCode, fmt.Errorf("failed to parse JSON: %v, response: %s", err, string(bodyBytes))
		}
	}

	if statusCode >= 400 {
		bodyText := strings.TrimSpace(string(bodyBytes))
		if bodyText == "" && result != nil {
			if raw, marshalErr := json.Marshal(result); marshalErr == nil {
				bodyText = string(raw)
			}
		}
		return nil, statusCode, &APIRequestError{StatusCode: statusCode, Body: bodyText}
	}

	if result == nil {
		result = map[string]interface{}{}
	}
	return result, statusCode, nil
}

func parseSSEFinalResponse(reader io.Reader) (map[string]interface{}, []byte, error) {
	scanner := bufio.NewScanner(reader)
	// Increase scanner buffer for long SSE payload lines.
	scanner.Buffer(make([]byte, 0, 128*1024), 4*1024*1024)

	currentEvent := ""
	var completedResponse map[string]interface{}
	var failedPayload map[string]interface{}
	var lastPayload map[string]interface{}

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

		switch eventType {
		case "response.completed":
			if responseMap, ok := payload["response"].(map[string]interface{}); ok {
				completedResponse = responseMap
			}
		case "response.failed", "error":
			failedPayload = payload
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, []byte(err.Error()), err
	}

	if completedResponse != nil {
		body, _ := json.Marshal(completedResponse)
		return completedResponse, body, nil
	}

	if failedPayload != nil {
		body, _ := json.Marshal(failedPayload)
		return nil, body, fmt.Errorf("stream response failed")
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
