package cs_ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
)

// AuthProfilesExhaustedError is returned when all auth profiles for a provider
// are either in cooldown, disabled, or otherwise unavailable.  It is treated as
// failover-worthy so the model-level fallback can advance to the next model
// candidate.
type AuthProfilesExhaustedError struct {
	Provider string
	Cause    error
}

func (e *AuthProfilesExhaustedError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("all auth profiles exhausted for provider %s: %v", e.Provider, e.Cause)
	}
	return fmt.Sprintf("all auth profiles exhausted for provider %s", e.Provider)
}

func (e *AuthProfilesExhaustedError) Unwrap() error { return e.Cause }

func (c *CsAI) sendWithModelCandidates(
	ctx context.Context,
	sessionID string,
	roleMessage []map[string]interface{},
	function []map[string]interface{},
) (Message, error) {
	candidates := c.collectModelCandidates()
	if len(candidates) == 0 {
		return Message{}, fmt.Errorf("no model candidate configured")
	}

	var lastErr error
	for idx, candidate := range candidates {
		candidateProvider := resolveModelProvider(candidate)
		content, err := c.sendWithModel(ctx, sessionID, candidate, roleMessage, function)
		if err == nil {
			return content, nil
		}
		lastErr = err

		reason := classifyFailoverReason(err)
		if !isModelFailoverWorthy(candidateProvider, reason) {
			return Message{}, err
		}
		if idx == len(candidates)-1 {
			return Message{}, err
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("model request failed")
	}
	return Message{}, lastErr
}

func (c *CsAI) collectModelCandidates() []Modeler {
	candidates := make([]Modeler, 0, 1+len(c.options.ModelFallbacks))
	if c.Model != nil {
		candidates = append(candidates, c.Model)
	}
	for _, fallback := range c.options.ModelFallbacks {
		if fallback == nil {
			continue
		}
		candidates = append(candidates, fallback)
	}
	return candidates
}

func (c *CsAI) sendWithModel(
	ctx context.Context,
	sessionID string,
	modelCandidate Modeler,
	roleMessage []map[string]interface{},
	function []map[string]interface{},
) (content Message, err error) {
	provider := resolveModelProvider(modelCandidate)
	apiMode := resolveModelAPIMode(modelCandidate)

	// When no AuthManager is configured, fall back to the static API key and
	// attempt a single request (the original behaviour).
	if c.options.AuthManager == nil {
		return c.attemptModelRequest(ctx, sessionID, modelCandidate, provider, apiMode, strings.TrimSpace(c.ApiKey), "", roleMessage, function)
	}

	// --- Profile retry loop ---
	// Try each available auth profile for this provider before giving up on
	// the model candidate.  MarkFailure + ResolveAuth together rotate through
	// profiles because ResolveAuth skips profiles that are in cooldown /
	// disabled.
	triedProfiles := map[string]struct{}{}
	var lastErr error

	for attempt := 0; ; attempt++ {
		selection, authErr := c.options.AuthManager.ResolveAuth(ctx, sessionID, provider)
		if authErr != nil || selection == nil || strings.TrimSpace(selection.Token) == "" {
			// Distinguish two cases:
			//
			// (a) First attempt, no prior API error, and provider is NOT
			//     an OAuth provider → this provider has no registered
			//     profiles at all (e.g. DeepSeek uses a plain API key).
			//     Fall back to the static API key if available.
			//
			// (b) Provider is OAuth-based (e.g. openai-codex), OR we
			//     already tried at least one profile that failed → all
			//     profiles are exhausted.  Return the exhaustion error so
			//     the model-level fallback can advance to the next model
			//     candidate.  Do NOT try the static key — it likely
			//     belongs to a different provider and would produce a 401.
			if attempt == 0 && lastErr == nil && !isOAuthProvider(provider) {
				staticKey := strings.TrimSpace(c.ApiKey)
				if staticKey != "" {
					return c.attemptModelRequest(ctx, sessionID, modelCandidate, provider, apiMode, staticKey, "", roleMessage, function)
				}
			}
			cause := lastErr
			if cause == nil {
				cause = authErr
			}
			return Message{}, &AuthProfilesExhaustedError{Provider: provider, Cause: cause}
		}

		profileID := strings.TrimSpace(selection.ProfileID)
		authToken := strings.TrimSpace(selection.Token)

		// Prevent trying the same profile twice in one send cycle.
		if _, seen := triedProfiles[profileID]; seen {
			return Message{}, &AuthProfilesExhaustedError{Provider: provider, Cause: lastErr}
		}
		triedProfiles[profileID] = struct{}{}

		content, err = c.attemptModelRequest(ctx, sessionID, modelCandidate, provider, apiMode, authToken, profileID, roleMessage, function)
		if err == nil {
			return content, nil
		}
		lastErr = err

		// Retry with the next profile when policy allows it.
		// For OpenAI Codex we intentionally rotate on any error signal so each
		// account gets a chance before model fallback to DeepSeek.
		reason := classifyFailoverReason(err)
		if !isProfileRetryWorthy(provider, reason) {
			return Message{}, err
		}
		// MarkFailure was already called inside attemptModelRequest, so the
		// next ResolveAuth call will skip this profile.
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("model request failed after profile retries for provider %s", provider)
	}
	return Message{}, lastErr
}

// attemptModelRequest performs a single HTTP request against the model endpoint
// using the given auth token.  On failure it marks the profile (when present)
// so that subsequent ResolveAuth calls skip it.
func (c *CsAI) attemptModelRequest(
	ctx context.Context,
	sessionID string,
	modelCandidate Modeler,
	provider string,
	apiMode string,
	authToken string,
	profileID string,
	roleMessage []map[string]interface{},
	function []map[string]interface{},
) (Message, error) {
	if authToken == "" {
		return Message{}, fmt.Errorf("missing auth token for provider %s", provider)
	}

	reqBody := buildRequestBodyByAPIMode(apiMode, modelCandidate.ModelName(), roleMessage, function, c.options)
	result, statusCode, responseHeaders, requestErr := RequestDetailed(modelCandidate.ApiURL(), "POST", reqBody, func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer "+authToken)
		if apiMode == APIModeOpenAICodexResponses {
			// ChatGPT Codex backend requires account routing metadata in headers.
			if accountID := extractOpenAIChatGPTAccountIDFromJWT(authToken); accountID != "" {
				request.Header.Set("chatgpt-account-id", accountID)
			}
			request.Header.Set("OpenAI-Beta", "responses=experimental")
			request.Header.Set("originator", "pi")
		}
	})
	c.recordRateLimitSnapshot(ctx, provider, profileID, statusCode, responseHeaders)
	if requestErr != nil {
		reason := classifyFailoverReason(requestErr)
		if c.options.AuthManager != nil && profileID != "" {
			_ = c.options.AuthManager.MarkFailure(ctx, sessionID, provider, profileID, reason)
		}
		return Message{}, requestErr
	}

	var content Message
	var err error
	switch apiMode {
	case APIModeOpenAICodexResponses:
		content, err = MessageFromResponsesMap(result)
	default:
		content, err = MessageFromMap(result)
	}
	if err != nil {
		if c.options.AuthManager != nil && profileID != "" {
			_ = c.options.AuthManager.MarkFailure(ctx, sessionID, provider, profileID, AuthFailureReasonUnknown)
		}
		return Message{}, err
	}

	if c.options.AuthManager != nil && profileID != "" {
		_ = c.options.AuthManager.MarkSuccess(ctx, sessionID, provider, profileID)
	}

	return content, nil
}

func (c *CsAI) recordRateLimitSnapshot(ctx context.Context, provider string, profileID string, statusCode int, headers map[string]string) {
	if c == nil || c.options.AuthManager == nil || strings.TrimSpace(profileID) == "" {
		return
	}
	recorder, ok := c.options.AuthManager.(AuthRateLimitRecorder)
	if !ok {
		return
	}
	_ = recorder.RecordRateLimit(ctx, provider, profileID, statusCode, headers)
}

func buildRequestBodyByAPIMode(
	apiMode string,
	modelName string,
	roleMessage []map[string]interface{},
	function []map[string]interface{},
	options Options,
) map[string]interface{} {
	toolChoice := "auto"
	if !options.UseTool || len(function) == 0 {
		toolChoice = "none"
	}

	temperature := float32(0.2)
	if options.Temperature != 0 {
		temperature = options.Temperature
	}
	topP := float32(0.7)
	if options.TopP != 0 {
		topP = options.TopP
	}
	frequencyPenalty := float32(0.0)
	if options.FrequencyPenalty != 0 {
		frequencyPenalty = options.FrequencyPenalty
	}
	presencePenalty := float32(-1.5)
	if options.PresencePenalty != 0 {
		presencePenalty = options.PresencePenalty
	}

	if apiMode == APIModeOpenAICodexResponses {
		instructions, input := buildCodexResponsesInput(roleMessage)
		codexTools := buildCodexTools(function)

		return map[string]interface{}{
			"model":        modelName,
			"instructions": instructions,
			"input":        input,
			"tools":        codexTools,
			"tool_choice":  toolChoice,
			"stream":       true,
			"store":        false,
		}
	}

	return map[string]interface{}{
		"model":             modelName,
		"messages":          roleMessage,
		"frequency_penalty": frequencyPenalty,
		"max_tokens":        1200,
		"presence_penalty":  presencePenalty,
		"stop":              nil,
		"stream":            false,
		"stream_options":    nil,
		"temperature":       temperature,
		"top_p":             topP,
		"tools":             buildChatCompletionTools(function),
		"tool_choice":       toolChoice,
		"logprobs":          false,
		"top_logprobs":      nil,
	}
}

func buildCodexResponsesInput(roleMessage []map[string]interface{}) (instructions string, input []map[string]interface{}) {
	systemLines := make([]string, 0)
	input = make([]map[string]interface{}, 0, len(roleMessage))

	for _, msg := range roleMessage {
		role := strings.TrimSpace(strings.ToLower(toString(msg["role"])))
		content := strings.TrimSpace(toString(msg["content"]))

		switch role {
		case "system":
			if content != "" {
				systemLines = append(systemLines, content)
			}
		case "user":
			if content != "" {
				input = append(input, map[string]interface{}{
					"role":    "user",
					"content": content,
				})
			}
		case "assistant":
			toolCalls := parseToolCallsFromRaw(msg["tool_calls"])
			for _, toolCall := range toolCalls {
				name := strings.TrimSpace(toolCall.Function.Name)
				if name == "" {
					continue
				}
				callID := strings.TrimSpace(toolCall.Id)
				if callID == "" {
					callID = randomID("call")
				}
				args := strings.TrimSpace(toolCall.Function.Arguments)
				if args == "" {
					args = "{}"
				}
				input = append(input, map[string]interface{}{
					"type":      "function_call",
					"call_id":   callID,
					"name":      name,
					"arguments": args,
				})
			}

			if content != "" {
				input = append(input, map[string]interface{}{
					"role":    "assistant",
					"content": content,
				})
			}
		case "tool":
			callID := strings.TrimSpace(toString(msg["tool_call_id"]))
			if callID == "" {
				callID = randomID("call")
			}
			if content != "" {
				input = append(input, map[string]interface{}{
					"type":    "function_call_output",
					"call_id": callID,
					"output":  content,
				})
			}
		default:
			if content != "" {
				input = append(input, map[string]interface{}{
					"role":    "user",
					"content": content,
				})
			}
		}
	}

	instructions = strings.TrimSpace(strings.Join(systemLines, "\n"))
	if instructions == "" {
		instructions = "Kamu adalah asisten AI yang membantu."
	}
	return instructions, input
}

func parseToolCallsFromRaw(raw interface{}) []ToolCall {
	list, ok := raw.([]interface{})
	if !ok || len(list) == 0 {
		return nil
	}

	result := make([]ToolCall, 0, len(list))
	for _, item := range list {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		call := ToolCall{}
		call.Index = len(result)
		call.Id = strings.TrimSpace(toString(entry["id"]))
		call.Type = strings.TrimSpace(toString(entry["type"]))

		if fn, ok := entry["function"].(map[string]interface{}); ok {
			call.Function.Name = strings.TrimSpace(toString(fn["name"]))
			call.Function.Arguments = strings.TrimSpace(toString(fn["arguments"]))
		}
		if call.Function.Name == "" {
			call.Function.Name = strings.TrimSpace(toString(entry["name"]))
		}
		if call.Function.Arguments == "" {
			call.Function.Arguments = strings.TrimSpace(toString(entry["arguments"]))
		}

		if call.Function.Name == "" {
			continue
		}
		result = append(result, call)
	}
	return result
}

func buildCodexTools(function []map[string]interface{}) []map[string]interface{} {
	if len(function) == 0 {
		return []map[string]interface{}{}
	}

	tools := make([]map[string]interface{}, 0, len(function))
	for _, item := range function {
		if item == nil {
			continue
		}
		toolType := strings.TrimSpace(toString(item["type"]))
		if toolType == "" {
			toolType = "function"
		}

		fn, ok := item["function"].(map[string]interface{})
		if !ok {
			tools = append(tools, item)
			continue
		}

		name := strings.TrimSpace(toString(fn["name"]))
		if name == "" {
			continue
		}

		strict := extractInternalToolStrict(item)
		tool := map[string]interface{}{
			"type": toolType,
			"name": name,
		}
		if desc := strings.TrimSpace(toString(fn["description"])); desc != "" {
			tool["description"] = desc
		}
		tool["parameters"] = normalizeCodexToolParameters(fn["parameters"], strict)
		tool["strict"] = strict
		tools = append(tools, tool)
	}

	if len(tools) == 0 {
		return []map[string]interface{}{}
	}
	return tools
}

func buildChatCompletionTools(function []map[string]interface{}) []map[string]interface{} {
	if len(function) == 0 {
		return []map[string]interface{}{}
	}

	tools := make([]map[string]interface{}, 0, len(function))
	for _, item := range function {
		if item == nil {
			continue
		}
		toolType := strings.TrimSpace(toString(item["type"]))
		if toolType == "" {
			toolType = "function"
		}

		fn, ok := item["function"].(map[string]interface{})
		if !ok || fn == nil {
			continue
		}

		tool := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        strings.TrimSpace(toString(fn["name"])),
				"description": strings.TrimSpace(toString(fn["description"])),
				"parameters":  normalizeCodexToolParameters(fn["parameters"], false),
			},
		}
		if toolType != "" {
			tool["type"] = toolType
		}
		tools = append(tools, tool)
	}
	return tools
}

func extractInternalToolStrict(item map[string]interface{}) bool {
	if item == nil {
		return false
	}
	raw, exists := item["_csai_strict"]
	if !exists {
		return false
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func normalizeCodexToolParameters(raw interface{}, strict bool) map[string]interface{} {
	schema, ok := toMapStringInterface(raw)
	if !ok || schema == nil {
		return defaultCodexToolParameters()
	}
	if shouldAssumeObjectSchema(schema) {
		assumed := defaultCodexToolParameters()
		for key, value := range schema {
			assumed[key] = value
		}
		schema = assumed
	}
	normalizeCodexJSONSchema(schema, strict)
	return schema
}

func defaultCodexToolParameters() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"properties":           map[string]interface{}{},
		"required":             []interface{}{},
		"additionalProperties": false,
	}
}

func shouldAssumeObjectSchema(schema map[string]interface{}) bool {
	if len(schema) == 0 {
		return true
	}
	if schemaType := strings.TrimSpace(strings.ToLower(toString(schema["type"]))); schemaType != "" {
		return false
	}
	if _, ok := schema["$ref"]; ok {
		return false
	}
	for _, key := range []string{"anyOf", "oneOf", "allOf", "not", "items"} {
		if _, ok := schema[key]; ok {
			return false
		}
	}
	return true
}

func normalizeCodexJSONSchema(schema map[string]interface{}, strict bool) {
	if schema == nil {
		return
	}

	schemaType := strings.TrimSpace(strings.ToLower(toString(schema["type"])))
	_, hasProperties := schema["properties"]
	isObject := schemaType == "object" || hasProperties
	isArray := schemaType == "array"

	if isObject {
		if schemaType == "" {
			schema["type"] = "object"
		}
		props, ok := toMapStringInterface(schema["properties"])
		if !ok || props == nil {
			props = map[string]interface{}{}
		}
		for key, value := range props {
			props[key] = normalizeCodexJSONSchemaNode(value, strict)
		}
		schema["properties"] = props

		rawRequired, _ := toSliceInterface(schema["required"])
		originalRequired := normalizeRequiredKeys(rawRequired, props, false)
		if strict {
			requiredSet := make(map[string]struct{}, len(originalRequired))
			for _, item := range originalRequired {
				requiredSet[toString(item)] = struct{}{}
			}
			for key, value := range props {
				if _, exists := requiredSet[key]; exists {
					continue
				}
				props[key] = makeCodexSchemaNullable(value)
			}
		}
		required := normalizeRequiredKeys(rawRequired, props, strict)
		schema["required"] = required

		// Strict tool mode on OpenAI requires this field and it must be false.
		schema["additionalProperties"] = false
	}

	if isArray {
		// OpenAI function schema validation requires `items` for array types.
		if _, ok := schema["items"]; !ok || schema["items"] == nil {
			schema["items"] = map[string]interface{}{}
		}
	}

	if items, ok := schema["items"]; ok && items != nil {
		schema["items"] = normalizeCodexJSONSchemaNode(items, strict)
	}
	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		if values, ok := schema[key].([]interface{}); ok && values != nil {
			for i, item := range values {
				values[i] = normalizeCodexJSONSchemaNode(item, strict)
			}
			schema[key] = values
		}
	}
	if notValue, ok := schema["not"]; ok && notValue != nil {
		schema["not"] = normalizeCodexJSONSchemaNode(notValue, strict)
	}
	if defs, ok := schema["$defs"].(map[string]interface{}); ok && defs != nil {
		for key, item := range defs {
			defs[key] = normalizeCodexJSONSchemaNode(item, strict)
		}
		schema["$defs"] = defs
	}
	if defs, ok := schema["definitions"].(map[string]interface{}); ok && defs != nil {
		for key, item := range defs {
			defs[key] = normalizeCodexJSONSchemaNode(item, strict)
		}
		schema["definitions"] = defs
	}
}

func normalizeCodexJSONSchemaNode(value interface{}, strict bool) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		normalizeCodexJSONSchema(v, strict)
		return v
	case []interface{}:
		for i, item := range v {
			v[i] = normalizeCodexJSONSchemaNode(item, strict)
		}
		return v
	default:
		return value
	}
}

func cloneInterface(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		clone := make(map[string]interface{}, len(v))
		for key, item := range v {
			clone[key] = cloneInterface(item)
		}
		return clone
	case []interface{}:
		clone := make([]interface{}, len(v))
		for i, item := range v {
			clone[i] = cloneInterface(item)
		}
		return clone
	default:
		return value
	}
}

func toMapStringInterface(value interface{}) (map[string]interface{}, bool) {
	if value == nil {
		return nil, false
	}
	if direct, ok := value.(map[string]interface{}); ok {
		cloned, _ := cloneInterface(direct).(map[string]interface{})
		return cloned, true
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, false
	}
	return out, true
}

func toSliceInterface(value interface{}) ([]interface{}, bool) {
	if value == nil {
		return nil, false
	}
	if direct, ok := value.([]interface{}); ok {
		cloned, _ := cloneInterface(direct).([]interface{})
		return cloned, true
	}
	if direct, ok := value.([]string); ok {
		out := make([]interface{}, 0, len(direct))
		for _, item := range direct {
			out = append(out, item)
		}
		return out, true
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	out := []interface{}{}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, false
	}
	return out, true
}

func normalizeRequiredKeys(raw []interface{}, props map[string]interface{}, strict bool) []interface{} {
	if len(props) == 0 {
		return []interface{}{}
	}
	seen := map[string]struct{}{}
	normalized := make([]interface{}, 0, len(raw))
	for _, item := range raw {
		key := strings.TrimSpace(toString(item))
		if key == "" {
			continue
		}
		if _, exists := props[key]; !exists {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, key)
	}
	if strict {
		keys := make([]string, 0, len(props))
		for key := range props {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		normalized = make([]interface{}, 0, len(keys))
		for _, key := range keys {
			normalized = append(normalized, key)
		}
	}
	return normalized
}

func makeCodexSchemaNullable(value interface{}) interface{} {
	schema, ok := value.(map[string]interface{})
	if !ok || schema == nil {
		return value
	}

	rawType, exists := schema["type"]
	if !exists || rawType == nil {
		switch {
		case schema["properties"] != nil:
			schema["type"] = []interface{}{"object", "null"}
		case schema["items"] != nil:
			schema["type"] = []interface{}{"array", "null"}
		}
		return schema
	}

	switch typed := rawType.(type) {
	case string:
		normalized := strings.TrimSpace(typed)
		if normalized == "" || strings.EqualFold(normalized, "null") {
			return schema
		}
		schema["type"] = []interface{}{normalized, "null"}
	case []interface{}:
		hasNull := false
		normalized := make([]interface{}, 0, len(typed)+1)
		for _, item := range typed {
			if strings.EqualFold(strings.TrimSpace(toString(item)), "null") {
				hasNull = true
			}
			normalized = append(normalized, item)
		}
		if !hasNull {
			normalized = append(normalized, "null")
		}
		schema["type"] = normalized
	case []string:
		hasNull := false
		normalized := make([]interface{}, 0, len(typed)+1)
		for _, item := range typed {
			if strings.EqualFold(strings.TrimSpace(item), "null") {
				hasNull = true
			}
			normalized = append(normalized, item)
		}
		if !hasNull {
			normalized = append(normalized, "null")
		}
		schema["type"] = normalized
	}

	return schema
}

func resolveModelProvider(modelCandidate Modeler) string {
	if modelCandidate == nil {
		return "default"
	}
	if providerModel, ok := modelCandidate.(ProviderModeler); ok {
		provider := strings.TrimSpace(strings.ToLower(providerModel.ProviderName()))
		if provider != "" {
			return provider
		}
	}
	if modelName := strings.TrimSpace(strings.ToLower(modelCandidate.ModelName())); modelName != "" {
		if strings.Contains(modelName, "deepseek") {
			return "deepseek"
		}
		if strings.Contains(modelName, "gpt") || strings.Contains(modelName, "codex") {
			return "openai-codex"
		}
	}
	return "default"
}

// isOAuthProvider returns true for providers that authenticate via OAuth
// profiles managed by the AuthManager.  For these providers, the static ApiKey
// should never be used directly — it likely belongs to a different provider.
func isOAuthProvider(provider string) bool {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "openai-codex", "openai":
		return true
	default:
		return false
	}
}

func resolveModelAPIMode(modelCandidate Modeler) string {
	if modelCandidate == nil {
		return APIModeChatCompletions
	}
	if providerModel, ok := modelCandidate.(ProviderModeler); ok {
		mode := strings.TrimSpace(providerModel.APIMode())
		if mode != "" {
			return mode
		}
	}
	return APIModeChatCompletions
}

func classifyFailoverReason(err error) AuthFailureReason {
	if err == nil {
		return AuthFailureReasonUnknown
	}

	// All profiles exhausted → treat as "full" so model-level fallback
	// advances to the next candidate unless the root cause is a non-fallback
	// reason (e.g. auth).
	var exhaustedErr *AuthProfilesExhaustedError
	if errors.As(err, &exhaustedErr) {
		if exhaustedErr.Cause != nil {
			var nested *AuthProfilesExhaustedError
			if !errors.As(exhaustedErr.Cause, &nested) {
				causeReason := classifyFailoverReason(exhaustedErr.Cause)
				if causeReason != AuthFailureReasonUnknown {
					return causeReason
				}
			}
		}
		return AuthFailureReasonFull
	}

	var apiErr *APIRequestError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusTooManyRequests:
			// 429 can mean either transient rate-limit or durable billing/quota-full.
			// OpenAI often returns 429 with "insufficient_quota".
			if reason := classifyFailoverReasonFromBody(apiErr.Body); reason != AuthFailureReasonUnknown {
				return reason
			}
			return AuthFailureReasonRateLimit
		case http.StatusPaymentRequired:
			return AuthFailureReasonFull
		case http.StatusRequestTimeout, http.StatusGatewayTimeout:
			return AuthFailureReasonTimeout
		case http.StatusUnauthorized, http.StatusForbidden:
			return AuthFailureReasonAuth
		}
		if apiErr.IsTimeout() {
			return AuthFailureReasonTimeout
		}
		if reason := classifyFailoverReasonFromBody(apiErr.Body); reason != AuthFailureReasonUnknown {
			return reason
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return AuthFailureReasonTimeout
	}

	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "timeout"), strings.Contains(lower, "timed out"), strings.Contains(lower, "deadline"):
		return AuthFailureReasonTimeout
	case strings.Contains(lower, "rate limit"), strings.Contains(lower, "too many requests"):
		return AuthFailureReasonRateLimit
	case strings.Contains(lower, "insufficient"), strings.Contains(lower, "quota"), strings.Contains(lower, "credit"), strings.Contains(lower, "billing"), strings.Contains(lower, "full"):
		return AuthFailureReasonFull
	case strings.Contains(lower, "unauthorized"), strings.Contains(lower, "forbidden"), strings.Contains(lower, "token"):
		return AuthFailureReasonAuth
	default:
		return AuthFailureReasonUnknown
	}
}

func classifyFailoverReasonFromBody(body string) AuthFailureReason {
	lower := strings.ToLower(strings.TrimSpace(body))
	if lower == "" {
		return AuthFailureReasonUnknown
	}
	switch {
	case strings.Contains(lower, "not supported when using codex with a chatgpt account"),
		strings.Contains(lower, "model is not supported when using codex with a chatgpt account"),
		strings.Contains(lower, "model is not supported"),
		strings.Contains(lower, "unsupported model"),
		strings.Contains(lower, "model_not_supported"):
		// Account/model capability mismatch (e.g. free ChatGPT profile + gpt-5.4).
		// Treat as auth/profile-specific so runtime retries next profile.
		return AuthFailureReasonAuth
	// Check quota/full first so "insufficient_quota" won't be misclassified as rate-limit.
	case strings.Contains(lower, "insufficient_quota"),
		strings.Contains(lower, "insufficient"),
		strings.Contains(lower, "quota"),
		strings.Contains(lower, "credit"),
		strings.Contains(lower, "billing"),
		strings.Contains(lower, "full"):
		return AuthFailureReasonFull
	case strings.Contains(lower, "rate limit"),
		strings.Contains(lower, "too many requests"),
		strings.Contains(lower, "\"429\""),
		strings.Contains(lower, " 429"):
		return AuthFailureReasonRateLimit
	case strings.Contains(lower, "timeout"),
		strings.Contains(lower, "timed out"),
		strings.Contains(lower, "deadline"):
		return AuthFailureReasonTimeout
	case strings.Contains(lower, "unauthorized"),
		strings.Contains(lower, "forbidden"),
		strings.Contains(lower, "invalid_token"),
		strings.Contains(lower, "token"):
		return AuthFailureReasonAuth
	default:
		return AuthFailureReasonUnknown
	}
}

func isModelFailoverWorthy(provider string, reason AuthFailureReason) bool {
	if isOpenAICodexProvider(provider) {
		// Policy requested by consumer: when OpenAI fails for any reason,
		// continue to model fallback after profile rotation is exhausted.
		return true
	}

	switch reason {
	case AuthFailureReasonRateLimit, AuthFailureReasonFull, AuthFailureReasonTimeout:
		return true
	default:
		return false
	}
}

func isProfileRetryWorthy(provider string, reason AuthFailureReason) bool {
	if isOpenAICodexProvider(provider) {
		// Policy requested by consumer: rotate to next OpenAI account on any
		// error classification.
		return true
	}

	switch reason {
	case AuthFailureReasonRateLimit, AuthFailureReasonFull, AuthFailureReasonTimeout, AuthFailureReasonAuth:
		return true
	default:
		return false
	}
}

func isOpenAICodexProvider(provider string) bool {
	switch normalizeProviderName(provider) {
	case "openai-codex", "openai":
		return true
	default:
		return false
	}
}
