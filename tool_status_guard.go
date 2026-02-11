package cs_ai

import (
	"encoding/json"
	"strings"
)

// shouldOverrideAssistantWithToolMessage prevents success-style assistant text
// when the latest tool result still requires explicit confirmation.
func shouldOverrideAssistantWithToolMessage(messages Messages) (Message, bool) {
	if len(messages) < 2 {
		return Message{}, false
	}

	lastIdx := len(messages) - 1
	last := messages[lastIdx]
	if last.Role != Assistant || len(last.ToolCalls) > 0 {
		return Message{}, false
	}

	latestTool, ok := findLatestToolBefore(messages, lastIdx)
	if !ok {
		return Message{}, false
	}

	status, message, ok := extractToolStatusAndMessage(latestTool.Content)
	if !ok || !strings.EqualFold(status, "CONFIRMATION_REQUIRED") {
		return Message{}, false
	}

	if strings.TrimSpace(message) == "" {
		message = "Saya masih perlu konfirmasi sebelum perubahan dijalankan. Mohon konfirmasi lagi ya."
	}

	return Message{
		Content: message,
		Name:    last.Name,
		Role:    Assistant,
	}, true
}

func findLatestToolBefore(messages Messages, from int) (Message, bool) {
	for i := from - 1; i >= 0; i-- {
		if messages[i].Role == Tool {
			return messages[i], true
		}
	}
	return Message{}, false
}

func extractToolStatusAndMessage(content string) (status string, message string, ok bool) {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return "", "", false
	}

	rawStatus, hasStatus := payload["status"]
	if !hasStatus {
		return "", "", false
	}

	status, statusOK := rawStatus.(string)
	if !statusOK || strings.TrimSpace(status) == "" {
		return "", "", false
	}

	rawMessage, hasMessage := payload["message"]
	if hasMessage {
		if msg, msgOK := rawMessage.(string); msgOK {
			message = msg
		}
	}

	return status, message, true
}
