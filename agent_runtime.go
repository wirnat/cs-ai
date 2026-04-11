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
	compactRecentContextPrefix    = "Konteks percakapan terbaru:\n"
)

type AgentRuntimeOptions struct {
	Strategy            ContextStrategy
	Summary             SummaryAgent
	Identifier          IdentifierAgent
	Answer              AnswerAgent
	Models              AgentModelProfiles
	Streaming           AgentStreamingProfiles
	Builtins            BuiltinAgentOptions
	InstructionProvider AgentInstructionProvider
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
	Reasoning       *ReasoningConfig
	FallbackToMain  bool
}

type AgentStage string

const (
	AgentStageSummary    AgentStage = "summary"
	AgentStageIdentifier AgentStage = "identifier"
	AgentStageAnswer     AgentStage = "answer"
)

type AgentRuntimeInstructions struct {
	Shared     []string
	Summary    []string
	Identifier []string
	Answer     []string
}

type AgentInstructionContext struct {
	Stage                AgentStage
	SessionID            string
	LatestUserMessage    UserMessage
	ConversationSummary  string
	ExternalState        map[string]interface{}
	ToolManifest         []ToolManifestEntry
	AllowedToolCodes     []string
	ResolvedSystemPrompt []string
}

type AgentInstructionProvider interface {
	InstructionsForAgent(ctx context.Context, input AgentInstructionContext) []string
}

type agentRuntimeInstructionsContextKey struct{}

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
	RecentConversation  []Message
	ToolEvidence        []StructuredToolTrace
}

type SummaryOutput struct {
	ConversationSummary string
	StatePatch          map[string]interface{}
	Warnings            []string
}

type ToolManifestEntry struct {
	Code                         string                    `json:"code"`
	Description                  string                    `json:"description,omitempty"`
	AccessMode                   ToolAccessMode            `json:"access_mode,omitempty"`
	RequiresExplicitConfirmation bool                      `json:"requires_explicit_confirmation,omitempty"`
	IdempotencyScope             string                    `json:"idempotency_scope,omitempty"`
	UserVisibleTextPolicy        ToolUserVisibleTextPolicy `json:"user_visible_text_policy,omitempty"`
}

type IdentifierInput struct {
	SessionID           string
	LatestUserMessage   UserMessage
	ConversationSummary string
	ExternalState       map[string]interface{}
	ToolManifest        []ToolManifestEntry
	RecentConversation  []Message
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
	RecentConversation   []Message
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

func WithAgentRuntimeInstructions(ctx context.Context, instructions AgentRuntimeInstructions) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, agentRuntimeInstructionsContextKey{}, instructions)
}

func (i AgentRuntimeInstructions) ForStage(stage AgentStage) []string {
	lines := append([]string(nil), i.Shared...)
	switch stage {
	case AgentStageSummary:
		lines = append(lines, i.Summary...)
	case AgentStageIdentifier:
		lines = append(lines, i.Identifier...)
	case AgentStageAnswer:
		lines = append(lines, i.Answer...)
	}
	return compactInstructionLines(lines)
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
		resolved.Streaming = raw.Streaming
		resolved.InstructionProvider = raw.InstructionProvider
		if raw.Builtins.FallbackOnError {
			resolved.Builtins.FallbackOnError = true
		}
	}

	resolved.Streaming.Summary = normalizeStageStreamingConfig(resolved.Streaming.Summary, AgentStageSummary)
	resolved.Streaming.Identifier = normalizeStageStreamingConfig(resolved.Streaming.Identifier, AgentStageIdentifier)
	resolved.Streaming.Answer = normalizeStageStreamingConfig(resolved.Streaming.Answer, AgentStageAnswer)

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
		if key == agentRuntimeStateKey || key == reasoningRuntimeStateKey {
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
		metadata := resolveToolMetadata(intent)
		manifest = append(manifest, ToolManifestEntry{
			Code:                         strings.TrimSpace(intent.Code()),
			Description:                  strings.TrimSpace(strings.Join(intent.Description(), ", ")),
			AccessMode:                   metadata.AccessMode,
			RequiresExplicitConfirmation: metadata.RequiresExplicitConfirmation,
			IdempotencyScope:             metadata.IdempotencyScope,
			UserVisibleTextPolicy:        metadata.UserVisibleTextPolicy,
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
	expandedMessages := make([]Message, 0, len(messages)+len(c.options.DeveloperMessages))
	var otherMsgs []Message

	for _, msg := range messages {
		if msg.Role == System {
			expandedMessages = append(expandedMessages, msg)
		} else {
			otherMsgs = append(otherMsgs, msg)
		}
	}

	for _, devMsg := range c.options.DeveloperMessages {
		trimmed := strings.TrimSpace(devMsg)
		if trimmed != "" {
			expandedMessages = append(expandedMessages, Message{Role: Developer, Content: trimmed})
		}
	}

	expandedMessages = append(expandedMessages, otherMsgs...)

	roleMessages := make([]map[string]interface{}, 0, len(expandedMessages))
	for _, msg := range expandedMessages {
		mapped, err := msg.MessageToMap()
		if err != nil {
			return Message{}, err
		}
		roleMessages = append(roleMessages, mapped)
	}

	modelCandidate := c.modelForAgent(profile)
	override := resolveAgentProfileReasoning(profile)
	return c.sendWithModelReasoning(ctx, "", modelCandidate, roleMessages, nil, override, true)
}

func resolveAgentProfileReasoning(profile AgentModelProfile) *ReasoningConfig {
	override := profile.Reasoning
	if override == nil && strings.TrimSpace(profile.ReasoningEffort) != "" {
		override = &ReasoningConfig{Effort: ReasoningEffort(strings.TrimSpace(profile.ReasoningEffort))}
	}
	return override
}

func (c *CsAI) resolveAgentInstructions(ctx context.Context, input AgentInstructionContext) []string {
	lines := make([]string, 0, 8)

	runtime := c.resolvedAgentRuntimeOptions()
	if runtime.InstructionProvider != nil {
		lines = append(lines, runtime.InstructionProvider.InstructionsForAgent(ctx, input)...)
	}

	if ctx != nil {
		if injected, ok := ctx.Value(agentRuntimeInstructionsContextKey{}).(AgentRuntimeInstructions); ok {
			lines = append(lines, injected.ForStage(input.Stage)...)
		}
	}

	return compactInstructionLines(lines)
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

	systemLines := []string{
		"Kamu adalah summary agent internal.",
		"Ringkas percakapan secara padat dengan bahasa yang sama seperti bahasa user pada percakapan terbaru.",
		"Fokus pada tujuan user, fakta yang sudah dikonfirmasi, constraint penting, dan tindak lanjut yang masih terbuka.",
		"Jangan hilangkan detail waktu, tanggal, angka, nama orang, dan pertanyaan klarifikasi yang masih menunggu jawaban user.",
		"Jawaban maksimal 6 baris pendek dan tanpa markdown.",
	}
	systemLines = append(systemLines, a.owner.resolveAgentInstructions(ctx, AgentInstructionContext{
		Stage:               AgentStageSummary,
		SessionID:           input.SessionID,
		LatestUserMessage:   input.LatestUserMessage,
		ConversationSummary: input.PreviousSummary,
		ExternalState:       input.ExternalState,
	})...)
	systemPrompt := strings.Join(compactInstructionLines(systemLines), "\n")

	stateText := stringifyCompactState(input.ExternalState)
	evidenceText := stringifyToolEvidence(input.ToolEvidence)

	userPrompt := strings.Join([]string{
		"Summary lama:",
		firstNonEmptyString(previous, "(kosong)"),
		"",
		"State eksternal:",
		firstNonEmptyString(stateText, "(kosong)"),
		"",
		"Konteks percakapan terbaru:",
		firstNonEmptyString(stringifyRecentConversation(input.RecentConversation), "(kosong)"),
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

	runtime := a.owner.resolvedAgentRuntimeOptions()
	stageCtx := withStageStreaming(ctx, AgentStageSummary, runtime.Streaming.Summary)
	stageCtx = WithHTTPLogMetadata(stageCtx, HTTPLogMetadata{
		SessionID:   strings.TrimSpace(input.SessionID),
		Stage:       "summary",
		RequestKind: "summary",
	})
	msg, err := a.owner.invokeAgentModel(stageCtx, runtime.Models.Summary, []Message{
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
	systemLines := []string{
		"Kamu adalah identifier agent internal.",
		"Tugasmu hanya memilih tool subset yang relevan untuk turn ini.",
		"Balas HANYA JSON valid tanpa markdown.",
		`Format: {"route":"answer_direct|run_tool_then_answer","need_tool":true|false,"allowed_tool_codes":["..."],"tool_order":["..."],"missing_info":["..."],"can_answer_direct":true|false,"confidence":0.0}`,
		"Jangan menulis jawaban untuk user.",
	}
	systemLines = append(systemLines, a.owner.resolveAgentInstructions(ctx, AgentInstructionContext{
		Stage:               AgentStageIdentifier,
		SessionID:           input.SessionID,
		LatestUserMessage:   input.LatestUserMessage,
		ConversationSummary: input.ConversationSummary,
		ExternalState:       input.ExternalState,
		ToolManifest:        input.ToolManifest,
	})...)
	systemPrompt := strings.Join(compactInstructionLines(systemLines), "\n")

	userPrompt := strings.Join([]string{
		"Ringkasan percakapan:",
		firstNonEmptyString(strings.TrimSpace(input.ConversationSummary), "(kosong)"),
		"",
		"Konteks percakapan terbaru:",
		firstNonEmptyString(stringifyRecentConversation(input.RecentConversation), "(kosong)"),
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

	runtime := a.owner.resolvedAgentRuntimeOptions()
	stageCtx := withStageStreaming(ctx, AgentStageIdentifier, runtime.Streaming.Identifier)
	stageCtx = WithHTTPLogMetadata(stageCtx, HTTPLogMetadata{
		SessionID:   strings.TrimSpace(input.SessionID),
		Stage:       "identifier",
		RequestKind: "identifier",
	})
	msg, err := a.owner.invokeAgentModel(stageCtx, runtime.Models.Identifier, []Message{
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
	resolvedSystemPrompt := append([]string(nil), input.ResolvedSystemPrompt...)
	resolvedSystemPrompt = append(resolvedSystemPrompt,
		"Jangan pernah menampilkan data teknis internal seperti uid, id, booking_reference, order_uid, provider_uid, service_uid, atau identifier internal lainnya kepada user.",
		"Selalu jawab menggunakan bahasa yang sama dengan bahasa user pada pesan terbaru, tanpa membatasi hanya pada bahasa tertentu.",
	)
	resolvedSystemPrompt = append(resolvedSystemPrompt, a.owner.resolveAgentInstructions(ctx, AgentInstructionContext{
		Stage:                AgentStageAnswer,
		SessionID:            input.SessionID,
		LatestUserMessage:    input.UserMessage,
		ConversationSummary:  input.ConversationSummary,
		ExternalState:        input.ExternalState,
		AllowedToolCodes:     input.AllowedToolCodes,
		ResolvedSystemPrompt: input.ResolvedSystemPrompt,
	})...)

	runtime := a.owner.resolvedAgentRuntimeOptions()
	stageCtx := withStageStreaming(ctx, AgentStageAnswer, runtime.Streaming.Answer)
	stageCtx = WithHTTPLogMetadata(stageCtx, HTTPLogMetadata{
		SessionID:   strings.TrimSpace(input.SessionID),
		Stage:       "answer",
		RequestKind: "answer",
	})
	if override := resolveAgentProfileReasoning(runtime.Models.Answer); override != nil {
		stageCtx = WithReasoningConfig(stageCtx, *override)
	}
	return a.owner.runCompactAnswerLoop(
		stageCtx,
		input.SessionID,
		input.UserMessage,
		input.RuntimeIntents,
		input.ConversationSummary,
		input.RecentConversation,
		input.ApplyBootstrap,
		compactInstructionLines(resolvedSystemPrompt)...,
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

func stringifyRecentConversation(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}

	lines := make([]string, 0, len(messages))
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}

		role := strings.ToLower(strings.TrimSpace(string(msg.Role)))
		if role == "" {
			role = "unknown"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", role, content))
	}
	return strings.Join(lines, "\n")
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

func compactInstructionLines(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}

	result := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
