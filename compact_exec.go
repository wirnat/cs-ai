package cs_ai

import (
	"context"
	"fmt"
	"strings"
)

func (c *CsAI) execCompact(
	ctx context.Context,
	sessionID string,
	userMessage UserMessage,
	runtimeIntents []Intent,
	additionalSystemMessage ...string,
) (Message, error) {
	userID := userMessage.ParticipantName
	if userID == "" {
		userID = "anonymous"
	}
	if c.securityManager != nil {
		if err := c.securityManager.CheckSecurity(userID, sessionID, userMessage.Message); err != nil {
			return Message{}, fmt.Errorf("security check failed: %v", err)
		}
	}

	if len(additionalSystemMessage) > 0 {
		systemMessages := make([]Message, 0, len(additionalSystemMessage))
		for _, s := range additionalSystemMessage {
			systemMessages = append(systemMessages, Message{Content: s, Role: System})
		}
		if err := c.SaveSystemMessages(sessionID, systemMessages); err != nil {
			fmt.Printf("Warning: Failed to save system messages: %v\n", err)
		}
	}

	rawMessages, err := c.GetSessionMessages(sessionID)
	if err != nil {
		fmt.Printf("Warning: Failed to load session messages: %v\n", err)
	}
	runtimeState, rawState, err := c.loadAgentRuntimeState(sessionID)
	if err != nil {
		return Message{}, err
	}
	externalState := stripInternalRuntimeState(rawState)
	resolvedSystemMessages := c.resolveSessionSystemMessages(sessionID, additionalSystemMessage)
	runtime := c.resolvedAgentRuntimeOptions()

	identifierOutput, err := runtime.Identifier.Identify(ctx, IdentifierInput{
		SessionID:           sessionID,
		LatestUserMessage:   userMessage,
		ConversationSummary: runtimeState.ConversationSummary,
		ExternalState:       externalState,
		ToolManifest:        buildToolManifest(runtimeIntents),
	})
	if err != nil && runtime.Builtins.FallbackOnError {
		identifierOutput, err = (&builtInIdentifierAgent{owner: c}).Identify(ctx, IdentifierInput{
			SessionID:           sessionID,
			LatestUserMessage:   userMessage,
			ConversationSummary: runtimeState.ConversationSummary,
			ExternalState:       externalState,
			ToolManifest:        buildToolManifest(runtimeIntents),
		})
	}
	if err != nil {
		return Message{}, err
	}

	allowedToolCodes := normalizeIdentifierAllowedTools(identifierOutput.AllowedToolCodes, runtimeIntents)
	selectedIntents := c.selectRuntimeIntents(allowedToolCodes)
	selectedIntents = mergeIntentsByCode(selectedIntents, filterIntentsByCode(runtimeIntents, allowedToolCodes))
	if len(allowedToolCodes) == 0 && len(runtimeIntents) > 0 && !identifierOutput.CanAnswerDirect {
		selectedIntents = runtimeIntents
		allowedToolCodes = normalizeIdentifierAllowedTools(nil, runtimeIntents)
	}

	answerOutput, err := runtime.Answer.Answer(ctx, AnswerInput{
		SessionID:            sessionID,
		UserMessage:          userMessage,
		RuntimeIntents:       selectedIntents,
		AllowedToolCodes:     allowedToolCodes,
		ConversationSummary:  runtimeState.ConversationSummary,
		ExternalState:        externalState,
		ResolvedSystemPrompt: resolvedSystemMessages,
		ApplyBootstrap:       len(rawMessages) == 0,
	})
	if err != nil && runtime.Builtins.FallbackOnError {
		answerOutput, err = (&builtInAnswerAgent{owner: c}).Answer(ctx, AnswerInput{
			SessionID:            sessionID,
			UserMessage:          userMessage,
			RuntimeIntents:       selectedIntents,
			AllowedToolCodes:     allowedToolCodes,
			ConversationSummary:  runtimeState.ConversationSummary,
			ExternalState:        externalState,
			ResolvedSystemPrompt: resolvedSystemMessages,
			ApplyBootstrap:       len(rawMessages) == 0,
		})
	}
	if err != nil {
		return Message{}, err
	}

	persisted := append(make(Messages, 0, len(rawMessages)+len(answerOutput.DeltaMessages)), rawMessages...)
	persisted.Add(answerOutput.DeltaMessages...)
	if _, err := c.SaveSessionMessages(sessionID, persisted); err != nil {
		fmt.Printf("Warning: Failed to save session messages: %v\n", err)
	}

	traces := buildStructuredToolTraces(answerOutput.DeltaMessages)
	summaryOutput, summaryErr := runtime.Summary.Summarize(ctx, SummaryInput{
		SessionID:           sessionID,
		PreviousSummary:     runtimeState.ConversationSummary,
		LatestUserMessage:   userMessage,
		LatestAssistantText: answerOutput.FinalMessage,
		ExternalState:       externalState,
		RecentMessages:      answerOutput.DeltaMessages,
		ToolEvidence:        traces,
	})
	if summaryErr != nil && runtime.Builtins.FallbackOnError {
		summaryOutput, summaryErr = (&builtInSummaryAgent{owner: c}).Summarize(ctx, SummaryInput{
			SessionID:           sessionID,
			PreviousSummary:     runtimeState.ConversationSummary,
			LatestUserMessage:   userMessage,
			LatestAssistantText: answerOutput.FinalMessage,
			ExternalState:       externalState,
			RecentMessages:      answerOutput.DeltaMessages,
			ToolEvidence:        traces,
		})
	}
	if summaryErr == nil {
		runtimeState.ConversationSummary = strings.TrimSpace(summaryOutput.ConversationSummary)
		if len(summaryOutput.StatePatch) > 0 {
			for key, value := range summaryOutput.StatePatch {
				externalState[key] = value
			}
		}
		rawToSave := map[string]interface{}{}
		for key, value := range externalState {
			rawToSave[key] = value
		}
		if err := c.saveAgentRuntimeState(sessionID, rawToSave, runtimeState); err != nil {
			fmt.Printf("Warning: Failed to save compact runtime state: %v\n", err)
		}
	}

	return answerOutput.RawMessage, nil
}

func (c *CsAI) resolveSessionSystemMessages(sessionID string, additionalSystemMessage []string) []string {
	resolved := append([]string(nil), additionalSystemMessage...)
	if len(resolved) > 0 || strings.TrimSpace(sessionID) == "" {
		return resolved
	}
	persisted, err := c.GetSystemMessages(sessionID)
	if err != nil || len(persisted) == 0 {
		return resolved
	}
	for _, msg := range persisted {
		if msg.Role != System {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		resolved = append(resolved, content)
	}
	return resolved
}

func filterIntentsByCode(intents []Intent, codes []string) []Intent {
	if len(codes) == 0 || len(intents) == 0 {
		return nil
	}
	allowed := map[string]struct{}{}
	for _, code := range codes {
		if trimmed := strings.TrimSpace(code); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	result := make([]Intent, 0, len(intents))
	for _, intent := range intents {
		if intent == nil {
			continue
		}
		if _, ok := allowed[intent.Code()]; ok {
			result = append(result, intent)
		}
	}
	return result
}

func (c *CsAI) runCompactAnswerLoop(
	ctx context.Context,
	sessionID string,
	userMessage UserMessage,
	runtimeIntents []Intent,
	conversationSummary string,
	applyBootstrap bool,
	resolvedSystemPrompt ...string,
) (AnswerOutput, error) {
	usageAggregate := DeepSeekUsage{}
	appendUsage := func(msg Message) {
		if msg.Usage != nil {
			usageAggregate = usageAggregate.Add(*msg.Usage)
		}
	}
	withAggregatedUsage := func(msg Message) Message {
		normalized := usageAggregate.Normalize()
		if normalized.IsZero() {
			return msg
		}
		msg.AggregatedUsage = &normalized
		return msg
	}

	promptMessages := make(Messages, 0, 2)
	if summary := strings.TrimSpace(conversationSummary); summary != "" {
		promptMessages.Add(Message{
			Role:    Assistant,
			Content: compactSummaryAssistantPrefix + summary,
		})
	}
	deltaMessages := make(Messages, 0, 6)
	deltaMessages.Add(Message{
		Role:    User,
		Content: userMessage.Message,
		Name:    userMessage.ParticipantName,
	})

	executionState := c.buildIntentExecutionStateWithIntents(runtimeIntents)
	if applyBootstrap {
		if err := c.maybeApplyFirstTurnBootstrap(ctx, sessionID, &promptMessages, userMessage, executionState); err != nil {
			return AnswerOutput{}, err
		}
	}
	promptMessages.Add(Message{
		Role:    User,
		Content: userMessage.Message,
		Name:    userMessage.ParticipantName,
	})
	aiResponse, err := c.sendWithIntentsForSession(ctx, "", promptMessages, runtimeIntents, resolvedSystemPrompt...)
	if err != nil {
		return AnswerOutput{}, err
	}
	appendUsage(aiResponse)
	deltaMessages.Add(aiResponse)
	promptMessages.Add(aiResponse)

	toolCache := make(map[ToolCacheKey]Message)
	loopCount := 0
	for len(aiResponse.ToolCalls) > 0 {
		loopCount++
		if loopCount > 10 {
			fallback := withAggregatedUsage(buildToolLoopLimitFallbackMessage(userMessage.ParticipantName, true))
			deltaMessages.Add(fallback)
			return AnswerOutput{
				RawMessage:    fallback,
				DeltaMessages: deltaMessages,
				FinalMessage:  fallback.Content,
				Warnings:      []string{"compact answer loop limit reached"},
			}, nil
		}

		toolMessages := make(Messages, 0, len(aiResponse.ToolCalls))
		for _, tool := range aiResponse.ToolCalls {
			cacheKey := ToolCacheKey{
				FunctionName: tool.Function.Name,
				Arguments:    tool.Function.Arguments,
			}
			if cached, ok := toolCache[cacheKey]; ok {
				cached.ToolCallID = tool.Id
				toolMessages.Add(cached)
				continue
			}

			toolResponse, _, execErr := c.executeIntentToolCall(ctx, sessionID, userMessage, tool, executionState)
			if execErr != nil {
				return AnswerOutput{}, execErr
			}
			toolCache[cacheKey] = toolResponse
			toolMessages.Add(toolResponse)
		}

		deltaMessages.Add(toolMessages...)
		promptMessages.Add(toolMessages...)

		aiResponse, err = c.sendWithIntentsForSession(ctx, "", promptMessages, runtimeIntents, resolvedSystemPrompt...)
		if err != nil {
			return AnswerOutput{}, err
		}
		appendUsage(aiResponse)
		deltaMessages.Add(aiResponse)
		promptMessages.Add(aiResponse)
	}

	finalMessage := withAggregatedUsage(aiResponse)
	return AnswerOutput{
		RawMessage:    finalMessage,
		DeltaMessages: deltaMessages,
		FinalMessage:  strings.TrimSpace(finalMessage.Content),
	}, nil
}
