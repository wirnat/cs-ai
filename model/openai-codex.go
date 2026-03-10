package model

const OPENAI_CODEX = "openai-codex"

// openAICodex menggunakan transport Codex responses ala OpenClaw.
type openAICodex struct {
	modelName string
}

func NewOpenAICodex(modelName ...string) *openAICodex {
	selected := "gpt-5.4"
	if len(modelName) > 0 && modelName[0] != "" {
		selected = modelName[0]
	}
	return &openAICodex{modelName: selected}
}

func (o *openAICodex) ModelName() string {
	return o.modelName
}

func (o *openAICodex) ApiURL() string {
	return "https://chatgpt.com/backend-api/codex/responses"
}

func (o *openAICodex) ProviderName() string {
	return OPENAI_CODEX
}

func (o *openAICodex) APIMode() string {
	return "openai-codex-responses"
}

func (o *openAICodex) Train() []string {
	return []string{
		"Kamu adalah Customer service AI yang harus berbicara secara natural dan sopan.",
		"Jawaban harus ringkas, relevan, dan tidak mengarang data di luar system/tools.",
		"Jika data tidak tersedia, katakan jujur bahwa informasi tidak tersedia.",
	}
}
