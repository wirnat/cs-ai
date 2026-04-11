package cs_ai

import (
	"encoding/json"
	"fmt"
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

func findLatestToolAfterIndex(messages Messages, from int) (Message, bool) {
	if from < -1 {
		from = -1
	}
	for i := len(messages) - 1; i > from; i-- {
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

func extractToolFollowUpCodes(content string, availableToolCodes []string) (codes []string, instruction string, ok bool) {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nil, "", false
	}

	nextAction := strings.ToLower(strings.TrimSpace(toString(payload["next_action"])))
	if isUserInputNextAction(nextAction) {
		return nil, "", false
	}

	status := strings.ToUpper(strings.TrimSpace(toString(payload["status"])))
	if status == "CONFIRMATION_REQUIRED" {
		return nil, "", false
	}

	codes = append(codes, parseStringSliceInterface(payload["required_tool_codes"])...)
	codes = append(codes, parseStringSliceInterface(payload["required_tools"])...)

	instruction = strings.TrimSpace(toString(payload["instruction"]))
	// Hanya infer tool dari instruction saat next_action tidak eksplisit.
	// Jika next_action ada (mis. ask_price_option/recheck_availability_or_input),
	// berarti umumnya menunggu input user dulu.
	if len(codes) == 0 && instruction != "" && nextAction == "" {
		lowerInstruction := strings.ToLower(instruction)
		normalizedAvailable := normalizeAllowedToolCodes(availableToolCodes)
		for _, code := range normalizedAvailable {
			if strings.Contains(lowerInstruction, strings.ToLower(strings.TrimSpace(code))) {
				codes = append(codes, code)
			}
		}
	}

	codes = normalizeAllowedToolCodes(codes)
	if len(codes) == 0 {
		return nil, instruction, false
	}

	return codes, instruction, true
}

func isUserInputNextAction(nextAction string) bool {
	nextAction = strings.ToLower(strings.TrimSpace(nextAction))
	if nextAction == "" {
		return false
	}
	if strings.HasPrefix(nextAction, "ask_") || strings.HasPrefix(nextAction, "collect_") || strings.HasPrefix(nextAction, "wait_") {
		return true
	}
	userInputHints := []string{
		"input",
		"choose",
		"select",
		"confirm",
		"retry",
	}
	for _, hint := range userInputHints {
		if strings.Contains(nextAction, hint) {
			return true
		}
	}
	return false
}

func buildToolFollowUpGateInstruction(requiredToolCodes []string, instruction string) string {
	tools := strings.Join(normalizeAllowedToolCodes(requiredToolCodes), ", ")
	rawInstruction := strings.TrimSpace(instruction)
	if rawInstruction == "" {
		rawInstruction = "(tidak ada)"
	}

	return fmt.Sprintf(
		"HARD GATE: Tool terakhir belum final dan mewajibkan follow-up tool (%s). Jangan jawab user dulu. Panggil tool yang diwajibkan hingga hasil siap dijelaskan ke user. Instruksi tool terakhir: %s",
		tools,
		rawInstruction,
	)
}

func parseStringSliceInterface(raw interface{}) []string {
	if raw == nil {
		return nil
	}

	if typed, ok := raw.([]string); ok {
		return append([]string(nil), typed...)
	}

	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(toString(item))
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}
