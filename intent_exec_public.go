package cs_ai

import (
	"context"
	"encoding/json"
	"strings"
)

type IntentExecutionOptions struct {
	AllowedToolCodes   []string
	AdditionalIntents  []Intent
	PersistToolMessage bool
	ToolCallID         string
}

type IntentExecutionResult struct {
	ToolCode    string                 `json:"tool_code"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Data        interface{}            `json:"data,omitempty"`
	ToolMessage Message                `json:"tool_message"`
}

// ExecuteIntent menjalankan sebuah intent/tool secara deterministik tanpa meminta
// model memilih tool terlebih dulu. Ini berguna untuk orchestrator backend yang
// ingin memaksa preflight tool tertentu sebelum masuk ke langkah LLM.
func (c *CsAI) ExecuteIntent(
	ctx context.Context,
	sessionID string,
	userMessage UserMessage,
	toolCode string,
	arguments map[string]interface{},
	opts IntentExecutionOptions,
) (IntentExecutionResult, error) {
	runtimeIntents := c.selectRuntimeIntents(opts.AllowedToolCodes)
	runtimeIntents = mergeIntentsByCode(runtimeIntents, opts.AdditionalIntents)
	executionState := c.buildIntentExecutionStateWithIntents(runtimeIntents)

	rawArguments := "{}"
	if len(arguments) > 0 {
		encoded, err := json.Marshal(arguments)
		if err != nil {
			return IntentExecutionResult{}, err
		}
		rawArguments = string(encoded)
	}

	data, paramMap, err := c.executeIntentHandler(
		ctx,
		sessionID,
		userMessage,
		strings.TrimSpace(toolCode),
		rawArguments,
		executionState,
	)
	if err != nil {
		return IntentExecutionResult{}, err
	}

	processedContent, err := executionState.Processor.Process(data)
	if err != nil {
		return IntentExecutionResult{}, err
	}

	toolMessage := Message{
		Content:    processedContent,
		Role:       Tool,
		ToolCallID: firstNonEmptyString(strings.TrimSpace(opts.ToolCallID), randomID("tool")),
	}
	toolMessage.PrepareForStorage()

	if opts.PersistToolMessage && strings.TrimSpace(sessionID) != "" {
		if err := c.AddMessageToSession(sessionID, toolMessage); err != nil {
			return IntentExecutionResult{}, err
		}
	}

	return IntentExecutionResult{
		ToolCode:    strings.TrimSpace(toolCode),
		Parameters:  paramMap,
		Data:        data,
		ToolMessage: toolMessage,
	}, nil
}
