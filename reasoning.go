package cs_ai

import (
	"context"
	"encoding/json"
	"strings"
)

const reasoningRuntimeStateKey = "_csai_reasoning_runtime"

type ReasoningSummary struct {
	Text string `json:"text,omitempty" bson:"text,omitempty"`
}

type ReasoningUsage struct {
	Tokens                 int64 `json:"tokens,omitempty" bson:"tokens,omitempty"`
	CachedContextTokens    int64 `json:"cached_context_tokens,omitempty" bson:"cached_context_tokens,omitempty"`
	PersistedContextTokens int64 `json:"persisted_context_tokens,omitempty" bson:"persisted_context_tokens,omitempty"`
}

type ResponseReasoningMetadata struct {
	Summaries          []ReasoningSummary      `json:"summaries,omitempty" bson:"summaries,omitempty"`
	SummaryText        string                  `json:"summary_text,omitempty" bson:"summary_text,omitempty"`
	EffortUsed         string                  `json:"effort_used,omitempty" bson:"effort_used,omitempty"`
	ItemsPresent       bool                    `json:"items_present,omitempty" bson:"items_present,omitempty"`
	PreviousResponseID string                  `json:"previous_response_id,omitempty" bson:"previous_response_id,omitempty"`
	ContinuityMode     ReasoningContinuityMode `json:"continuity_mode,omitempty" bson:"continuity_mode,omitempty"`
	Usage              ReasoningUsage          `json:"usage,omitempty" bson:"usage,omitempty"`
	TransportWarning   string                  `json:"transport_warning,omitempty" bson:"transport_warning,omitempty"`
}

type persistedReasoningRuntimeState struct {
	LastResponseID         string                  `json:"last_response_id,omitempty"`
	LastPreviousResponseID string                  `json:"last_previous_response_id,omitempty"`
	LastSummaryText        string                  `json:"last_summary_text,omitempty"`
	LastEffortUsed         string                  `json:"last_effort_used,omitempty"`
	LastItemsPresent       bool                    `json:"last_items_present,omitempty"`
	LastContinuityMode     ReasoningContinuityMode `json:"last_continuity_mode,omitempty"`
	LastCapabilityWarning  string                  `json:"last_capability_warning,omitempty"`
	LastCapabilities       TransportCapabilities   `json:"last_capabilities,omitempty"`
}

type reasoningConfigContextKey struct{}

type resolvedReasoningRequest struct {
	Config             *ReasoningConfig
	Capabilities       TransportCapabilities
	PreviousResponseID string
	Enabled            bool
}

func WithReasoningConfig(ctx context.Context, config ReasoningConfig) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, reasoningConfigContextKey{}, config)
}

func ResolveTransportCapabilities(provider string, apiMode string) TransportCapabilities {
	if strings.TrimSpace(apiMode) == APIModeOpenAICodexResponses || isOpenAICodexProvider(provider) {
		return TransportCapabilities{
			Reasoning:           CapabilitySupportSupported,
			ReasoningSummary:    CapabilitySupportUnsafe,
			ReasoningContinuity: CapabilitySupportUnsafe,
		}
	}
	return TransportCapabilities{
		Reasoning:           CapabilitySupportIgnored,
		ReasoningSummary:    CapabilitySupportIgnored,
		ReasoningContinuity: CapabilitySupportIgnored,
	}
}

func normalizeReasoningConfig(cfg *ReasoningConfig) *ReasoningConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Effort = ReasoningEffort(strings.TrimSpace(string(normalized.Effort)))
	normalized.Summary = ReasoningSummaryMode(strings.TrimSpace(string(normalized.Summary)))
	normalized.Continuity = ReasoningContinuityMode(strings.TrimSpace(string(normalized.Continuity)))
	normalized.ExposeSummary = ReasoningSummaryExposure(strings.TrimSpace(string(normalized.ExposeSummary)))

	if normalized.Summary == "" {
		normalized.Summary = ReasoningSummaryAuto
	}
	if normalized.Continuity == "" {
		normalized.Continuity = ReasoningContinuityDisabled
	}
	if normalized.ExposeSummary == "" {
		normalized.ExposeSummary = ReasoningSummaryExposureInternal
	}
	if normalized.Effort == "" && normalized.Summary == ReasoningSummaryAuto && normalized.Continuity == ReasoningContinuityDisabled {
		return nil
	}
	return &normalized
}

func mergeReasoningConfig(base *ReasoningConfig, override *ReasoningConfig) *ReasoningConfig {
	if base == nil && override == nil {
		return nil
	}
	result := &ReasoningConfig{}
	if base != nil {
		*result = *base
	}
	if override != nil {
		if override.Effort != "" {
			result.Effort = override.Effort
		}
		if override.Summary != "" {
			result.Summary = override.Summary
		}
		if override.Continuity != "" {
			result.Continuity = override.Continuity
		}
		if override.ExposeSummary != "" {
			result.ExposeSummary = override.ExposeSummary
		}
	}
	return normalizeReasoningConfig(result)
}

func (c *CsAI) resolveReasoningConfig(ctx context.Context, override *ReasoningConfig, disableContinuity bool) *ReasoningConfig {
	merged := mergeReasoningConfig(c.options.Reasoning, override)
	if ctx != nil {
		if injected, ok := ctx.Value(reasoningConfigContextKey{}).(ReasoningConfig); ok {
			merged = mergeReasoningConfig(merged, &injected)
		}
	}
	if merged == nil {
		return nil
	}
	if disableContinuity {
		merged.Continuity = ReasoningContinuityDisabled
	}
	return normalizeReasoningConfig(merged)
}

func (c *CsAI) loadReasoningRuntimeState(sessionID string) (persistedReasoningRuntimeState, map[string]interface{}, error) {
	state := persistedReasoningRuntimeState{}
	if strings.TrimSpace(sessionID) == "" {
		return state, map[string]interface{}{}, nil
	}

	raw, err := c.GetSessionState(sessionID)
	if err != nil {
		return state, nil, err
	}
	if raw == nil {
		raw = map[string]interface{}{}
	}
	if internal, ok := raw[reasoningRuntimeStateKey].(map[string]interface{}); ok && internal != nil {
		payload, marshalErr := json.Marshal(internal)
		if marshalErr == nil {
			_ = json.Unmarshal(payload, &state)
		}
	}
	return state, raw, nil
}

func (c *CsAI) saveReasoningRuntimeState(sessionID string, raw map[string]interface{}, state persistedReasoningRuntimeState) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if raw == nil {
		raw = map[string]interface{}{}
	}

	internalBytes, err := json.Marshal(state)
	if err != nil {
		return err
	}
	internal := map[string]interface{}{}
	if err := json.Unmarshal(internalBytes, &internal); err != nil {
		return err
	}
	raw[reasoningRuntimeStateKey] = internal
	return c.SaveSessionState(sessionID, raw)
}

func (c *CsAI) persistReasoningState(sessionID string, msg Message, request *resolvedReasoningRequest) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}

	state, raw, err := c.loadReasoningRuntimeState(sessionID)
	if err != nil {
		return
	}
	if strings.TrimSpace(msg.ResponseID) != "" {
		state.LastResponseID = strings.TrimSpace(msg.ResponseID)
	}
	if request != nil {
		state.LastCapabilities = request.Capabilities
		if request.Config != nil {
			state.LastContinuityMode = request.Config.Continuity
		}
		if strings.TrimSpace(request.PreviousResponseID) != "" {
			state.LastPreviousResponseID = strings.TrimSpace(request.PreviousResponseID)
		}
	}
	if msg.Reasoning != nil {
		if strings.TrimSpace(msg.Reasoning.SummaryText) != "" {
			state.LastSummaryText = strings.TrimSpace(msg.Reasoning.SummaryText)
		} else {
			state.LastSummaryText = ""
		}
		if strings.TrimSpace(msg.Reasoning.EffortUsed) != "" {
			state.LastEffortUsed = strings.TrimSpace(msg.Reasoning.EffortUsed)
		}
		state.LastItemsPresent = msg.Reasoning.ItemsPresent
		if strings.TrimSpace(msg.Reasoning.TransportWarning) != "" {
			state.LastCapabilityWarning = strings.TrimSpace(msg.Reasoning.TransportWarning)
		}
	}
	_ = c.saveReasoningRuntimeState(sessionID, raw, state)
}
