package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/llm"
	"github.com/icoo-ai/icoo-ai/internal/tools"
)

func TestRuntimePromptRunsLoop(t *testing.T) {
	provider := newMockProvider([][]llm.CompletionEvent{{
		{Type: llm.CompletionEventMessageDelta, Delta: "hello"},
		{Type: llm.CompletionEventCompleted},
	}})
	loop, err := NewReactLoop(ReactLoopOptions{Provider: provider})
	if err != nil {
		t.Fatalf("NewReactLoop() error = %v", err)
	}
	runtime, err := NewRuntime(RuntimeOptions{
		Loop:  loop,
		Store: newMemorySessionStore(),
		CWD:   "E:/repo",
		Model: "gpt-4.1",
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

	sess, err := runtime.NewSession(context.Background(), NewSessionRequest{})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	events, err := runtime.Prompt(context.Background(), PromptRequest{SessionID: sess.ID, Prompt: "hi"})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	got, err := collectEvents(context.Background(), events)
	if err != nil {
		t.Fatalf("CollectEvents() error = %v", err)
	}
	if got[len(got)-1].Type != EventRunCompleted {
		t.Fatalf("last event = %s", got[len(got)-1].Type)
	}
	saved, err := runtime.LoadSession(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if len(saved.Messages) != 2 || saved.Messages[1].Content != "hello" {
		t.Fatalf("saved messages = %+v", saved.Messages)
	}
	if len(saved.Events) == 0 || saved.Events[len(saved.Events)-1].Type != EventRunCompleted {
		t.Fatalf("saved events = %+v", saved.Events)
	}
}

func TestRuntimePersistsToolAndApprovalSummaries(t *testing.T) {
	provider := newMockProvider([][]llm.CompletionEvent{
		{{Type: llm.CompletionEventToolCall, ToolCall: &tools.ToolCall{ID: "tc1", Name: "needs_approval", Arguments: json.RawMessage(`{"secret":"do-not-store"}`)}}},
		{{Type: llm.CompletionEventMessageDelta, Delta: "done"}, {Type: llm.CompletionEventCompleted}},
	})
	tool := &approvalMockTool{name: "needs_approval", result: tools.ToolResult{
		OK:      true,
		Content: strings.Repeat("sensitive output ", 100),
		Data: map[string]any{
			"bytes": 42,
			"token": "secret-token",
		},
	}}
	loop, err := NewReactLoop(ReactLoopOptions{Provider: provider, Tools: []tools.Tool{tool}})
	if err != nil {
		t.Fatalf("NewReactLoop() error = %v", err)
	}
	runtime, err := NewRuntime(RuntimeOptions{
		Loop:     loop,
		Store:    newMemorySessionStore(),
		CWD:      "E:/repo",
		Model:    "gpt-4.1",
		Approver: staticApprover{decision: ApprovalDecisionOnce},
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

	sess, err := runtime.NewSession(context.Background(), NewSessionRequest{})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	events, err := runtime.Prompt(context.Background(), PromptRequest{SessionID: sess.ID, Prompt: "run tool"})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if _, err := collectEvents(context.Background(), events); err != nil {
		t.Fatalf("CollectEvents() error = %v", err)
	}

	saved, err := runtime.LoadSession(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	serialized, err := json.Marshal(saved.Events)
	if err != nil {
		t.Fatalf("Marshal(events) error = %v", err)
	}
	text := string(serialized)
	if !strings.Contains(text, string(EventToolCallStarted)) || !strings.Contains(text, string(EventToolCallCompleted)) {
		t.Fatalf("tool summaries missing: %s", text)
	}
	if !strings.Contains(text, string(EventApprovalRequested)) || !strings.Contains(text, string(EventApprovalDecided)) || !strings.Contains(text, string(ApprovalDecisionOnce)) {
		t.Fatalf("approval summaries missing: %s", text)
	}
	for _, forbidden := range []string{"do-not-store", "secret-token", "sensitive output"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("session summary leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"bytes":42`) {
		t.Fatalf("safe tool metadata missing: %s", text)
	}
}

type memorySessionStore struct {
	sessions map[string]Session
}

func newMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{sessions: map[string]Session{}}
}

func (s *memorySessionStore) Create(ctx context.Context, session Session) (Session, error) {
	if session.ID == "" {
		session.ID = "s1"
	}
	s.sessions[session.ID] = session
	return session, nil
}

func (s *memorySessionStore) Get(ctx context.Context, id string) (Session, error) {
	return s.sessions[id], nil
}

func (s *memorySessionStore) Update(ctx context.Context, session Session) error {
	s.sessions[session.ID] = session
	return nil
}

func (s *memorySessionStore) List(ctx context.Context) ([]Session, error) {
	out := make([]Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		out = append(out, session)
	}
	return out, nil
}
