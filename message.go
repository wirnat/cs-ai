package cs_ai

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

type Role string

// consts role
const (
	System    Role = "system"
	Developer Role = "developer"
	User      Role = "user"
	Assistant Role = "assistant"
	Tool      Role = "tool"
)

type Message struct {
	Content         string                     `json:"content" bson:"content"`
	ContentMap      interface{}                `json:"content_map,omitempty" bson:"content_map,omitempty"`         // Auto-populated when Content is JSON
	ID              int                        `json:"id,omitempty" bson:"id,omitempty" dynamodbav:"id,omitempty"` // 1-based incremental ID per session
	Name            string                     `json:"name" bson:"name"`
	Role            Role                       `json:"role" bson:"role"`
	ToolCalls       []ToolCall                 `json:"tool_calls" bson:"tool_calls"`
	ToolCallID      string                     `json:"tool_call_id" bson:"tool_call_id"`
	Model           string                     `json:"model,omitempty" bson:"model,omitempty"`
	ResponseID      string                     `json:"response_id,omitempty" bson:"response_id,omitempty"`
	Usage           *DeepSeekUsage             `json:"usage,omitempty" bson:"usage,omitempty"`
	AggregatedUsage *DeepSeekUsage             `json:"aggregated_usage,omitempty" bson:"aggregated_usage,omitempty"`
	Reasoning       *ResponseReasoningMetadata `json:"reasoning,omitempty" bson:"reasoning,omitempty"`
}

type DeepSeekPromptTokensDetails struct {
	CachedTokens int64 `json:"cached_tokens,omitempty" bson:"cached_tokens,omitempty"`
}

type DeepSeekUsage struct {
	PromptTokens          int64                       `json:"prompt_tokens,omitempty" bson:"prompt_tokens,omitempty"`
	CompletionTokens      int64                       `json:"completion_tokens,omitempty" bson:"completion_tokens,omitempty"`
	TotalTokens           int64                       `json:"total_tokens,omitempty" bson:"total_tokens,omitempty"`
	PromptTokensDetails   DeepSeekPromptTokensDetails `json:"prompt_tokens_details,omitempty" bson:"prompt_tokens_details,omitempty"`
	PromptCacheHitTokens  int64                       `json:"prompt_cache_hit_tokens,omitempty" bson:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens int64                       `json:"prompt_cache_miss_tokens,omitempty" bson:"prompt_cache_miss_tokens,omitempty"`
	Reasoning             ReasoningUsage              `json:"reasoning,omitempty" bson:"reasoning,omitempty"`
}

func (u DeepSeekUsage) IsZero() bool {
	return u.PromptTokens == 0 &&
		u.CompletionTokens == 0 &&
		u.TotalTokens == 0 &&
		u.PromptTokensDetails.CachedTokens == 0 &&
		u.PromptCacheHitTokens == 0 &&
		u.PromptCacheMissTokens == 0 &&
		u.Reasoning.Tokens == 0 &&
		u.Reasoning.CachedContextTokens == 0 &&
		u.Reasoning.PersistedContextTokens == 0
}

func (u DeepSeekUsage) Normalize() DeepSeekUsage {
	normalized := u
	normalized.PromptTokens = clampUsageToken(normalized.PromptTokens)
	normalized.CompletionTokens = clampUsageToken(normalized.CompletionTokens)
	normalized.TotalTokens = clampUsageToken(normalized.TotalTokens)
	normalized.PromptTokensDetails.CachedTokens = clampUsageToken(normalized.PromptTokensDetails.CachedTokens)
	normalized.PromptCacheHitTokens = clampUsageToken(normalized.PromptCacheHitTokens)
	normalized.PromptCacheMissTokens = clampUsageToken(normalized.PromptCacheMissTokens)
	normalized.Reasoning.Tokens = clampUsageToken(normalized.Reasoning.Tokens)
	normalized.Reasoning.CachedContextTokens = clampUsageToken(normalized.Reasoning.CachedContextTokens)
	normalized.Reasoning.PersistedContextTokens = clampUsageToken(normalized.Reasoning.PersistedContextTokens)

	if normalized.PromptCacheHitTokens == 0 && normalized.PromptTokensDetails.CachedTokens > 0 {
		normalized.PromptCacheHitTokens = normalized.PromptTokensDetails.CachedTokens
	}
	if normalized.PromptTokensDetails.CachedTokens == 0 && normalized.PromptCacheHitTokens > 0 {
		normalized.PromptTokensDetails.CachedTokens = normalized.PromptCacheHitTokens
	}

	if normalized.PromptCacheMissTokens == 0 && normalized.PromptTokens > 0 {
		derivedMiss := normalized.PromptTokens - normalized.PromptCacheHitTokens
		if derivedMiss < 0 {
			derivedMiss = 0
		}
		normalized.PromptCacheMissTokens = derivedMiss
	}

	if normalized.TotalTokens == 0 && (normalized.PromptTokens > 0 || normalized.CompletionTokens > 0) {
		normalized.TotalTokens = normalized.PromptTokens + normalized.CompletionTokens
	}

	return normalized
}

func (u DeepSeekUsage) Add(other DeepSeekUsage) DeepSeekUsage {
	a := u.Normalize()
	b := other.Normalize()
	return DeepSeekUsage{
		PromptTokens:     a.PromptTokens + b.PromptTokens,
		CompletionTokens: a.CompletionTokens + b.CompletionTokens,
		TotalTokens:      a.TotalTokens + b.TotalTokens,
		PromptTokensDetails: DeepSeekPromptTokensDetails{
			CachedTokens: a.PromptTokensDetails.CachedTokens + b.PromptTokensDetails.CachedTokens,
		},
		PromptCacheHitTokens:  a.PromptCacheHitTokens + b.PromptCacheHitTokens,
		PromptCacheMissTokens: a.PromptCacheMissTokens + b.PromptCacheMissTokens,
		Reasoning: ReasoningUsage{
			Tokens:                 a.Reasoning.Tokens + b.Reasoning.Tokens,
			CachedContextTokens:    a.Reasoning.CachedContextTokens + b.Reasoning.CachedContextTokens,
			PersistedContextTokens: a.Reasoning.PersistedContextTokens + b.Reasoning.PersistedContextTokens,
		},
	}
}

func clampUsageToken(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

// PrepareForStorage populates ContentMap if Content is valid JSON.
// Call this before saving to storage to enable JSON content as object.
func (m *Message) PrepareForStorage() {
	if m.Content == "" {
		return
	}
	trimmed := strings.TrimSpace(m.Content)
	// Check if content looks like JSON object or array
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		var parsed interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			m.ContentMap = parsed
		}
	}
}

// StripInternalMetadataForStorage removes response metadata that is useful for
// runtime/accounting but should not be persisted inside raw session transcripts.
func (m *Message) StripInternalMetadataForStorage() {
	m.ResponseID = ""
	m.AggregatedUsage = nil
	m.Reasoning = nil
}

func cloneMessagesForStorage(messages []Message) []Message {
	if len(messages) == 0 {
		return nil
	}

	cloned := append([]Message(nil), messages...)
	EnsureAutoIncrementMessageIDs(cloned)
	for i := range cloned {
		cloned[i].PrepareForStorage()
		cloned[i].StripInternalMetadataForStorage()
	}
	return cloned
}

// EnsureAutoIncrementMessageIDs guarantees sequential 1-based message IDs
// for the current order of messages in a session.
func EnsureAutoIncrementMessageIDs(messages []Message) (changed bool) {
	for i := range messages {
		expectedID := i + 1
		if messages[i].ID != expectedID {
			messages[i].ID = expectedID
			changed = true
		}
	}
	return changed
}

type ToolCall struct {
	Index    int    `json:"index"`
	Id       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func MessageFromMap(result map[string]interface{}) (content Message, err error) {
	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return Message{}, nil
	}

	jsonChoices, err := json.Marshal(choices[0].(map[string]interface{})["message"])
	if err != nil {
		fmt.Println("Error marshaling choices:", err)
		return Message{}, nil
	}

	err = json.Unmarshal(jsonChoices, &content)
	if err != nil {
		fmt.Println("Error unmarshaling choices:", err)
		return Message{}, nil
	}

	if modelName, ok := result["model"].(string); ok {
		content.Model = strings.TrimSpace(modelName)
	}
	if content.Role == Assistant {
		content.Content = sanitizeAssistantFinalMessage(content.Content)
	}
	content.Usage = parseDeepSeekUsage(result["usage"])

	return
}

func MessageFromResponsesMap(result map[string]interface{}) (Message, error) {
	content := Message{
		Role: Assistant,
	}
	if modelName, ok := result["model"].(string); ok {
		content.Model = strings.TrimSpace(modelName)
	}
	content.ResponseID = strings.TrimSpace(toString(result["id"]))
	content.Usage = parseResponsesUsage(result["usage"])
	content.Reasoning = parseResponsesReasoningMetadata(result, content.Usage)

	var texts []string
	directText := strings.TrimSpace(toString(result["output_text"]))

	rawOutput, _ := result["output"].([]interface{})
	for idx, item := range rawOutput {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		entryType := strings.ToLower(strings.TrimSpace(toString(entry["type"])))
		switch entryType {
		case "message":
			role := strings.ToLower(strings.TrimSpace(toString(entry["role"])))
			if role != "" && role != "assistant" {
				continue
			}
			if msgText := extractResponseMessageText(entry); msgText != "" {
				texts = append(texts, msgText)
			}
		case "function_call":
			toolCall := ToolCall{
				Index: idx,
				Id:    firstNonEmptyString(toString(entry["id"]), toString(entry["call_id"]), randomID("tool")),
				Type:  "function",
			}
			toolCall.Function.Name = firstNonEmptyString(toString(entry["name"]), toString(entry["function"]))
			toolCall.Function.Arguments = toString(entry["arguments"])
			if toolCall.Function.Name != "" {
				content.ToolCalls = append(content.ToolCalls, toolCall)
			}
		}
	}

	if len(texts) == 0 && directText != "" {
		texts = append(texts, directText)
	}
	if len(texts) > 0 {
		content.Content = sanitizeAssistantFinalMessage(strings.Join(texts, "\n"))
	}
	return content, nil
}

func extractResponseMessageText(entry map[string]interface{}) string {
	var lines []string

	contents, _ := entry["content"].([]interface{})
	for _, c := range contents {
		item, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		ctype := strings.ToLower(strings.TrimSpace(toString(item["type"])))
		if ctype == "output_text" || ctype == "text" {
			if text := strings.TrimSpace(toString(item["text"])); text != "" {
				lines = append(lines, text)
			}
		}
	}

	if len(lines) == 0 {
		if text := strings.TrimSpace(toString(entry["text"])); text != "" {
			lines = append(lines, text)
		}
	}
	return sanitizeAssistantFinalMessage(strings.Join(lines, "\n"))
}

func (m Message) MessageToMap() (map[string]interface{}, error) {
	// Konversi struct Message ke JSON string
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	// Konversi JSON string ke map[string]interface{}
	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	if err != nil {
		return nil, err
	}

	// Internal storage ID should not be sent to model APIs.
	delete(result, "id")
	delete(result, "usage")
	delete(result, "aggregated_usage")
	delete(result, "model")
	delete(result, "response_id")
	delete(result, "reasoning")

	return result, nil
}

type Messages []Message

func (m *Messages) Add(messages ...Message) {
	*m = append(*m, messages...)
}

func parseDeepSeekUsage(raw interface{}) *DeepSeekUsage {
	usageMap, ok := raw.(map[string]interface{})
	if !ok || usageMap == nil {
		return nil
	}

	usage := DeepSeekUsage{
		PromptTokens:          parseUsageInt64(usageMap["prompt_tokens"]),
		CompletionTokens:      parseUsageInt64(usageMap["completion_tokens"]),
		TotalTokens:           parseUsageInt64(usageMap["total_tokens"]),
		PromptCacheHitTokens:  parseUsageInt64(usageMap["prompt_cache_hit_tokens"]),
		PromptCacheMissTokens: parseUsageInt64(usageMap["prompt_cache_miss_tokens"]),
	}

	if details, ok := usageMap["prompt_tokens_details"].(map[string]interface{}); ok {
		usage.PromptTokensDetails.CachedTokens = parseUsageInt64(details["cached_tokens"])
	}

	normalized := usage.Normalize()
	if normalized.IsZero() {
		return nil
	}
	return &normalized
}

var assistantThinkingTagPattern = regexp.MustCompile(`(?is)<\s*(?:think|thinking|reasoning|analysis)\b[^>]*>.*?<\s*/\s*(?:think|thinking|reasoning|analysis)\s*>`)

func sanitizeAssistantFinalMessage(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	cleaned := assistantThinkingTagPattern.ReplaceAllString(trimmed, "")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return trimmed
	}
	return cleaned
}

func parseResponsesUsage(raw interface{}) *DeepSeekUsage {
	usageMap, ok := raw.(map[string]interface{})
	if !ok || usageMap == nil {
		return nil
	}

	usage := DeepSeekUsage{
		PromptTokens:     parseUsageInt64(usageMap["input_tokens"]),
		CompletionTokens: parseUsageInt64(usageMap["output_tokens"]),
		TotalTokens:      parseUsageInt64(usageMap["total_tokens"]),
	}
	if usage.PromptTokens == 0 {
		usage.PromptTokens = parseUsageInt64(usageMap["prompt_tokens"])
	}
	if usage.CompletionTokens == 0 {
		usage.CompletionTokens = parseUsageInt64(usageMap["completion_tokens"])
	}
	if details, ok := usageMap["input_tokens_details"].(map[string]interface{}); ok {
		usage.PromptTokensDetails.CachedTokens = parseUsageInt64(details["cached_tokens"])
		usage.Reasoning.CachedContextTokens = parseUsageInt64(details["cached_tokens"])
		usage.Reasoning.PersistedContextTokens = parseUsageInt64(details["persisted_tokens"])
	}
	if details, ok := usageMap["output_tokens_details"].(map[string]interface{}); ok {
		usage.Reasoning.Tokens = parseUsageInt64(details["reasoning_tokens"])
	}
	if usage.Reasoning.Tokens == 0 {
		usage.Reasoning.Tokens = parseUsageInt64(usageMap["reasoning_tokens"])
	}

	normalized := usage.Normalize()
	if normalized.IsZero() {
		return nil
	}
	return &normalized
}

func parseResponsesReasoningMetadata(result map[string]interface{}, usage *DeepSeekUsage) *ResponseReasoningMetadata {
	if result == nil {
		return nil
	}

	meta := &ResponseReasoningMetadata{
		PreviousResponseID: strings.TrimSpace(toString(result["previous_response_id"])),
	}
	if usage != nil {
		meta.Usage = usage.Reasoning
	}

	if reasoningMap, ok := result["reasoning"].(map[string]interface{}); ok {
		if effort := strings.TrimSpace(toString(reasoningMap["effort"])); effort != "" {
			meta.EffortUsed = effort
		}
		if summary := extractReasoningSummary(reasoningMap["summary"]); summary != "" {
			meta.Summaries = []ReasoningSummary{{Text: summary}}
			meta.SummaryText = summary
		}
	}

	rawOutput, _ := result["output"].([]interface{})
	for _, item := range rawOutput {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(toString(entry["type"]))) != "reasoning" {
			continue
		}
		meta.ItemsPresent = true
		if effort := strings.TrimSpace(toString(entry["effort"])); effort != "" && meta.EffortUsed == "" {
			meta.EffortUsed = effort
		}
		if summary := extractReasoningSummary(entry["summary"]); summary != "" {
			meta.Summaries = append(meta.Summaries, ReasoningSummary{Text: summary})
		}
	}

	if meta.SummaryText == "" && len(meta.Summaries) > 0 {
		lines := make([]string, 0, len(meta.Summaries))
		for _, item := range meta.Summaries {
			if text := strings.TrimSpace(item.Text); text != "" {
				lines = append(lines, text)
			}
		}
		meta.SummaryText = strings.TrimSpace(strings.Join(lines, "\n"))
	}

	if meta.SummaryText == "" && meta.EffortUsed == "" && !meta.ItemsPresent && meta.PreviousResponseID == "" && meta.Usage.Tokens == 0 && meta.Usage.CachedContextTokens == 0 && meta.Usage.PersistedContextTokens == 0 {
		return nil
	}
	return meta
}

func extractReasoningSummary(raw interface{}) string {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case []interface{}:
		lines := make([]string, 0, len(v))
		for _, item := range v {
			switch typed := item.(type) {
			case string:
				if text := strings.TrimSpace(typed); text != "" {
					lines = append(lines, text)
				}
			case map[string]interface{}:
				if text := firstNonEmptyString(
					toString(typed["text"]),
					toString(typed["summary_text"]),
					toString(typed["content"]),
				); strings.TrimSpace(text) != "" {
					lines = append(lines, strings.TrimSpace(text))
				}
			}
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	case map[string]interface{}:
		return firstNonEmptyString(
			strings.TrimSpace(toString(v["text"])),
			strings.TrimSpace(toString(v["summary_text"])),
			strings.TrimSpace(toString(v["content"])),
		)
	default:
		return ""
	}
}

func firstNonEmptyString(candidates ...string) string {
	for _, item := range candidates {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseUsageInt64(raw interface{}) int64 {
	switch v := raw.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case float32:
		return int64(math.Round(float64(v)))
	case float64:
		return int64(math.Round(v))
	case json.Number:
		if parsedInt, err := v.Int64(); err == nil {
			return parsedInt
		}
		if parsedFloat, err := v.Float64(); err == nil {
			return int64(math.Round(parsedFloat))
		}
		return 0
	default:
		return 0
	}
}
