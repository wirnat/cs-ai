package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type StageExecutionMode string

const (
	StageModeStream     StageExecutionMode = "stream"
	StageModeCompletion StageExecutionMode = "completion"
)

type StageStreamingConfig struct {
	Mode          StageExecutionMode `json:"mode,omitempty" bson:"mode,omitempty"`
	EmitTextDelta bool               `json:"emit_text_delta,omitempty" bson:"emit_text_delta,omitempty"`
	EmitProgress  bool               `json:"emit_progress,omitempty" bson:"emit_progress,omitempty"`
}

type AgentStreamingProfiles struct {
	Summary    StageStreamingConfig `json:"summary,omitempty" bson:"summary,omitempty"`
	Identifier StageStreamingConfig `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Answer     StageStreamingConfig `json:"answer,omitempty" bson:"answer,omitempty"`
}

const StreamTransportOrchestratedHTTP = "orchestrated_http"

type StreamGuardPolicy struct {
	MaxHopsPerTurn         int `json:"max_hops_per_turn,omitempty" bson:"max_hops_per_turn,omitempty"`
	MaxSameSignatureRepeat int `json:"max_same_signature_repeat,omitempty" bson:"max_same_signature_repeat,omitempty"`
	MaxNoProgressLoops     int `json:"max_no_progress_loops,omitempty" bson:"max_no_progress_loops,omitempty"`
	MaxToolErrorStreak     int `json:"max_tool_error_streak,omitempty" bson:"max_tool_error_streak,omitempty"`
}

type StreamingOptions struct {
	Enabled       bool              `json:"enabled,omitempty" bson:"enabled,omitempty"`
	TransportMode string            `json:"transport_mode,omitempty" bson:"transport_mode,omitempty"`
	EmitProgress  bool              `json:"emit_progress,omitempty" bson:"emit_progress,omitempty"`
	EmitDebug     bool              `json:"emit_debug,omitempty" bson:"emit_debug,omitempty"`
	GuardPolicy   StreamGuardPolicy `json:"guard_policy,omitempty" bson:"guard_policy,omitempty"`
}

type StreamEvent struct {
	TurnID    string         `json:"turn_id,omitempty" bson:"turn_id,omitempty"`
	Seq       int64          `json:"seq,omitempty" bson:"seq,omitempty"`
	Timestamp time.Time      `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
	Stage     string         `json:"stage,omitempty" bson:"stage,omitempty"`
	Type      string         `json:"type,omitempty" bson:"type,omitempty"`
	Status    string         `json:"status,omitempty" bson:"status,omitempty"`
	Attempt   int            `json:"attempt,omitempty" bson:"attempt,omitempty"`
	Hop       int            `json:"hop,omitempty" bson:"hop,omitempty"`
	Provider  string         `json:"provider,omitempty" bson:"provider,omitempty"`
	Model     string         `json:"model,omitempty" bson:"model,omitempty"`
	ToolName  string         `json:"tool_name,omitempty" bson:"tool_name,omitempty"`
	TextDelta string         `json:"text_delta,omitempty" bson:"text_delta,omitempty"`
	Message   string         `json:"message,omitempty" bson:"message,omitempty"`
	LatencyMs int64          `json:"latency_ms,omitempty" bson:"latency_ms,omitempty"`
	Usage     *DeepSeekUsage `json:"usage,omitempty" bson:"usage,omitempty"`
	ErrCode   string         `json:"err_code,omitempty" bson:"err_code,omitempty"`
	Final     bool           `json:"final,omitempty" bson:"final,omitempty"`
}

type StreamSink interface {
	Emit(ctx context.Context, event StreamEvent) error
}

type streamRuntime struct {
	sink    StreamSink
	options StreamingOptions
	turnID  string

	mu  sync.Mutex
	seq int64
}

type streamRuntimeContextKey struct{}
type streamStageContextKey struct{}

type streamStageContext struct {
	Stage  AgentStage
	Config StageStreamingConfig
}

func normalizedStreamingOptions(raw *StreamingOptions) StreamingOptions {
	options := StreamingOptions{
		Enabled:       false,
		TransportMode: StreamTransportOrchestratedHTTP,
		EmitProgress:  true,
		EmitDebug:     false,
		GuardPolicy: StreamGuardPolicy{
			MaxHopsPerTurn:         8,
			MaxSameSignatureRepeat: 2,
			MaxNoProgressLoops:     2,
			MaxToolErrorStreak:     2,
		},
	}
	if raw == nil {
		return options
	}
	options.Enabled = raw.Enabled
	if mode := strings.TrimSpace(raw.TransportMode); mode != "" {
		options.TransportMode = mode
	}
	options.EmitProgress = raw.EmitProgress
	options.EmitDebug = raw.EmitDebug
	if raw.GuardPolicy.MaxHopsPerTurn > 0 {
		options.GuardPolicy.MaxHopsPerTurn = raw.GuardPolicy.MaxHopsPerTurn
	}
	if raw.GuardPolicy.MaxSameSignatureRepeat > 0 {
		options.GuardPolicy.MaxSameSignatureRepeat = raw.GuardPolicy.MaxSameSignatureRepeat
	}
	if raw.GuardPolicy.MaxNoProgressLoops > 0 {
		options.GuardPolicy.MaxNoProgressLoops = raw.GuardPolicy.MaxNoProgressLoops
	}
	if raw.GuardPolicy.MaxToolErrorStreak > 0 {
		options.GuardPolicy.MaxToolErrorStreak = raw.GuardPolicy.MaxToolErrorStreak
	}
	return options
}

func defaultStageStreamingConfig(stage AgentStage) StageStreamingConfig {
	switch stage {
	case AgentStageAnswer:
		return StageStreamingConfig{
			Mode:          StageModeStream,
			EmitTextDelta: true,
			EmitProgress:  true,
		}
	default:
		return StageStreamingConfig{
			Mode:          StageModeCompletion,
			EmitTextDelta: false,
			EmitProgress:  true,
		}
	}
}

func normalizeStageStreamingConfig(raw StageStreamingConfig, stage AgentStage) StageStreamingConfig {
	defaults := defaultStageStreamingConfig(stage)
	result := defaults

	switch strings.ToLower(strings.TrimSpace(string(raw.Mode))) {
	case string(StageModeStream):
		result.Mode = StageModeStream
	case string(StageModeCompletion):
		result.Mode = StageModeCompletion
	}

	if raw.Mode == "" {
		result.Mode = defaults.Mode
	}
	result.EmitTextDelta = raw.EmitTextDelta
	result.EmitProgress = raw.EmitProgress
	if raw.Mode == "" && !raw.EmitTextDelta {
		result.EmitTextDelta = defaults.EmitTextDelta
	}
	if raw.Mode == "" && !raw.EmitProgress {
		result.EmitProgress = defaults.EmitProgress
	}

	return result
}

func (profiles AgentStreamingProfiles) ForStage(stage AgentStage) StageStreamingConfig {
	switch stage {
	case AgentStageSummary:
		return normalizeStageStreamingConfig(profiles.Summary, stage)
	case AgentStageIdentifier:
		return normalizeStageStreamingConfig(profiles.Identifier, stage)
	case AgentStageAnswer:
		return normalizeStageStreamingConfig(profiles.Answer, stage)
	default:
		return normalizeStageStreamingConfig(StageStreamingConfig{}, stage)
	}
}

func (c *CsAI) resolvedStageStreamingConfig(stage AgentStage) StageStreamingConfig {
	runtime := c.resolvedAgentRuntimeOptions()
	return runtime.Streaming.ForStage(stage)
}

func (c *CsAI) newStreamRuntime(sink StreamSink) *streamRuntime {
	if sink == nil {
		return nil
	}
	opts := normalizedStreamingOptions(c.options.Streaming)
	if !opts.Enabled {
		return nil
	}
	return &streamRuntime{
		sink:    sink,
		options: opts,
		turnID:  randomID("turn"),
	}
}

func withStreamRuntime(ctx context.Context, rt *streamRuntime) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if rt == nil {
		return ctx
	}
	return context.WithValue(ctx, streamRuntimeContextKey{}, rt)
}

func extractStreamRuntime(ctx context.Context) *streamRuntime {
	if ctx == nil {
		return nil
	}
	rt, _ := ctx.Value(streamRuntimeContextKey{}).(*streamRuntime)
	return rt
}

func withStageStreaming(ctx context.Context, stage AgentStage, config StageStreamingConfig) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	payload := streamStageContext{
		Stage:  stage,
		Config: normalizeStageStreamingConfig(config, stage),
	}
	return context.WithValue(ctx, streamStageContextKey{}, payload)
}

func extractStageStreaming(ctx context.Context) (streamStageContext, bool) {
	if ctx == nil {
		return streamStageContext{}, false
	}
	stageCtx, ok := ctx.Value(streamStageContextKey{}).(streamStageContext)
	if !ok {
		return streamStageContext{}, false
	}
	stageCtx.Config = normalizeStageStreamingConfig(stageCtx.Config, stageCtx.Stage)
	return stageCtx, true
}

func stageName(stage AgentStage) string {
	switch stage {
	case AgentStageSummary:
		return "summary"
	case AgentStageIdentifier:
		return "identifier"
	case AgentStageAnswer:
		return "answer"
	default:
		return "turn"
	}
}

func emitStreamEvent(ctx context.Context, event StreamEvent) {
	rt := extractStreamRuntime(ctx)
	if rt == nil || rt.sink == nil {
		return
	}

	event.Type = strings.TrimSpace(event.Type)
	if event.Type == "" {
		return
	}

	rt.mu.Lock()
	rt.seq++
	event.Seq = rt.seq
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if strings.TrimSpace(event.TurnID) == "" {
		event.TurnID = rt.turnID
	}
	rt.mu.Unlock()

	if payload, err := json.Marshal(event); err == nil {
		fmt.Printf("[cs-ai-stream] %s\n", string(payload))
	}
	_ = rt.sink.Emit(ctx, event)
}

func emitStageEvent(ctx context.Context, stage AgentStage, eventType string, status string, message string) {
	stageCtx, hasStage := extractStageStreaming(ctx)
	config := defaultStageStreamingConfig(stage)
	if hasStage && stageCtx.Stage == stage {
		config = stageCtx.Config
	}
	rt := extractStreamRuntime(ctx)
	if rt == nil || !config.EmitProgress {
		return
	}
	if !rt.options.EmitProgress {
		return
	}
	emitStreamEvent(ctx, StreamEvent{
		Stage:   stageName(stage),
		Type:    strings.TrimSpace(eventType),
		Status:  strings.TrimSpace(status),
		Message: strings.TrimSpace(message),
	})
}

func shouldStreamModelRequest(ctx context.Context) bool {
	stageCtx, ok := extractStageStreaming(ctx)
	if !ok {
		return true
	}
	return stageCtx.Config.Mode != StageModeCompletion
}

func shouldEmitTextDelta(ctx context.Context) bool {
	rt := extractStreamRuntime(ctx)
	if rt == nil {
		return false
	}
	stageCtx, ok := extractStageStreaming(ctx)
	if !ok {
		return false
	}
	if stageCtx.Config.Mode == StageModeCompletion {
		return false
	}
	return stageCtx.Config.EmitTextDelta
}

func effectiveGuardPolicy(ctx context.Context, options *StreamingOptions) StreamGuardPolicy {
	legacy := StreamGuardPolicy{
		MaxHopsPerTurn:         10,
		MaxSameSignatureRepeat: 2,
		MaxNoProgressLoops:     3,
		MaxToolErrorStreak:     2,
	}

	if rt := extractStreamRuntime(ctx); rt != nil {
		return rt.options.GuardPolicy
	}

	normalized := normalizedStreamingOptions(options)
	if !normalized.Enabled {
		return legacy
	}
	return normalized.GuardPolicy
}
