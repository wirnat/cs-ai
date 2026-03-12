package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type StructuredExecOptions struct {
	AllowedToolCodes         []string
	AdditionalIntents        []Intent
	AdditionalSystemMessages []string
	SessionMessages          []Message
	ToolSelector             RuntimeToolSelector
	ResponseBuilder          StructuredResponseBuilder
	ResponseVerifier         StructuredResponseVerifier
}

type StructuredExecResult struct {
	AssistantMessage string                `json:"assistant_message"`
	Visuals          []StructuredVisual    `json:"visuals,omitempty"`
	Citations        []StructuredCitation  `json:"citations,omitempty"`
	Confidence       StructuredConfidence  `json:"confidence"`
	Warnings         []string              `json:"warnings,omitempty"`
	ToolCalls        []StructuredToolTrace `json:"tool_calls,omitempty"`
	Usage            *DeepSeekUsage        `json:"usage,omitempty"`
	Decision         *StructuredDecision   `json:"decision,omitempty"`
	MessageID        int                   `json:"message_id,omitempty"`
}

type StructuredVisual struct {
	ID           string                   `json:"id,omitempty"`
	Kind         string                   `json:"kind,omitempty"`
	Title        string                   `json:"title,omitempty"`
	VegaLiteSpec map[string]interface{}   `json:"vega_lite_spec,omitempty"`
	TableData    []map[string]interface{} `json:"table_data,omitempty"`
}

type StructuredCitation struct {
	SourceType string `json:"source_type,omitempty"`
	SourceID   string `json:"source_id,omitempty"`
	Label      string `json:"label,omitempty"`
	URL        string `json:"url,omitempty"`
	Snippet    string `json:"snippet,omitempty"`
}

type StructuredConfidence struct {
	Score     float64 `json:"score"`
	Abstained bool    `json:"abstained"`
	Reason    string  `json:"reason,omitempty"`
}

type StructuredDecision struct {
	Intent       string   `json:"intent,omitempty"`
	Confidence   float64  `json:"confidence,omitempty"`
	FallbackUsed bool     `json:"fallback_used,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
}

type StructuredToolTrace struct {
	ToolName   string                 `json:"tool_name,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	Status     string                 `json:"status,omitempty"`
	Arguments  map[string]interface{} `json:"arguments,omitempty"`
	Output     interface{}            `json:"output,omitempty"`
	ErrorCode  string                 `json:"error_code,omitempty"`
}

type StructuredResponseBuildInput struct {
	SessionID                string
	UserMessage              UserMessage
	AllowedToolCodes         []string
	AdditionalIntents        []Intent
	AdditionalSystemMessages []string
	RawMessage               Message
	SessionMessages          []Message
	DeltaMessages            []Message
}

type RuntimeToolSelector interface {
	SelectTools(ctx context.Context, sessionID string, userMessage UserMessage, availableIntents []Intent, opts StructuredExecOptions) ([]string, error)
}

type RuntimeToolSelectorFunc func(ctx context.Context, sessionID string, userMessage UserMessage, availableIntents []Intent, opts StructuredExecOptions) ([]string, error)

func (f RuntimeToolSelectorFunc) SelectTools(ctx context.Context, sessionID string, userMessage UserMessage, availableIntents []Intent, opts StructuredExecOptions) ([]string, error) {
	return f(ctx, sessionID, userMessage, availableIntents, opts)
}

type StructuredResponseBuilder interface {
	Build(ctx context.Context, input StructuredResponseBuildInput) (StructuredExecResult, error)
}

type StructuredResponseBuilderFunc func(ctx context.Context, input StructuredResponseBuildInput) (StructuredExecResult, error)

func (f StructuredResponseBuilderFunc) Build(ctx context.Context, input StructuredResponseBuildInput) (StructuredExecResult, error) {
	return f(ctx, input)
}

type StructuredResponseVerifier interface {
	Verify(ctx context.Context, result StructuredExecResult) (StructuredExecResult, error)
}

type StructuredResponseVerifierFunc func(ctx context.Context, result StructuredExecResult) (StructuredExecResult, error)

func (f StructuredResponseVerifierFunc) Verify(ctx context.Context, result StructuredExecResult) (StructuredExecResult, error) {
	return f(ctx, result)
}

func (c *CsAI) ExecStructured(
	ctx context.Context,
	sessionID string,
	userMessage UserMessage,
	opts StructuredExecOptions,
) (StructuredExecResult, error) {
	startMessages, err := c.GetSessionMessages(sessionID)
	if err != nil && startMessages == nil {
		return StructuredExecResult{}, err
	}

	availableIntents := mergeIntentsByCode(c.intents, opts.AdditionalIntents)
	allowedToolCodes := normalizeAllowedToolCodes(opts.AllowedToolCodes)
	opts.SessionMessages = append([]Message(nil), startMessages...)
	if opts.ToolSelector != nil {
		selected, selectErr := opts.ToolSelector.SelectTools(ctx, sessionID, userMessage, availableIntents, opts)
		if selectErr != nil {
			return StructuredExecResult{}, selectErr
		}
		allowedToolCodes = normalizeAllowedToolCodes(selected)
	}

	rawMessage, execErr := c.ExecWithToolCodesAndIntents(
		ctx,
		sessionID,
		userMessage,
		allowedToolCodes,
		opts.AdditionalIntents,
		opts.AdditionalSystemMessages...,
	)
	if execErr != nil {
		return StructuredExecResult{}, execErr
	}

	sessionMessages, err := c.GetSessionMessages(sessionID)
	if err != nil {
		return StructuredExecResult{}, err
	}

	deltaMessages := diffMessagesByIndex(startMessages, sessionMessages)
	builder := opts.ResponseBuilder
	if builder == nil {
		builder = defaultStructuredResponseBuilder{}
	}

	result, err := builder.Build(ctx, StructuredResponseBuildInput{
		SessionID:                sessionID,
		UserMessage:              userMessage,
		AllowedToolCodes:         allowedToolCodes,
		AdditionalIntents:        opts.AdditionalIntents,
		AdditionalSystemMessages: opts.AdditionalSystemMessages,
		RawMessage:               rawMessage,
		SessionMessages:          sessionMessages,
		DeltaMessages:            deltaMessages,
	})
	if err != nil {
		return StructuredExecResult{}, err
	}

	if strings.TrimSpace(result.AssistantMessage) == "" {
		result.AssistantMessage = strings.TrimSpace(rawMessage.Content)
	}
	if result.Usage == nil {
		if rawMessage.AggregatedUsage != nil {
			usageCopy := *rawMessage.AggregatedUsage
			result.Usage = &usageCopy
		} else if rawMessage.Usage != nil {
			usageCopy := *rawMessage.Usage
			result.Usage = &usageCopy
		}
	}
	if result.Decision == nil {
		result.Decision = &StructuredDecision{
			AllowedTools: append([]string(nil), allowedToolCodes...),
		}
	} else if len(result.Decision.AllowedTools) == 0 {
		result.Decision.AllowedTools = append([]string(nil), allowedToolCodes...)
	}

	if opts.ResponseVerifier != nil {
		result, err = opts.ResponseVerifier.Verify(ctx, result)
		if err != nil {
			return StructuredExecResult{}, err
		}
	}

	if result.MessageID == 0 {
		result.MessageID = len(sessionMessages)
	}

	if persistErr := c.persistStructuredEnvelope(sessionID, sessionMessages, result); persistErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to persist structured envelope: %v", persistErr))
	}

	return result, nil
}

type defaultStructuredResponseBuilder struct{}

func (b defaultStructuredResponseBuilder) Build(_ context.Context, input StructuredResponseBuildInput) (StructuredExecResult, error) {
	traces := buildStructuredToolTraces(input.DeltaMessages)
	citations := buildDefaultCitations(traces)
	warnings := []string{}
	confidence := StructuredConfidence{
		Score: 0.35,
	}

	if len(traces) > 0 {
		confidence.Score = 0.78
	}
	if len(citations) == 0 {
		warnings = append(warnings, "response is not grounded by tool evidence")
		confidence.Score = 0.25
		confidence.Reason = "missing_tool_evidence"
	}

	return StructuredExecResult{
		AssistantMessage: strings.TrimSpace(input.RawMessage.Content),
		Citations:        citations,
		Confidence:       confidence,
		Warnings:         warnings,
		ToolCalls:        traces,
		Usage:            input.RawMessage.AggregatedUsage,
		Decision: &StructuredDecision{
			AllowedTools: append([]string(nil), input.AllowedToolCodes...),
		},
	}, nil
}

func normalizeAllowedToolCodes(codes []string) []string {
	if codes == nil {
		return nil
	}

	result := make([]string, 0, len(codes))
	seen := map[string]struct{}{}
	for _, code := range codes {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func diffMessagesByIndex(before []Message, after []Message) []Message {
	if len(after) <= len(before) {
		return []Message{}
	}
	result := make([]Message, 0, len(after)-len(before))
	result = append(result, after[len(before):]...)
	return result
}

func buildStructuredToolTraces(messages []Message) []StructuredToolTrace {
	callByID := map[string]ToolCall{}
	traces := make([]StructuredToolTrace, 0)

	for _, msg := range messages {
		if msg.Role == Assistant {
			for _, toolCall := range msg.ToolCalls {
				callByID[toolCall.Id] = toolCall
			}
			continue
		}
		if msg.Role != Tool {
			continue
		}

		trace := StructuredToolTrace{
			ToolCallID: msg.ToolCallID,
			Status:     "SUCCESS",
		}

		if toolCall, exists := callByID[msg.ToolCallID]; exists {
			trace.ToolName = toolCall.Function.Name
			trace.Arguments = parseJSONObject(toolCall.Function.Arguments)
		}

		if parsed := parseJSONValue(msg.Content); parsed != nil {
			trace.Output = parsed
			if payload, ok := parsed.(map[string]interface{}); ok {
				if status, ok := payload["status"].(string); ok && strings.TrimSpace(status) != "" {
					trace.Status = status
				}
				if rawErr, exists := payload["error"]; exists {
					if errMap, ok := rawErr.(map[string]interface{}); ok {
						if errorCode, ok := errMap["code"].(string); ok {
							trace.ErrorCode = errorCode
							if trace.Status == "SUCCESS" {
								trace.Status = "ERROR"
							}
						}
					}
				}
			}
		} else {
			trace.Output = map[string]interface{}{"content": msg.Content}
		}

		if trace.ToolName == "" {
			trace.ToolName = "unknown_tool"
		}
		traces = append(traces, trace)
	}

	return traces
}

func buildDefaultCitations(traces []StructuredToolTrace) []StructuredCitation {
	result := make([]StructuredCitation, 0, len(traces))
	for _, trace := range traces {
		if strings.EqualFold(trace.Status, "ERROR") {
			continue
		}
		result = append(result, StructuredCitation{
			SourceType: "tool_result",
			SourceID:   trace.ToolCallID,
			Label:      trace.ToolName,
		})
	}
	return result
}

func parseJSONObject(raw string) map[string]interface{} {
	parsed := parseJSONValue(raw)
	if result, ok := parsed.(map[string]interface{}); ok {
		return result
	}
	return nil
}

func parseJSONValue(raw string) interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	var result interface{}
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		return nil
	}
	return result
}

func (c *CsAI) persistStructuredEnvelope(sessionID string, sessionMessages []Message, result StructuredExecResult) error {
	if len(sessionMessages) == 0 {
		return nil
	}

	targetIndex := -1
	for i := len(sessionMessages) - 1; i >= 0; i-- {
		if sessionMessages[i].Role != Assistant {
			continue
		}
		if len(sessionMessages[i].ToolCalls) > 0 {
			continue
		}
		targetIndex = i
		break
	}
	if targetIndex == -1 {
		return nil
	}

	sessionMessages[targetIndex].Content = result.AssistantMessage
	sessionMessages[targetIndex].ContentMap = structuredResultToMap(result)
	_, err := c.SaveSessionMessages(sessionID, sessionMessages)
	return err
}

func structuredResultToMap(result StructuredExecResult) map[string]interface{} {
	payload, err := json.Marshal(result)
	if err != nil {
		return map[string]interface{}{
			"assistant_message": result.AssistantMessage,
		}
	}

	var structured map[string]interface{}
	if err := json.Unmarshal(payload, &structured); err != nil {
		return map[string]interface{}{
			"assistant_message": result.AssistantMessage,
		}
	}
	return structured
}
