package cs_ai

type Modeler interface {
	ModelName() string
	ApiURL() string
	Train() []string
}

// ProviderModeler menambahkan metadata provider/API mode tanpa memecah interface lama.
type ProviderModeler interface {
	Modeler
	ProviderName() string
	APIMode() string
}

// APIMode constants.
const (
	APIModeChatCompletions      = "chat-completions"
	APIModeOpenAICodexResponses = "openai-codex-responses"
)
