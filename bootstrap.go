package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	defaultBootstrapToolCallIDPrefix = "bootstrap-fc"
	defaultBootstrapMaxDepth         = 4
	defaultBootstrapMaxArrayItems    = 20
	defaultBootstrapMaxObjectKeys    = 30
	defaultBootstrapMaxStringChars   = 280
	defaultBootstrapMaxPayloadBytes  = 12000
)

type bootstrapPreparedCall struct {
	intentCode   string
	rawArguments string
	toolCallID   string
}

func (c *CsAI) maybeApplyFirstTurnBootstrap(
	ctx context.Context,
	sessionID string,
	messages *Messages,
	userMessage UserMessage,
	executionState *intentExecutionState,
) error {
	if messages == nil {
		return nil
	}

	if !shouldBootstrapFirstTurn(c.options.FirstTurnBootstrap, sessionID, *messages) {
		return nil
	}

	options := normalizeFirstTurnBootstrapOptions(*c.options.FirstTurnBootstrap)
	options.IntentCalls = filterBootstrapIntentCalls(options.IntentCalls, executionState.IntentsByCode)
	if len(options.IntentCalls) == 0 {
		return nil
	}

	assistantMessage, calls, err := buildSyntheticBootstrapAssistant(options)
	if err != nil {
		return err
	}

	toolMessages, err := c.executeBootstrapCalls(ctx, sessionID, userMessage, calls, options, executionState)
	if err != nil {
		return err
	}

	messages.Add(assistantMessage)
	messages.Add(toolMessages...)

	return nil
}

func shouldBootstrapFirstTurn(options *FirstTurnBootstrapOptions, sessionID string, messages Messages) bool {
	if options == nil {
		return false
	}
	if len(options.IntentCalls) == 0 {
		return false
	}
	if options.RequireSessionID && strings.TrimSpace(sessionID) == "" {
		return false
	}
	if hasUserMessage(messages) {
		return false
	}
	return true
}

func hasUserMessage(messages Messages) bool {
	for _, message := range messages {
		if message.Role == User {
			return true
		}
	}
	return false
}

func filterBootstrapIntentCalls(intentCalls []BootstrapIntentCall, availableIntents map[string]Intent) []BootstrapIntentCall {
	if len(intentCalls) == 0 || len(availableIntents) == 0 {
		return []BootstrapIntentCall{}
	}

	availableSet := make(map[string]struct{}, len(availableIntents))
	for code := range availableIntents {
		availableSet[strings.ToLower(strings.TrimSpace(code))] = struct{}{}
	}

	result := make([]BootstrapIntentCall, 0, len(intentCalls))
	for _, intentCall := range intentCalls {
		intentCode := strings.ToLower(strings.TrimSpace(intentCall.IntentCode))
		if intentCode == "" {
			continue
		}
		if _, exists := availableSet[intentCode]; !exists {
			continue
		}
		result = append(result, intentCall)
	}

	return result
}

func buildSyntheticBootstrapAssistant(options FirstTurnBootstrapOptions) (Message, []bootstrapPreparedCall, error) {
	toolCalls := make([]ToolCall, 0, len(options.IntentCalls))
	preparedCalls := make([]bootstrapPreparedCall, 0, len(options.IntentCalls))

	for idx, intentCall := range options.IntentCalls {
		intentCode := strings.TrimSpace(intentCall.IntentCode)
		if intentCode == "" {
			return Message{}, nil, fmt.Errorf("bootstrap intent code cannot be empty at index %d", idx)
		}

		args := intentCall.Params
		if args == nil {
			args = map[string]interface{}{}
		}

		argsBytes, err := json.Marshal(args)
		if err != nil {
			return Message{}, nil, fmt.Errorf("failed to marshal bootstrap arguments for %s: %w", intentCode, err)
		}

		toolCallID := fmt.Sprintf(
			"%s-%d-%s",
			options.ToolCallIDPrefix,
			idx+1,
			sanitizeIntentCode(intentCode),
		)

		toolCall := ToolCall{
			Index: idx,
			Id:    toolCallID,
			Type:  "function",
		}
		toolCall.Function.Name = intentCode
		toolCall.Function.Arguments = string(argsBytes)

		toolCalls = append(toolCalls, toolCall)
		preparedCalls = append(preparedCalls, bootstrapPreparedCall{
			intentCode:   intentCode,
			rawArguments: string(argsBytes),
			toolCallID:   toolCallID,
		})
	}

	return Message{
		Role:      Assistant,
		Content:   "",
		ToolCalls: toolCalls,
	}, preparedCalls, nil
}

func sanitizeIntentCode(intentCode string) string {
	intentCode = strings.TrimSpace(intentCode)
	if intentCode == "" {
		return "intent"
	}

	var b strings.Builder
	for _, r := range intentCode {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('-')
	}

	sanitized := strings.Trim(b.String(), "-")
	if sanitized == "" {
		return "intent"
	}
	return sanitized
}

func (c *CsAI) executeBootstrapCalls(
	ctx context.Context,
	sessionID string,
	userMessage UserMessage,
	calls []bootstrapPreparedCall,
	options FirstTurnBootstrapOptions,
	executionState *intentExecutionState,
) (Messages, error) {
	toolMessages := make(Messages, 0, len(calls))
	for _, call := range calls {
		data, _, execErr := c.executeIntentHandler(
			ctx,
			sessionID,
			userMessage,
			call.intentCode,
			call.rawArguments,
			executionState,
		)

		payload := map[string]interface{}{
			"source":      "first_turn_bootstrap",
			"intent_code": call.intentCode,
		}

		if execErr != nil {
			if options.FailurePolicy == BootstrapFailureStrict {
				return nil, fmt.Errorf("first-turn bootstrap failed for %s: %w", call.intentCode, execErr)
			}

			errorCode := "bootstrap_execution_error"
			errorMessage := execErr.Error()
			var intentErr *intentExecutionError
			if errorsAsIntentExecution(execErr, &intentErr) {
				errorCode = intentErr.Code
				if intentErr.Cause != nil {
					errorMessage = intentErr.Cause.Error()
				}
			}

			payload["status"] = "ERROR"
			payload["summary"] = fmt.Sprintf("bootstrap intent %s gagal", call.intentCode)
			payload["error"] = map[string]interface{}{
				"code":    errorCode,
				"message": errorMessage,
			}
		} else {
			payload["status"] = "SUCCESS"
			payload["summary"] = extractBootstrapSummary(call.intentCode, data)
			payload["data"] = compactBootstrapData(data, options.Compaction)
		}

		contentBytes, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			if options.FailurePolicy == BootstrapFailureStrict {
				return nil, fmt.Errorf("failed to marshal bootstrap payload for %s: %w", call.intentCode, marshalErr)
			}
			contentBytes = []byte(`{"source":"first_turn_bootstrap","status":"ERROR","summary":"bootstrap payload marshal gagal"}`)
		}

		toolMessages.Add(Message{
			Role:       Tool,
			ToolCallID: call.toolCallID,
			Content:    string(contentBytes),
		})
	}

	return toolMessages, nil
}

func extractBootstrapSummary(intentCode string, data interface{}) string {
	if payload, ok := data.(map[string]interface{}); ok {
		if message, ok := payload["message"].(string); ok {
			message = strings.TrimSpace(message)
			if message != "" {
				return message
			}
		}
	}
	return fmt.Sprintf("bootstrap intent %s berhasil", intentCode)
}

func compactBootstrapData(data interface{}, compaction BootstrapCompactionOptions) interface{} {
	normalized := normalizeBootstrapCompactionOptions(compaction)

	normalizedData := data
	if raw, err := json.Marshal(data); err == nil {
		_ = json.Unmarshal(raw, &normalizedData)
	}

	compacted := compactBootstrapValue(normalizedData, 0, normalized)
	compactedBytes, err := json.Marshal(compacted)
	if err == nil && len(compactedBytes) > normalized.MaxPayloadBytes {
		previewLimit := normalized.MaxPayloadBytes
		if previewLimit > len(compactedBytes) {
			previewLimit = len(compactedBytes)
		}
		return map[string]interface{}{
			"truncated":         true,
			"reason":            "max_payload_bytes",
			"max_payload_bytes": normalized.MaxPayloadBytes,
			"truncated_preview": string(compactedBytes[:previewLimit]),
		}
	}

	return compacted
}

func compactBootstrapValue(data interface{}, depth int, options BootstrapCompactionOptions) interface{} {
	if depth >= options.MaxDepth {
		return map[string]interface{}{
			"truncated": true,
			"reason":    "max_depth",
		}
	}

	switch value := data.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		limit := options.MaxObjectKeys
		if limit > len(keys) {
			limit = len(keys)
		}

		result := make(map[string]interface{}, limit+1)
		for _, key := range keys[:limit] {
			result[key] = compactBootstrapValue(value[key], depth+1, options)
		}

		if len(keys) > limit {
			result["_truncated_keys"] = len(keys) - limit
		}

		return result
	case []interface{}:
		limit := options.MaxArrayItems
		if limit > len(value) {
			limit = len(value)
		}

		result := make([]interface{}, 0, limit+1)
		for i := 0; i < limit; i++ {
			result = append(result, compactBootstrapValue(value[i], depth+1, options))
		}

		if len(value) > limit {
			result = append(result, map[string]interface{}{
				"truncated":       true,
				"remaining_items": len(value) - limit,
			})
		}

		return result
	case string:
		return truncateBootstrapString(value, options.MaxStringChars)
	default:
		return value
	}
}

func truncateBootstrapString(value string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = defaultBootstrapMaxStringChars
	}
	if len(value) <= maxChars {
		return value
	}
	if maxChars <= 3 {
		return value[:maxChars]
	}
	return value[:maxChars-3] + "..."
}

func normalizeFirstTurnBootstrapOptions(options FirstTurnBootstrapOptions) FirstTurnBootstrapOptions {
	normalized := options
	if normalized.FailurePolicy == "" {
		normalized.FailurePolicy = BootstrapFailureBestEffort
	}
	if normalized.ToolCallIDPrefix == "" {
		normalized.ToolCallIDPrefix = defaultBootstrapToolCallIDPrefix
	}
	normalized.Compaction = normalizeBootstrapCompactionOptions(normalized.Compaction)
	return normalized
}

func normalizeBootstrapCompactionOptions(options BootstrapCompactionOptions) BootstrapCompactionOptions {
	normalized := options
	if normalized.MaxDepth <= 0 {
		normalized.MaxDepth = defaultBootstrapMaxDepth
	}
	if normalized.MaxArrayItems <= 0 {
		normalized.MaxArrayItems = defaultBootstrapMaxArrayItems
	}
	if normalized.MaxObjectKeys <= 0 {
		normalized.MaxObjectKeys = defaultBootstrapMaxObjectKeys
	}
	if normalized.MaxStringChars <= 0 {
		normalized.MaxStringChars = defaultBootstrapMaxStringChars
	}
	if normalized.MaxPayloadBytes <= 0 {
		normalized.MaxPayloadBytes = defaultBootstrapMaxPayloadBytes
	}
	return normalized
}
