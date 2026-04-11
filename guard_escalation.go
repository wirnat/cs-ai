package cs_ai

import (
	"context"
	"fmt"
	"strings"
)

func (c *CsAI) tryEscalateTurn(
	ctx context.Context,
	sessionID string,
	userMessage UserMessage,
	executionState *intentExecutionState,
	reason string,
) (Message, bool) {
	if executionState == nil || len(executionState.IntentsByCode) == 0 {
		return Message{}, false
	}

	toolName := ""
	for code := range executionState.IntentsByCode {
		if strings.EqualFold(strings.TrimSpace(code), "agent-takeover") {
			toolName = code
			break
		}
	}
	if toolName == "" {
		return Message{}, false
	}

	args := "{}"
	data, _, err := c.executeIntentHandler(
		ctx,
		sessionID,
		userMessage,
		toolName,
		args,
		executionState,
	)
	if err != nil {
		return Message{}, false
	}

	processed, err := executionState.Processor.Process(data)
	if err != nil {
		processed = ""
	}
	content := strings.TrimSpace(processed)
	if content == "" {
		content = "Permintaanmu sedang kami teruskan ke tim terkait ya."
	}

	emitStreamEvent(ctx, StreamEvent{
		Stage:    "turn",
		Type:     "turn.escalated",
		Status:   "escalated",
		ToolName: strings.TrimSpace(toolName),
		Message:  fmt.Sprintf("escalation dipicu karena %s", strings.TrimSpace(reason)),
	})

	return Message{
		Role:    Assistant,
		Content: sanitizeAssistantFinalMessage(content),
	}, true
}
