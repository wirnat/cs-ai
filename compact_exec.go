package cs_ai

import (
	"context"
	"fmt"
	"strings"
)

const (
	compactRecentConversationMaxMessages     = 4
	compactRecentConversationMaxMessageChars = 280
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
	recentConversation := buildCompactRecentConversation(rawMessages, compactRecentConversationMaxMessages, compactRecentConversationMaxMessageChars)
	resolvedSystemMessages := c.resolveSessionSystemMessages(sessionID, additionalSystemMessage)
	runtime := c.resolvedAgentRuntimeOptions()

	identifierCtx := withStageStreaming(ctx, AgentStageIdentifier, runtime.Streaming.Identifier)
	emitStageEvent(identifierCtx, AgentStageIdentifier, "agent.stage.started", "ok", "identifier stage dimulai")
	identifierOutput, err := runtime.Identifier.Identify(identifierCtx, IdentifierInput{
		SessionID:           sessionID,
		LatestUserMessage:   userMessage,
		ConversationSummary: runtimeState.ConversationSummary,
		ExternalState:       externalState,
		ToolManifest:        buildToolManifest(runtimeIntents),
		RecentConversation:  recentConversation,
	})
	if err != nil && runtime.Builtins.FallbackOnError {
		identifierOutput, err = (&builtInIdentifierAgent{owner: c}).Identify(identifierCtx, IdentifierInput{
			SessionID:           sessionID,
			LatestUserMessage:   userMessage,
			ConversationSummary: runtimeState.ConversationSummary,
			ExternalState:       externalState,
			ToolManifest:        buildToolManifest(runtimeIntents),
			RecentConversation:  recentConversation,
		})
	}
	if err != nil {
		emitStageEvent(identifierCtx, AgentStageIdentifier, "agent.stage.completed", "error", strings.TrimSpace(err.Error()))
		return Message{}, err
	}
	emitStageEvent(identifierCtx, AgentStageIdentifier, "agent.stage.completed", "ok", "identifier stage selesai")

	allowedToolCodes := normalizeIdentifierAllowedTools(identifierOutput.AllowedToolCodes, runtimeIntents)
	selectedIntents := c.selectRuntimeIntents(allowedToolCodes)
	selectedIntents = mergeIntentsByCode(selectedIntents, filterIntentsByCode(runtimeIntents, allowedToolCodes))
	if len(allowedToolCodes) == 0 && len(runtimeIntents) > 0 && !identifierOutput.CanAnswerDirect {
		selectedIntents = runtimeIntents
		allowedToolCodes = normalizeIdentifierAllowedTools(nil, runtimeIntents)
	}

	answerCtx := withStageStreaming(ctx, AgentStageAnswer, runtime.Streaming.Answer)
	emitStageEvent(answerCtx, AgentStageAnswer, "agent.stage.started", "ok", "answer stage dimulai")
	answerOutput, err := runtime.Answer.Answer(answerCtx, AnswerInput{
		SessionID:            sessionID,
		UserMessage:          userMessage,
		RuntimeIntents:       selectedIntents,
		AllowedToolCodes:     allowedToolCodes,
		ConversationSummary:  runtimeState.ConversationSummary,
		ExternalState:        externalState,
		ResolvedSystemPrompt: resolvedSystemMessages,
		ApplyBootstrap:       len(rawMessages) == 0,
		RecentConversation:   recentConversation,
	})
	if err != nil && runtime.Builtins.FallbackOnError {
		answerOutput, err = (&builtInAnswerAgent{owner: c}).Answer(answerCtx, AnswerInput{
			SessionID:            sessionID,
			UserMessage:          userMessage,
			RuntimeIntents:       selectedIntents,
			AllowedToolCodes:     allowedToolCodes,
			ConversationSummary:  runtimeState.ConversationSummary,
			ExternalState:        externalState,
			ResolvedSystemPrompt: resolvedSystemMessages,
			ApplyBootstrap:       len(rawMessages) == 0,
			RecentConversation:   recentConversation,
		})
	}
	if err != nil {
		emitStageEvent(answerCtx, AgentStageAnswer, "agent.stage.completed", "error", strings.TrimSpace(err.Error()))
		return Message{}, err
	}
	emitStageEvent(answerCtx, AgentStageAnswer, "agent.stage.completed", "ok", "answer stage selesai")

	persisted := append(make(Messages, 0, len(rawMessages)+len(answerOutput.DeltaMessages)), rawMessages...)
	persisted.Add(answerOutput.DeltaMessages...)
	if _, err := c.SaveSessionMessages(sessionID, persisted); err != nil {
		fmt.Printf("Warning: Failed to save session messages: %v\n", err)
	}

	traces := buildStructuredToolTraces(answerOutput.DeltaMessages)
	summaryCtx := withStageStreaming(ctx, AgentStageSummary, runtime.Streaming.Summary)
	emitStageEvent(summaryCtx, AgentStageSummary, "agent.stage.started", "ok", "summary stage dimulai")
	summaryOutput, summaryErr := runtime.Summary.Summarize(summaryCtx, SummaryInput{
		SessionID:           sessionID,
		PreviousSummary:     runtimeState.ConversationSummary,
		LatestUserMessage:   userMessage,
		LatestAssistantText: answerOutput.FinalMessage,
		ExternalState:       externalState,
		RecentMessages:      answerOutput.DeltaMessages,
		RecentConversation:  recentConversation,
		ToolEvidence:        traces,
	})
	if summaryErr != nil && runtime.Builtins.FallbackOnError {
		summaryOutput, summaryErr = (&builtInSummaryAgent{owner: c}).Summarize(summaryCtx, SummaryInput{
			SessionID:           sessionID,
			PreviousSummary:     runtimeState.ConversationSummary,
			LatestUserMessage:   userMessage,
			LatestAssistantText: answerOutput.FinalMessage,
			ExternalState:       externalState,
			RecentMessages:      answerOutput.DeltaMessages,
			RecentConversation:  recentConversation,
			ToolEvidence:        traces,
		})
	}
	if summaryErr != nil {
		emitStageEvent(summaryCtx, AgentStageSummary, "agent.stage.completed", "error", strings.TrimSpace(summaryErr.Error()))
	} else {
		emitStageEvent(summaryCtx, AgentStageSummary, "agent.stage.completed", "ok", "summary stage selesai")
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
	recentConversation []Message,
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
	withAnswerRequestMeta := func(base context.Context, requestKind string, hop int) context.Context {
		return WithHTTPLogMetadata(base, HTTPLogMetadata{
			SessionID:   strings.TrimSpace(sessionID),
			Stage:       "answer",
			RequestKind: strings.TrimSpace(requestKind),
			Hop:         hop,
		})
	}

	promptMessages := make(Messages, 0, 2)
	if summary := strings.TrimSpace(conversationSummary); summary != "" {
		promptMessages.Add(Message{
			Role:    Assistant,
			Content: compactSummaryAssistantPrefix + summary,
		})
	}
	if recentText := strings.TrimSpace(stringifyRecentConversation(recentConversation)); recentText != "" {
		promptMessages.Add(Message{
			Role:    Assistant,
			Content: compactRecentContextPrefix + recentText,
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
	currentUserPromptIndex := len(promptMessages) - 1
	aiResponse, err := c.sendWithIntentsForSession(withAnswerRequestMeta(ctx, "answer.initial", 0), "", promptMessages, runtimeIntents, resolvedSystemPrompt...)
	if err != nil {
		return AnswerOutput{}, err
	}
	appendUsage(aiResponse)
	deltaMessages.Add(aiResponse)
	promptMessages.Add(aiResponse)

	toolCache := make(map[ToolCacheKey]Message)
	guardPolicy := effectiveGuardPolicy(ctx, c.options.Streaming)
	loopCount := 0
	invalidToolCalls := 0
	successfulToolCalls := 0
	maxInvalidToolCalls := guardPolicy.MaxToolErrorStreak
	lastToolCallSignature := ""
	repeatedNoProgressLoops := 0
	maxRepeatedNoProgressLoops := guardPolicy.MaxSameSignatureRepeat
	consecutiveNoProgressLoops := 0
	maxConsecutiveNoProgressLoops := guardPolicy.MaxNoProgressLoops
	followUpRetryCount := 0
	const maxFollowUpRetryCount = 2
	availableToolCodes := manifestCodes(buildToolManifest(runtimeIntents))
	maxLoop := guardPolicy.MaxHopsPerTurn
	groundingRepairOptions := normalizeGroundingRepairOptions(c.options.GroundingRepair)
	groundingRepairAttempts := 0

	for {
		if len(aiResponse.ToolCalls) == 0 {
			latestTool, hasLatestTool := findLatestToolAfterIndex(promptMessages, currentUserPromptIndex)
			if !hasLatestTool {
				if groundingRepairOptions.Enabled &&
					groundingRepairAttempts < groundingRepairOptions.MaxAttempts &&
					strings.TrimSpace(aiResponse.Content) != "" {
					repairCtx := withAnswerRequestMeta(ctx, "answer.grounding_verifier", loopCount)
					needsRepair, reason := c.evaluateGroundingRepairNeed(
						repairCtx,
						userMessage.Message,
						aiResponse.Content,
						availableToolCodes,
					)
					if needsRepair {
						groundingRepairAttempts++
						repairInstruction := buildGroundingRepairInstruction(
							reason,
							availableToolCodes,
							groundingRepairAttempts,
						)
						aiResponse, err = c.sendWithIntentsForSession(
							withAnswerRequestMeta(ctx, "answer.grounding_repair", loopCount),
							"",
							promptMessages,
							runtimeIntents,
							append(resolvedSystemPrompt, repairInstruction)...,
						)
						if err != nil {
							return AnswerOutput{}, err
						}
						appendUsage(aiResponse)
						deltaMessages.Add(aiResponse)
						promptMessages.Add(aiResponse)
						continue
					}
				}
				break
			}

			requiredToolCodes, followUpInstruction, needsFollowUp := extractToolFollowUpCodes(latestTool.Content, availableToolCodes)
			if !needsFollowUp {
				break
			}

			if followUpRetryCount >= maxFollowUpRetryCount {
				status, message, ok := extractToolStatusAndMessage(latestTool.Content)
				fallbackContent := strings.TrimSpace(message)
				if !ok || fallbackContent == "" {
					fallbackContent = "Proses belum bisa dilanjutkan sekarang, mohon coba lagi sebentar ya."
				}

				fallback := withAggregatedUsage(Message{
					Role:    Assistant,
					Content: fallbackContent,
				})
				deltaMessages.Add(fallback)
				return AnswerOutput{
					RawMessage:    fallback,
					DeltaMessages: deltaMessages,
					FinalMessage:  strings.TrimSpace(fallback.Content),
					Warnings: []string{
						fmt.Sprintf("compact answer follow-up gate exhausted after status %s", strings.TrimSpace(status)),
					},
				}, nil
			}

			followUpRetryCount++
			gateInstruction := buildToolFollowUpGateInstruction(requiredToolCodes, followUpInstruction)
			aiResponse, err = c.sendWithIntentsForSession(
				withAnswerRequestMeta(ctx, "answer.followup_gate", loopCount),
				"",
				promptMessages,
				runtimeIntents,
				append(resolvedSystemPrompt, gateInstruction)...,
			)
			if err != nil {
				return AnswerOutput{}, err
			}
			appendUsage(aiResponse)
			deltaMessages.Add(aiResponse)
			promptMessages.Add(aiResponse)
			continue
		}

		loopCount++
		if loopCount > maxLoop {
			emitStreamEvent(ctx, StreamEvent{
				Stage:   "answer",
				Type:    "guard.triggered",
				Status:  "error",
				ErrCode: "max_hops_per_turn",
				Message: "batas hop per turn tercapai",
				Hop:     loopCount,
			})
			if escalatedMessage, escalated := c.tryEscalateTurn(ctx, sessionID, userMessage, executionState, "max_hops_per_turn"); escalated {
				escalatedMessage = withAggregatedUsage(escalatedMessage)
				deltaMessages.Add(escalatedMessage)
				return AnswerOutput{
					RawMessage:    escalatedMessage,
					DeltaMessages: deltaMessages,
					FinalMessage:  strings.TrimSpace(escalatedMessage.Content),
					Warnings:      []string{"compact answer escalated after loop guard"},
				}, nil
			}
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
		currentToolCallSignature := buildToolCallSignature(aiResponse.ToolCalls)
		for _, tool := range aiResponse.ToolCalls {
			emitStreamEvent(ctx, StreamEvent{
				Stage:    "answer",
				Type:     "tool.call.started",
				Status:   "ok",
				Hop:      loopCount,
				ToolName: strings.TrimSpace(tool.Function.Name),
			})
			cacheKey := ToolCacheKey{
				FunctionName: tool.Function.Name,
				Arguments:    tool.Function.Arguments,
			}
			if cached, ok := toolCache[cacheKey]; ok {
				cached.ToolCallID = tool.Id
				toolMessages.Add(cached)
				successfulToolCalls++
				emitStreamEvent(ctx, StreamEvent{
					Stage:    "answer",
					Type:     "tool.call.completed",
					Status:   "ok",
					Hop:      loopCount,
					ToolName: strings.TrimSpace(tool.Function.Name),
					Message:  "tool response diambil dari cache",
				})
				continue
			}

			toolResponse, isInvalid, execErr := c.executeIntentToolCall(ctx, sessionID, userMessage, tool, executionState)
			if execErr != nil {
				return AnswerOutput{}, execErr
			}
			if isInvalid {
				invalidToolCalls++
				emitStreamEvent(ctx, StreamEvent{
					Stage:    "answer",
					Type:     "tool.call.completed",
					Status:   "error",
					Hop:      loopCount,
					ToolName: strings.TrimSpace(tool.Function.Name),
					ErrCode:  "invalid_tool_call",
				})
			} else {
				toolCache[cacheKey] = toolResponse
				successfulToolCalls++
				emitStreamEvent(ctx, StreamEvent{
					Stage:    "answer",
					Type:     "tool.call.completed",
					Status:   "ok",
					Hop:      loopCount,
					ToolName: strings.TrimSpace(tool.Function.Name),
				})
			}
			toolMessages.Add(toolResponse)
		}

		if toolResponsesIndicateProgress(toolMessages) {
			consecutiveNoProgressLoops = 0
			repeatedNoProgressLoops = 0
		} else {
			consecutiveNoProgressLoops++
			if currentToolCallSignature != "" && currentToolCallSignature == lastToolCallSignature {
				repeatedNoProgressLoops++
			} else {
				repeatedNoProgressLoops = 1
			}
		}
		lastToolCallSignature = currentToolCallSignature
		deltaMessages.Add(toolMessages...)
		promptMessages.Add(toolMessages...)

		if invalidToolCalls >= maxInvalidToolCalls && successfulToolCalls == 0 {
			emitStreamEvent(ctx, StreamEvent{
				Stage:   "answer",
				Type:    "guard.triggered",
				Status:  "error",
				ErrCode: "max_tool_error_streak",
				Message: "tool error streak melebihi batas",
				Hop:     loopCount,
			})
			if escalatedMessage, escalated := c.tryEscalateTurn(ctx, sessionID, userMessage, executionState, "max_tool_error_streak"); escalated {
				escalatedMessage = withAggregatedUsage(escalatedMessage)
				deltaMessages.Add(escalatedMessage)
				return AnswerOutput{
					RawMessage:    escalatedMessage,
					DeltaMessages: deltaMessages,
					FinalMessage:  strings.TrimSpace(escalatedMessage.Content),
					Warnings:      []string{"compact answer escalated after tool error streak"},
				}, nil
			}
			fallback := withAggregatedUsage(buildToolSafetyFallbackMessage(userMessage.ParticipantName))
			deltaMessages.Add(fallback)
			return AnswerOutput{
				RawMessage:    fallback,
				DeltaMessages: deltaMessages,
				FinalMessage:  strings.TrimSpace(fallback.Content),
				Warnings:      []string{"compact answer safety fallback after invalid tool calls"},
			}, nil
		}
		if repeatedNoProgressLoops >= maxRepeatedNoProgressLoops || consecutiveNoProgressLoops >= maxConsecutiveNoProgressLoops {
			emitStreamEvent(ctx, StreamEvent{
				Stage:   "answer",
				Type:    "guard.triggered",
				Status:  "error",
				ErrCode: "max_no_progress_loops",
				Message: "tool loop berulang tanpa progres",
				Hop:     loopCount,
			})
			if escalatedMessage, escalated := c.tryEscalateTurn(ctx, sessionID, userMessage, executionState, "max_no_progress_loops"); escalated {
				escalatedMessage = withAggregatedUsage(escalatedMessage)
				deltaMessages.Add(escalatedMessage)
				return AnswerOutput{
					RawMessage:    escalatedMessage,
					DeltaMessages: deltaMessages,
					FinalMessage:  strings.TrimSpace(escalatedMessage.Content),
					Warnings:      []string{"compact answer escalated after no-progress guard"},
				}, nil
			}
			fallback := withAggregatedUsage(buildToolNoProgressFallbackMessage(userMessage.ParticipantName, successfulToolCalls > 0))
			deltaMessages.Add(fallback)
			return AnswerOutput{
				RawMessage:    fallback,
				DeltaMessages: deltaMessages,
				FinalMessage:  strings.TrimSpace(fallback.Content),
				Warnings:      []string{"compact answer fallback after no-progress tool loops"},
			}, nil
		}

		aiResponse, err = c.sendWithIntentsForSession(withAnswerRequestMeta(ctx, "answer.after_tools", loopCount), "", promptMessages, runtimeIntents, resolvedSystemPrompt...)
		if err != nil {
			return AnswerOutput{}, err
		}
		appendUsage(aiResponse)
		deltaMessages.Add(aiResponse)
		promptMessages.Add(aiResponse)
	}

	finalMessage := withAggregatedUsage(aiResponse)
	if finalMessage.Role != Assistant || len(finalMessage.ToolCalls) > 0 || strings.TrimSpace(finalMessage.Content) == "" {
		fallback := withAggregatedUsage(buildToolNoProgressFallbackMessage(userMessage.ParticipantName, successfulToolCalls > 0))
		deltaMessages.Add(fallback)
		return AnswerOutput{
			RawMessage:    fallback,
			DeltaMessages: deltaMessages,
			FinalMessage:  strings.TrimSpace(fallback.Content),
			Warnings:      []string{"compact answer fallback karena final assistant kosong/tidak valid"},
		}, nil
	}
	return AnswerOutput{
		RawMessage:    finalMessage,
		DeltaMessages: deltaMessages,
		FinalMessage:  strings.TrimSpace(finalMessage.Content),
	}, nil
}

func buildCompactRecentConversation(messages Messages, maxMessages int, maxChars int) []Message {
	if len(messages) == 0 || maxMessages <= 0 {
		return nil
	}

	filtered := make([]Message, 0, maxMessages)
	for i := len(messages) - 1; i >= 0 && len(filtered) < maxMessages; i-- {
		msg := messages[i]
		if msg.Role != User && msg.Role != Assistant {
			continue
		}
		if msg.Role == Assistant && len(msg.ToolCalls) > 0 {
			continue
		}

		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		msg.Content = trimCompactMessage(content, maxChars)
		filtered = append(filtered, msg)
	}

	for left, right := 0, len(filtered)-1; left < right; left, right = left+1, right-1 {
		filtered[left], filtered[right] = filtered[right], filtered[left]
	}

	return filtered
}

func trimCompactMessage(content string, maxChars int) string {
	content = strings.TrimSpace(content)
	if maxChars <= 0 || len(content) <= maxChars {
		return content
	}
	if maxChars <= 3 {
		return content[:maxChars]
	}
	return strings.TrimSpace(content[:maxChars-3]) + "..."
}
