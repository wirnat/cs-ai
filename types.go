package cs_ai

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
)

type Options struct {
	// === Storage Options ===
	StorageProvider StorageProvider // Storage provider (Redis, MongoDB, DynamoDB, etc.)
	StorageConfig   *StorageConfig  // Storage configuration

	// === Legacy Redis Support (deprecated, use StorageProvider instead) ===
	Redis          *redis.Client // Use *redis.Client for Redis (deprecated)
	UseTool        bool          // Gunakan tool handler api
	LogChatFile    bool          // Simpan chat ke file
	EnableLearning bool          // Aktifkan learning manager
	ResponseType   ResponseType  // Tipe response

	// === LLM Generation Parameters ===
	Temperature      float32 // Kontrol kreativitas output LLM (0.0-2.0, default 0.2)
	TopP             float32 // Kontrol probabilitas sampling LLM (0.0-1.0, default 0.7)
	FrequencyPenalty float32 // Kontrol repetisi token (-2.0-2.0, default 0.0)
	PresencePenalty  float32 // Kontrol repetisi topik (-2.0-2.0, default -1.5)
	Reasoning        *ReasoningConfig
	GroundingRepair  *GroundingRepairOptions
	Streaming        *StreamingOptions

	// === Cache & Session Options ===
	SessionTTL time.Duration // TTL untuk session messages (default: 12 jam)

	// === Security Options ===
	SecurityOptions *SecurityOptions // Security configuration

	// === First-turn bootstrap options ===
	FirstTurnBootstrap *FirstTurnBootstrapOptions // Optional server-side bootstrap intents for first user turn

	// === Injectable agent runtime options ===
	AgentRuntime *AgentRuntimeOptions // Optional compact runtime with injectable summary/identifier/answer agents

	// === Auth & Model Failover ===
	AuthManager     AuthManager // Optional auth resolver (OAuth/profile rotation)
	ModelFallbacks  []Modeler   // Candidate fallback models in order
	DeveloperMessages []string  // Messages injected with role=developer in every LLM request (identity override, persona lock, etc.)
}

type GroundingRepairOptions struct {
	// Enabled mengaktifkan retry soft grounding pada answer stage ketika
	// model memberi jawaban faktual tanpa evidence tool.
	Enabled bool `json:"enabled,omitempty" bson:"enabled,omitempty"`
	// MaxAttempts batas maksimum retry per turn.
	MaxAttempts int `json:"max_attempts,omitempty" bson:"max_attempts,omitempty"`
}

type BootstrapFailurePolicy string

const (
	BootstrapFailureBestEffort BootstrapFailurePolicy = "best_effort"
	BootstrapFailureStrict     BootstrapFailurePolicy = "strict"
)

type BootstrapIntentCall struct {
	IntentCode string                 `json:"intent_code"`
	Params     map[string]interface{} `json:"params,omitempty"`
}

type BootstrapCompactionOptions struct {
	MaxDepth        int `json:"max_depth"`
	MaxArrayItems   int `json:"max_array_items"`
	MaxObjectKeys   int `json:"max_object_keys"`
	MaxStringChars  int `json:"max_string_chars"`
	MaxPayloadBytes int `json:"max_payload_bytes"`
}

type FirstTurnBootstrapOptions struct {
	IntentCalls      []BootstrapIntentCall      `json:"intent_calls"`
	FailurePolicy    BootstrapFailurePolicy     `json:"failure_policy"`
	ToolCallIDPrefix string                     `json:"tool_call_id_prefix"`
	RequireSessionID bool                       `json:"require_session_id"`
	Compaction       BootstrapCompactionOptions `json:"compaction"`
}

// SecurityOptions holds security configuration
type SecurityOptions struct {
	// Rate limiting
	MaxRequestsPerMinute  int     // Maximum requests per minute (default: 10)
	MaxRequestsPerHour    int     // Maximum requests per hour (default: 100)
	MaxRequestsPerDay     int     // Maximum requests per day (default: 1000)
	SpamThreshold         float64 // Spam detection threshold 0.0-1.0 (default: 0.5)
	EnableSecurityLogging bool    // Enable security logging (default: true)
	UserIDField           string  // Field to identify user (default: "ParticipantName")
}

type UserMessage struct {
	Message         string `json:"message"`
	ParticipantName string `json:"participant_name"`
}

type AIResponse struct {
	Response interface{} `json:"response"`
}

type ResponseProcessor interface {
	Process(data interface{}) (string, error)
}

type DefaultResponseProcessor struct{}

func (p *DefaultResponseProcessor) Process(data interface{}) (string, error) {
	switch v := data.(type) {
	case string:
		return v, nil
	case map[string]interface{}, []interface{}:
		processedData, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "", err
		}
		return string(processedData), nil
	default:
		processedData, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(processedData), nil
	}
}

type ToolCacheKey struct {
	FunctionName       string
	Arguments          string
	ToolDefinitionHash string // Hash dari tool definition untuk invalidate cache saat tool berubah
}

type AuthFailureReason string

const (
	AuthFailureReasonRateLimit AuthFailureReason = "rate_limit"
	AuthFailureReasonFull      AuthFailureReason = "full"
	AuthFailureReasonTimeout   AuthFailureReason = "timeout"
	AuthFailureReasonAuth      AuthFailureReason = "auth"
	AuthFailureReasonUnknown   AuthFailureReason = "unknown"
)

type AuthSelection struct {
	Provider  string
	ProfileID string
	Token     string
}

// AuthManager mengelola pemilihan auth profile/token per provider.
type AuthManager interface {
	ResolveAuth(ctx context.Context, sessionID string, provider string) (*AuthSelection, error)
	MarkSuccess(ctx context.Context, sessionID string, provider string, profileID string) error
	MarkFailure(ctx context.Context, sessionID string, provider string, profileID string, reason AuthFailureReason) error
}

// AuthRateLimitRecorder is an optional extension for auth managers that can
// persist rate-limit snapshots per profile.
type AuthRateLimitRecorder interface {
	RecordRateLimit(ctx context.Context, provider string, profileID string, statusCode int, headers map[string]string) error
}

type ReasoningEffort string

const (
	ReasoningEffortNone    ReasoningEffort = "none"
	ReasoningEffortMinimal ReasoningEffort = "minimal"
	ReasoningEffortLow     ReasoningEffort = "low"
	ReasoningEffortMedium  ReasoningEffort = "medium"
	ReasoningEffortHigh    ReasoningEffort = "high"
	ReasoningEffortXHigh   ReasoningEffort = "xhigh"
)

type ReasoningSummaryMode string

const (
	ReasoningSummaryOff      ReasoningSummaryMode = "off"
	ReasoningSummaryAuto     ReasoningSummaryMode = "auto"
	ReasoningSummaryConcise  ReasoningSummaryMode = "concise"
	ReasoningSummaryDetailed ReasoningSummaryMode = "detailed"
)

type ReasoningContinuityMode string

const (
	ReasoningContinuityDisabled        ReasoningContinuityMode = "disabled"
	ReasoningContinuityProviderManaged ReasoningContinuityMode = "provider_managed"
	ReasoningContinuityBackendPass     ReasoningContinuityMode = "backend_pass_through"
)

type ReasoningSummaryExposure string

const (
	ReasoningSummaryExposureInternal ReasoningSummaryExposure = "internal_only"
	ReasoningSummaryExposureApp      ReasoningSummaryExposure = "app_visible"
	ReasoningSummaryExposureUser     ReasoningSummaryExposure = "user_visible"
)

type CapabilitySupport string

const (
	CapabilitySupportSupported CapabilitySupport = "supported"
	CapabilitySupportIgnored   CapabilitySupport = "ignored"
	CapabilitySupportUnsafe    CapabilitySupport = "unsafe"
)

type TransportCapabilities struct {
	Reasoning           CapabilitySupport `json:"reasoning,omitempty" bson:"reasoning,omitempty"`
	ReasoningSummary    CapabilitySupport `json:"reasoning_summary,omitempty" bson:"reasoning_summary,omitempty"`
	ReasoningContinuity CapabilitySupport `json:"reasoning_continuity,omitempty" bson:"reasoning_continuity,omitempty"`
}

type ReasoningConfig struct {
	Effort        ReasoningEffort          `json:"effort,omitempty" bson:"effort,omitempty"`
	Summary       ReasoningSummaryMode     `json:"summary,omitempty" bson:"summary,omitempty"`
	Continuity    ReasoningContinuityMode  `json:"continuity,omitempty" bson:"continuity,omitempty"`
	ExposeSummary ReasoningSummaryExposure `json:"expose_summary,omitempty" bson:"expose_summary,omitempty"`
}
