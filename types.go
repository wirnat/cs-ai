package cs_ai

import (
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

	// === Cache & Session Options ===
	SessionTTL time.Duration // TTL untuk session messages (default: 12 jam)

	// === Security Options ===
	SecurityOptions *SecurityOptions // Security configuration

	// === First-turn bootstrap options ===
	FirstTurnBootstrap *FirstTurnBootstrapOptions // Optional server-side bootstrap intents for first user turn
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
