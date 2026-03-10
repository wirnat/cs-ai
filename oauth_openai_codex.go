package cs_ai

import (
	"os"
	"strings"
)

const OpenAICodexOAuthDefaultClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

func ResolveOpenAICodexClientID(explicit string) string {
	if value := strings.TrimSpace(explicit); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("CS_AI_OAUTH_CLIENT_ID")); value != "" {
		return value
	}
	return OpenAICodexOAuthDefaultClientID
}
