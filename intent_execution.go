package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	intentExecutionCodeToolNotFound          = "tool_not_found"
	intentExecutionCodeInvalidArguments      = "invalid_tool_arguments"
	intentExecutionCodeInvalidArgumentFormat = "invalid_tool_argument_format"
	intentExecutionCodeInvalidArgumentType   = "invalid_tool_argument_type"
	intentExecutionCodeIntentExecution       = "intent_execution_failed"
	intentExecutionCodeInvalidResponse       = "invalid_intent_response"
)

type intentExecutionState struct {
	IntentsByCode        map[string]Intent
	AvailableTools       []string
	ToolDefinitionHashes map[string]string
	Processor            ResponseProcessor
}

type intentExecutionError struct {
	Code          string
	RequestedTool string
	Cause         error
}

func (e *intentExecutionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.RequestedTool)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.RequestedTool, e.Cause)
}

func (e *intentExecutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newIntentExecutionError(code string, requestedTool string, cause error) *intentExecutionError {
	return &intentExecutionError{
		Code:          code,
		RequestedTool: requestedTool,
		Cause:         cause,
	}
}

func isRecoverableToolExecutionCode(code string) bool {
	switch code {
	case intentExecutionCodeToolNotFound,
		intentExecutionCodeInvalidArguments,
		intentExecutionCodeInvalidArgumentFormat,
		intentExecutionCodeInvalidArgumentType:
		return true
	default:
		return false
	}
}

func (c *CsAI) buildIntentExecutionState() *intentExecutionState {
	return c.buildIntentExecutionStateWithIntents(c.intents)
}

func (c *CsAI) buildIntentExecutionStateWithIntents(intents []Intent) *intentExecutionState {
	intentsByCode := make(map[string]Intent, len(intents))
	toolDefinitionHashes := make(map[string]string, len(intents))
	availableTools := make([]string, 0, len(intents))
	for _, intent := range intents {
		intentCode := intent.Code()
		intentsByCode[intentCode] = intent
		availableTools = append(availableTools, intentCode)

		if hash, err := generateToolDefinitionHash(intent); err == nil {
			toolDefinitionHashes[intentCode] = hash
		}
	}
	sort.Strings(availableTools)

	return &intentExecutionState{
		IntentsByCode:        intentsByCode,
		AvailableTools:       availableTools,
		ToolDefinitionHashes: toolDefinitionHashes,
		Processor:            &DefaultResponseProcessor{},
	}
}

func (c *CsAI) executeIntentHandler(
	ctx context.Context,
	sessionID string,
	userMessage UserMessage,
	functionName string,
	rawArguments string,
	executionState *intentExecutionState,
) (data interface{}, paramMap map[string]interface{}, err error) {
	intent, exists := executionState.IntentsByCode[functionName]
	if !exists {
		return nil, nil, newIntentExecutionError(intentExecutionCodeToolNotFound, functionName, nil)
	}

	if strings.TrimSpace(rawArguments) == "" {
		rawArguments = "{}"
	}

	paramTemplate := intent.Param()
	p := paramTemplate
	if err := json.Unmarshal([]byte(rawArguments), &p); err != nil {
		return nil, nil, newIntentExecutionError(intentExecutionCodeInvalidArguments, functionName, err)
	}

	paramMap, ok := p.(map[string]interface{})
	if !ok {
		return nil, nil, newIntentExecutionError(intentExecutionCodeInvalidArgumentFormat, functionName, nil)
	}

	paramMap, err = normalizeToolArguments(paramTemplate, paramMap)
	if err != nil {
		return nil, nil, newIntentExecutionError(intentExecutionCodeInvalidArgumentType, functionName, err)
	}

	middlewareCtx := &MiddlewareContext{
		SessionID:       sessionID,
		IntentCode:      intent.Code(),
		UserMessage:     userMessage,
		Parameters:      paramMap,
		StartTime:       time.Now(),
		Metadata:        make(map[string]interface{}),
		PreviousResults: make([]interface{}, 0),
	}

	finalHandler := func(ctx context.Context, mctx *MiddlewareContext) (interface{}, error) {
		return intent.Handle(ctx, mctx.Parameters)
	}

	data, err = c.middlewareChain.Execute(ctx, middlewareCtx, finalHandler)
	if err != nil {
		return nil, nil, newIntentExecutionError(intentExecutionCodeIntentExecution, functionName, err)
	}

	if err := validateResponse(data, paramMap); err != nil {
		return nil, nil, newIntentExecutionError(intentExecutionCodeInvalidResponse, functionName, err)
	}

	return data, paramMap, nil
}

func (c *CsAI) executeIntentToolCall(
	ctx context.Context,
	sessionID string,
	userMessage UserMessage,
	tool ToolCall,
	executionState *intentExecutionState,
) (toolResponse Message, isInvalid bool, err error) {
	data, _, execErr := c.executeIntentHandler(
		ctx,
		sessionID,
		userMessage,
		tool.Function.Name,
		tool.Function.Arguments,
		executionState,
	)
	if execErr != nil {
		var intentErr *intentExecutionError
		if errorsAsIntentExecution(execErr, &intentErr) && isRecoverableToolExecutionCode(intentErr.Code) {
			return buildToolErrorMessage(
				tool.Id,
				intentErr.Code,
				tool.Function.Name,
				executionState.AvailableTools,
			), true, nil
		}

		if errorsAsIntentExecution(execErr, &intentErr) {
			switch intentErr.Code {
			case intentExecutionCodeInvalidResponse:
				return Message{}, false, fmt.Errorf("invalid response for parameters: %v", intentErr.Cause)
			case intentExecutionCodeIntentExecution:
				return Message{}, false, intentErr.Cause
			}
		}
		return Message{}, false, execErr
	}

	processedContent, processErr := executionState.Processor.Process(data)
	if processErr != nil {
		return Message{}, false, fmt.Errorf("failed to process tool response: %v", processErr)
	}

	return Message{
		Content:    processedContent,
		Role:       Tool,
		ToolCallID: tool.Id,
	}, false, nil
}

func errorsAsIntentExecution(err error, target **intentExecutionError) bool {
	if err == nil || target == nil {
		return false
	}

	intentErr, ok := err.(*intentExecutionError)
	if !ok {
		return false
	}

	*target = intentErr
	return true
}
