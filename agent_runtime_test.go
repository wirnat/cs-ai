package cs_ai

import (
	"context"
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
}

func (s *identifierAgentStub) Identify(ctx context.Context, input IdentifierInput) (IdentifierOutput, error) {
	s.calls++
	return s.output, s.err
}

type answerAgentStub struct {
	output AnswerOutput
	err    error
	calls  int
}

func (s *answerAgentStub) Answer(ctx context.Context, input AnswerInput) (AnswerOutput, error) {
	s.calls++
	return s.output, s.err
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

	messages, err := cs.GetSessionMessages("compact-1")
	if err != nil {
		t.Fatalf("failed to get session messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 raw messages to persist, got %d", len(messages))
	}
	if messages[0].Role != User || messages[1].Role != Assistant {
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
