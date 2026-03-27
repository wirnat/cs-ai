package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type ContextStrategy string

const (
	ContextStrategyLegacy         ContextStrategy = "legacy"
	ContextStrategyCompactBackend ContextStrategy = "compact_backend"
	ContextStrategyCompactHybrid  ContextStrategy = "compact_hybrid"
)

const (
	agentRuntimeStateKey          = "_csai_agent_runtime"
	compactSummaryAssistantPrefix = "Ringkasan percakapan sejauh ini:\n"
)

type AgentRuntimeOptions struct {
	Strategy   ContextStrategy
	Summary    SummaryAgent
	Identifier IdentifierAgent
	Answer     AnswerAgent
	Models     AgentModelProfiles
	Builtins   BuiltinAgentOptions
}

type BuiltinAgentOptions struct {
	FallbackOnError bool
}

type AgentModelProfiles struct {
	Summary    AgentModelProfile
	Identifier AgentModelProfile
	Answer     AgentModelProfile
}

type AgentModelProfile struct {
	Model           string
	ReasoningEffort string
	FallbackToMain  bool
}

type SummaryAgent interface {
	Summarize(ctx context.Context, input SummaryInput) (SummaryOutput, error)
}

type IdentifierAgent interface {
	Identify(ctx context.Context, input IdentifierInput) (IdentifierOutput, error)
}

type AnswerAgent interface {
	Answer(ctx context.Context, input AnswerInput) (AnswerOutput, error)
}

type SummaryInput struct {
	SessionID           string
	PreviousSummary     string
	LatestUserMessage   UserMessage
	LatestAssistantText string
	ExternalState       map[string]interface{}
	RecentMessages      []Message
	ToolEvidence        []StructuredToolTrace
}

type SummaryOutput struct {
	ConversationSummary string
	StatePatch          map[string]interface{}
	Warnings            []string
}

type ToolManifestEntry struct {
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
}

type IdentifierInput struct {
	SessionID           string
	LatestUserMessage   UserMessage
	ConversationSummary string
	ExternalState       map[string]interface{}
	ToolManifest        []ToolManifestEntry
}

type IdentifierOutput struct {
	Route            string   `json:"route,omitempty"`
	NeedTool         bool     `json:"need_tool"`
	AllowedToolCodes []string `json:"allowed_tool_codes,omitempty"`
	ToolOrder        []string `json:"tool_order,omitempty"`
	MissingInfo      []string `json:"missing_info,omitempty"`
	CanAnswerDirect  bool     `json:"can_answer_direct"`
	Confidence       float64  `json:"confidence,omitempty"`
}

type AnswerInput struct {
	SessionID            string
	UserMessage          UserMessage
	RuntimeIntents       []Intent
	AllowedToolCodes     []string
	ConversationSummary  string
	ExternalState        map[string]interface{}
	ResolvedSystemPrompt []string
	ApplyBootstrap       bool
}

type AnswerOutput struct {
	RawMessage    Message
	DeltaMessages []Message
	FinalMessage  string
	Warnings      []string
}

type persistedAgentRuntimeState struct {
	ConversationSummary string `json:"conversation_summary,omitempty"`
}

type builtInSummaryAgent struct {
	owner *CsAI
}

type builtInIdentifierAgent struct {
	owner *CsAI
}

type builtInAnswerAgent struct {
	owner *CsAI
}

func (c *CsAI) resolvedAgentRuntimeOptions() AgentRuntimeOptions {
	resolved := AgentRuntimeOptions{
		Strategy: ContextStrategyLegacy,
		Builtins: BuiltinAgentOptions{
			FallbackOnError: true,
		},
	}

	if c != nil && c.options.AgentRuntime != nil {
		raw := *c.options.AgentRuntime
		if raw.Strategy != "" {
			resolved.Strategy = raw.Strategy
		}
		resolved.Summary = raw.Summary
		resolved.Identifier = raw.Identifier
		resolved.Answer = raw.Answer
		resolved.Models = raw.Models
		if raw.Builtins.FallbackOnError {
			resolved.Builtins.FallbackOnError = true
		}
	}

	if resolved.Summary == nil {
		resolved.Summary = &builtInSummaryAgent{owner: c}
	}
	if resolved.Identifier == nil {
		resolved.Identifier = &builtInIdentifierAgent{owner: c}
	}
	if resolved.Answer == nil {
		resolved.Answer = &builtInAnswerAgent{owner: c}
	}

	return resolved
}

func (c *CsAI) loadAgentRuntimeState(sessionID string) (persistedAgentRuntimeState, map[string]interface{}, error) {
	state := persistedAgentRuntimeState{}
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
	if internal, ok := raw[agentRuntimeStateKey].(map[string]interface{}); ok && internal != nil {
		payload, marshalErr := json.Marshal(internal)
		if marshalErr == nil {
			_ = json.Unmarshal(payload, &state)
		}
	}
	return state, raw, nil
}

func (c *CsAI) saveAgentRuntimeState(sessionID string, raw map[string]interface{}, state persistedAgentRuntimeState) error {
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
	raw[agentRuntimeStateKey] = internal
	return c.SaveSessionState(sessionID, raw)
}

func stripInternalRuntimeState(raw map[string]interface{}) map[string]interface{} {
	if len(raw) == 0 {
		return map[string]interface{}{}
	}
	result := map[string]interface{}{}
	for key, value := range raw {
		if key == agentRuntimeStateKey {
			continue
		}
		result[key] = value
	}
	return result
}

func buildToolManifest(intents []Intent) []ToolManifestEntry {
	if len(intents) == 0 {
		return nil
	}
	manifest := make([]ToolManifestEntry, 0, len(intents))
	for _, intent := range intents {
		if intent == nil {
			continue
		}
		manifest = append(manifest, ToolManifestEntry{
			Code:        strings.TrimSpace(intent.Code()),
			Description: strings.TrimSpace(strings.Join(intent.Description(), ", ")),
		})
	}
	return manifest
}

func normalizeIdentifierAllowedTools(codes []string, runtimeIntents []Intent) []string {
	if len(runtimeIntents) == 0 {
		return []string{}
	}
	if codes == nil {
		result := make([]string, 0, len(runtimeIntents))
		for _, intent := range runtimeIntents {
			result = append(result, intent.Code())
		}
		return result
	}
	return normalizeAllowedToolCodes(codes)
}

func (c *CsAI) invokeAgentModel(
	ctx context.Context,
	profile AgentModelProfile,
	messages []Message,
) (Message, error) {
	roleMessages := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		mapped, err := msg.MessageToMap()
		if err != nil {
			return Message{}, err
		}
		roleMessages = append(roleMessages, mapped)
	}

	modelCandidate := c.modelForAgent(profile)
	return c.sendWithModel(ctx, "", modelCandidate, roleMessages, nil)
}

func (c *CsAI) modelForAgent(profile AgentModelProfile) Modeler {
	base := c.Model
	if base == nil {
		return nil
	}
	modelName := strings.TrimSpace(profile.Model)
	if modelName == "" {
		return base
	}
	return overrideModeler{
		base:      base,
		modelName: modelName,
	}
}

type overrideModeler struct {
	base      Modeler
	modelName string
}

func (o overrideModeler) ModelName() string {
	if o.modelName != "" {
		return o.modelName
	}
	if o.base == nil {
		return ""
	}
	return o.base.ModelName()
}

func (o overrideModeler) ApiURL() string {
	if o.base == nil {
		return ""
	}
	return o.base.ApiURL()
}

func (o overrideModeler) Train() []string {
	if o.base == nil {
		return nil
	}
	return o.base.Train()
}

func (o overrideModeler) ProviderName() string {
	if providerModel, ok := o.base.(ProviderModeler); ok {
		return providerModel.ProviderName()
	}
	return ""
}

func (o overrideModeler) APIMode() string {
	if providerModel, ok := o.base.(ProviderModeler); ok {
		return providerModel.APIMode()
	}
	return ""
}

func (a *builtInSummaryAgent) Summarize(ctx context.Context, input SummaryInput) (SummaryOutput, error) {
	previous := strings.TrimSpace(input.PreviousSummary)
	if previous != "" && strings.TrimSpace(input.LatestAssistantText) == "" {
		return SummaryOutput{ConversationSummary: previous}, nil
	}

	systemPrompt := strings.Join([]string{
		"Kamu adalah summary agent internal.",
		"Ringkas percakapan secara padat dalam Bahasa Indonesia/Bahasa Inggris sesuai dengan bahasa user.",
		"Fokus pada tujuan user, fakta yang sudah dikonfirmasi, constraint penting, dan tindak lanjut yang masih terbuka.",
		"Jawaban maksimal 6 baris pendek dan tanpa markdown.",
	}, "\n")

	stateText := stringifyCompactState(input.ExternalState)
	evidenceText := stringifyToolEvidence(input.ToolEvidence)

	userPrompt := strings.Join([]string{
		"Summary lama:",
		firstNonEmptyString(previous, "(kosong)"),
		"",
		"State eksternal:",
		firstNonEmptyString(stateText, "(kosong)"),
		"",
		"User terbaru:",
		strings.TrimSpace(input.LatestUserMessage.Message),
		"",
		"Asisten terbaru:",
		firstNonEmptyString(strings.TrimSpace(input.LatestAssistantText), "(kosong)"),
		"",
		"Evidence tool:",
		firstNonEmptyString(evidenceText, "(kosong)"),
	}, "\n")

	msg, err := a.owner.invokeAgentModel(ctx, a.owner.resolvedAgentRuntimeOptions().Models.Summary, []Message{
		{Role: System, Content: systemPrompt},
		{Role: User, Content: userPrompt},
	})
	if err != nil {
		fallback := strings.TrimSpace(strings.Join([]string{
			previous,
			strings.TrimSpace(input.LatestUserMessage.Message),
			strings.TrimSpace(input.LatestAssistantText),
		}, "\n"))
		return SummaryOutput{
			ConversationSummary: strings.TrimSpace(fallback),
			Warnings:            []string{fmt.Sprintf("summary agent fallback: %v", err)},
		}, nil
	}

	return SummaryOutput{
		ConversationSummary: strings.TrimSpace(msg.Content),
	}, nil
}

func (a *builtInIdentifierAgent) Identify(ctx context.Context, input IdentifierInput) (IdentifierOutput, error) {
	if len(input.ToolManifest) == 0 {
		return IdentifierOutput{
			Route:           "answer_direct",
			NeedTool:        false,
			CanAnswerDirect: true,
			Confidence:      1,
		}, nil
	}

	manifestBytes, _ := json.Marshal(input.ToolManifest)
	systemPrompt := strings.Join([]string{
		"Kamu adalah identifier agent internal.",
		"Tugasmu hanya memilih tool subset yang relevan untuk turn ini.",
		"Balas HANYA JSON valid tanpa markdown.",
		`Format: {"route":"answer_direct|run_tool_then_answer","need_tool":true|false,"allowed_tool_codes":["..."],"tool_order":["..."],"missing_info":["..."],"can_answer_direct":true|false,"confidence":0.0}`,
		"Jangan menulis jawaban untuk user.",
	}, "\n")

	userPrompt := strings.Join([]string{
		"Ringkasan percakapan:",
		firstNonEmptyString(strings.TrimSpace(input.ConversationSummary), "(kosong)"),
		"",
		"State eksternal:",
		firstNonEmptyString(stringifyCompactState(input.ExternalState), "(kosong)"),
		"",
		"Pesan user:",
		strings.TrimSpace(input.LatestUserMessage.Message),
		"",
		"Manifest tool:",
		string(manifestBytes),
	}, "\n")

	msg, err := a.owner.invokeAgentModel(ctx, a.owner.resolvedAgentRuntimeOptions().Models.Identifier, []Message{
		{Role: System, Content: systemPrompt},
		{Role: User, Content: userPrompt},
	})
	if err != nil {
		return IdentifierOutput{
			Route:            "run_tool_then_answer",
			NeedTool:         true,
			AllowedToolCodes: manifestCodes(input.ToolManifest),
			ToolOrder:        manifestCodes(input.ToolManifest),
			Confidence:       0.2,
		}, nil
	}

	output := IdentifierOutput{}
	if err := decodeJSONObjectStrict(msg.Content, &output); err != nil {
		return IdentifierOutput{
			Route:            "run_tool_then_answer",
			NeedTool:         true,
			AllowedToolCodes: manifestCodes(input.ToolManifest),
			ToolOrder:        manifestCodes(input.ToolManifest),
			Confidence:       0.2,
		}, nil
	}

	output.AllowedToolCodes = normalizeAllowedToolCodes(output.AllowedToolCodes)
	output.ToolOrder = normalizeAllowedToolCodes(output.ToolOrder)
	if len(output.ToolOrder) == 0 {
		output.ToolOrder = append([]string(nil), output.AllowedToolCodes...)
	}
	return output, nil
}

func (a *builtInAnswerAgent) Answer(ctx context.Context, input AnswerInput) (AnswerOutput, error) {
	return a.owner.runCompactAnswerLoop(
		ctx,
		input.SessionID,
		input.UserMessage,
		input.RuntimeIntents,
		input.ConversationSummary,
		input.ApplyBootstrap,
		input.ResolvedSystemPrompt...,
	)
}

func manifestCodes(manifest []ToolManifestEntry) []string {
	if len(manifest) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(manifest))
	for _, item := range manifest {
		if code := strings.TrimSpace(item.Code); code != "" {
			result = append(result, code)
		}
	}
	return result
}

func stringifyCompactState(state map[string]interface{}) string {
	if len(state) == 0 {
		return ""
	}
	bytes, err := json.Marshal(state)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func stringifyToolEvidence(traces []StructuredToolTrace) string {
	if len(traces) == 0 {
		return ""
	}
	lines := make([]string, 0, len(traces))
	for _, trace := range traces {
		if trace.ToolName == "" {
			continue
		}
		line := trace.ToolName
		if trace.Status != "" {
			line += ":" + trace.Status
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, ", ")
}

func decodeJSONObjectStrict(raw string, target interface{}) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("empty json payload")
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}
	return json.Unmarshal([]byte(trimmed), target)
}
