package cs_ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type summaryAgentStub struct {
	output SummaryOutput
	err    error
	calls  int
}

func (s *summaryAgentStub) Summarize(ctx context.Context, input SummaryInput) (SummaryOutput, error) {
	s.calls++
	return s.output, s.err
}

type identifierAgentStub struct {
	output IdentifierOutput
	err    error
	calls  int
	inputs []IdentifierInput
}

func (s *identifierAgentStub) Identify(ctx context.Context, input IdentifierInput) (IdentifierOutput, error) {
	s.calls++
	s.inputs = append(s.inputs, input)
	return s.output, s.err
}

type answerAgentStub struct {
	output AnswerOutput
	err    error
	calls  int
	inputs []AnswerInput
}

type instructionProviderStub struct {
	lines []string
	calls int
}

func (s *answerAgentStub) Answer(ctx context.Context, input AnswerInput) (AnswerOutput, error) {
	s.calls++
	s.inputs = append(s.inputs, input)
	return s.output, s.err
}

func (s *instructionProviderStub) InstructionsForAgent(ctx context.Context, input AgentInstructionContext) []string {
	s.calls++
	return append([]string(nil), s.lines...)
}

func TestResolvedAgentRuntimeOptions_UsesBuiltinsByDefault(t *testing.T) {
	cs := newTestCsAIWithInMemoryStorage(t)

	runtime := cs.resolvedAgentRuntimeOptions()
	if runtime.Strategy != ContextStrategyLegacy {
		t.Fatalf("expected default strategy legacy, got %s", runtime.Strategy)
	}
	if runtime.Summary == nil || runtime.Identifier == nil || runtime.Answer == nil {
		t.Fatalf("expected default built-in agents to be resolved")
	}
}

func TestExecCompact_UsesInjectedAgentsAndPersistsSummary(t *testing.T) {
	cs := newTestCsAIWithInMemoryStorage(t)
	if _, err := cs.SaveSessionMessages("compact-1", []Message{
		{Role: User, Content: "saya mau haircut"},
		{Role: Assistant, Content: "Untuk barber-nya ada request tertentu kak?"},
	}); err != nil {
		t.Fatalf("failed to seed session messages: %v", err)
	}

	summary := &summaryAgentStub{
		output: SummaryOutput{
			ConversationSummary: "user sudah pilih lucas",
		},
	}
	identifier := &identifierAgentStub{
		output: IdentifierOutput{
			Route:           "answer_direct",
			NeedTool:        false,
			CanAnswerDirect: true,
		},
	}
	answer := &answerAgentStub{
		output: AnswerOutput{
			RawMessage: Message{
				Role:    Assistant,
				Content: "Siap kak, saya catat Lucas ya.",
			},
			DeltaMessages: []Message{
				{Role: User, Content: "oke lucas aja", Name: "tester"},
				{Role: Assistant, Content: "Siap kak, saya catat Lucas ya."},
			},
			FinalMessage: "Siap kak, saya catat Lucas ya.",
		},
	}

	cs.options.AgentRuntime = &AgentRuntimeOptions{
		Strategy:   ContextStrategyCompactBackend,
		Summary:    summary,
		Identifier: identifier,
		Answer:     answer,
	}

	resp, err := cs.Exec(context.Background(), "compact-1", UserMessage{
		Message:         "oke lucas aja",
		ParticipantName: "tester",
	})
	if err != nil {
		t.Fatalf("exec compact returned error: %v", err)
	}
	if resp.Content != "Siap kak, saya catat Lucas ya." {
		t.Fatalf("unexpected response: %q", resp.Content)
	}
	if summary.calls != 1 || identifier.calls != 1 || answer.calls != 1 {
		t.Fatalf("expected each custom agent to run exactly once, got summary=%d identifier=%d answer=%d", summary.calls, identifier.calls, answer.calls)
	}
	if len(identifier.inputs) != 1 || len(identifier.inputs[0].RecentConversation) != 2 {
		t.Fatalf("expected identifier to receive 2 recent conversation messages, got %#v", identifier.inputs)
	}
	if len(answer.inputs) != 1 || len(answer.inputs[0].RecentConversation) != 2 {
		t.Fatalf("expected answer to receive 2 recent conversation messages, got %#v", answer.inputs)
	}

	messages, err := cs.GetSessionMessages("compact-1")
	if err != nil {
		t.Fatalf("failed to get session messages: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("expected 4 raw messages to persist, got %d", len(messages))
	}
	if messages[0].Role != User || messages[1].Role != Assistant || messages[2].Role != User || messages[3].Role != Assistant {
		t.Fatalf("unexpected raw transcript roles: %#v", messages)
	}

	state, err := cs.GetSessionState("compact-1")
	if err != nil {
		t.Fatalf("failed to get session state: %v", err)
	}
	internal, ok := state[agentRuntimeStateKey].(map[string]interface{})
	if !ok {
		t.Fatalf("expected internal compact runtime state, got %#v", state[agentRuntimeStateKey])
	}
	if internal["conversation_summary"] != "user sudah pilih lucas" {
		t.Fatalf("unexpected stored summary: %#v", internal["conversation_summary"])
	}
}

func TestBuildCompactRecentConversation_FiltersToolMessagesAndKeepsLatestTurns(t *testing.T) {
	messages := Messages{
		{Role: User, Content: "halo"},
		{Role: Assistant, Content: "Malam kak, mau haircut ya?"},
		{Role: Assistant, ToolCalls: []ToolCall{{Id: "call-1"}}},
		{Role: Tool, Content: "{\"status\":\"SUCCESS\"}"},
		{Role: User, Content: "jam 10 pagi"},
		{Role: Assistant, Content: "Jam 10 pagi berarti 10:00 ya; besok atau tanggal lain?"},
	}

	recent := buildCompactRecentConversation(messages, 4, 280)
	if len(recent) != 4 {
		t.Fatalf("expected 4 recent conversation messages, got %d", len(recent))
	}
	if recent[0].Content != "halo" || recent[1].Content != "Malam kak, mau haircut ya?" {
		t.Fatalf("unexpected earliest recent messages: %#v", recent)
	}
	if recent[2].Content != "jam 10 pagi" || recent[3].Content != "Jam 10 pagi berarti 10:00 ya; besok atau tanggal lain?" {
		t.Fatalf("unexpected latest recent messages: %#v", recent)
	}
}

func TestResolveAgentInstructions_MergesProviderAndContextByStage(t *testing.T) {
	cs := newTestCsAIWithInMemoryStorage(t)
	provider := &instructionProviderStub{
		lines: []string{"provider-line", "shared-line"},
	}
	cs.options.AgentRuntime = &AgentRuntimeOptions{
		InstructionProvider: provider,
	}

	ctx := WithAgentRuntimeInstructions(context.Background(), AgentRuntimeInstructions{
		Shared:  []string{"shared-line", "ctx-shared"},
		Answer:  []string{"ctx-answer"},
		Summary: []string{"ctx-summary"},
	})

	lines := cs.resolveAgentInstructions(ctx, AgentInstructionContext{
		Stage:     AgentStageAnswer,
		SessionID: "s-1",
	})

	if provider.calls != 1 {
		t.Fatalf("expected provider to be called once, got %d", provider.calls)
	}
	if len(lines) != 4 {
		t.Fatalf("expected 4 merged lines, got %d (%v)", len(lines), lines)
	}
	expected := []string{"provider-line", "shared-line", "ctx-shared", "ctx-answer"}
	for i, want := range expected {
		if lines[i] != want {
			t.Fatalf("line %d = %q, want %q", i, lines[i], want)
		}
	}
}

func TestInvokeAgentModel_UsesReasoningProfile(t *testing.T) {
	var captured map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "resp_agent_reasoning",
			"model": "gpt-5.4",
			"output": []map[string]interface{}{
				{
					"type": "message",
					"role": "assistant",
					"content": []map[string]interface{}{
						{
							"type": "output_text",
							"text": "ok",
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &reasoningTestModel{apiURL: server.URL}

	_, err := cs.invokeAgentModel(context.Background(), AgentModelProfile{
		Model:           "gpt-5.4",
		ReasoningEffort: "low",
	}, []Message{
		{Role: System, Content: "sys"},
		{Role: User, Content: "halo"},
	})
	if err != nil {
		t.Fatalf("invokeAgentModel returned error: %v", err)
	}

	reasoning, ok := captured["reasoning"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected reasoning payload, got %#v", captured["reasoning"])
	}
	if toString(reasoning["effort"]) != "low" {
		t.Fatalf("unexpected reasoning effort: %#v", reasoning["effort"])
	}
}

func TestBuiltInAnswerAgent_UsesAnswerReasoningProfileInCompactMode(t *testing.T) {
	var captured map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "resp_answer_reasoning",
			"model": "gpt-5.4",
			"output": []map[string]interface{}{
				{
					"type": "message",
					"role": "assistant",
					"content": []map[string]interface{}{
						{
							"type": "output_text",
							"text": "Siap kak.",
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	cs := newTestCsAIWithInMemoryStorage(t)
	cs.Model = &reasoningTestModel{apiURL: server.URL}
	cs.options.Reasoning = &ReasoningConfig{
		Effort:  ReasoningEffortMedium,
		Summary: ReasoningSummaryDetailed,
	}
	cs.options.AgentRuntime = &AgentRuntimeOptions{
		Strategy: ContextStrategyCompactBackend,
		Summary:  passthroughSummaryAgent{},
		Identifier: &identifierAgentStub{
			output: IdentifierOutput{
				Route:           "answer_direct",
				NeedTool:        false,
				CanAnswerDirect: true,
			},
		},
		Models: AgentModelProfiles{
			Answer: AgentModelProfile{
				Reasoning: &ReasoningConfig{
					Effort:  ReasoningEffortLow,
					Summary: ReasoningSummaryOff,
				},
			},
		},
	}

	resp, err := cs.Exec(context.Background(), "compact-answer-reasoning", UserMessage{
		Message:         "halo",
		ParticipantName: "tester",
	})
	if err != nil {
		t.Fatalf("exec returned error: %v", err)
	}
	if resp.Content != "Siap kak." {
		t.Fatalf("unexpected response: %q", resp.Content)
	}

	reasoning, ok := captured["reasoning"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected reasoning payload, got %#v", captured["reasoning"])
	}
	if toString(reasoning["effort"]) != "low" {
		t.Fatalf("unexpected reasoning effort: %#v", reasoning["effort"])
	}
	if _, exists := reasoning["summary"]; exists {
		t.Fatalf("expected answer profile summary=off to suppress summary field, got %#v", reasoning["summary"])
	}
}
