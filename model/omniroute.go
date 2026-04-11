package model

const OMNIROUTE = "omniroute"

type omniRoute struct {
	modelName string
	apiURL    string
}

func NewOmniRoute(modelName string, apiURL string) *omniRoute {
	selectedModel := "kr/claude-sonnet-4.5"
	if modelName != "" {
		selectedModel = modelName
	}

	selectedURL := "http://localhost:20128/v1/chat/completions"
	if apiURL != "" {
		selectedURL = apiURL
	}

	return &omniRoute{
		modelName: selectedModel,
		apiURL:    selectedURL,
	}
}

func (o *omniRoute) ModelName() string {
	return o.modelName
}

func (o *omniRoute) ApiURL() string {
	return o.apiURL
}

func (o *omniRoute) ProviderName() string {
	return OMNIROUTE
}

func (o *omniRoute) APIMode() string {
	return "chat-completions"
}

func (o *omniRoute) Train() []string {
	return []string{
		"Kamu adalah Customer service AI yang harus berbicara secara natural dan sopan.",
		"Jawaban harus ringkas, relevan, dan tidak mengarang data di luar system/tools.",
		"Jika data tidak tersedia, katakan jujur bahwa informasi tidak tersedia.",
	}
}
