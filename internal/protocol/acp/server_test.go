package acp

import (
	"context"
	"errors"
	"testing"
	"time"

	sdk "github.com/coder/acp-go-sdk"
	"github.com/icoo-ai/icoo-ai/internal/agent"
)

func TestAgentAdapterInitialize(t *testing.T) {
	adapter := newAgentAdapter(&fakeRuntime{}, CapabilitiesOptions{Name: "icoo-test", Version: "1.2.3"})
	resp, err := adapter.Initialize(context.Background(), sdk.InitializeRequest{})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if resp.ProtocolVersion != sdk.ProtocolVersionNumber {
		t.Fatalf("ProtocolVersion = %d", resp.ProtocolVersion)
	}
	if resp.AgentInfo == nil || resp.AgentInfo.Name != "icoo-test" || resp.AgentInfo.Version != "1.2.3" {
		t.Fatalf("AgentInfo = %#v", resp.AgentInfo)
	}
}

func TestAgentAdapterNewSessionAndCancel(t *testing.T) {
	runtime := &fakeRuntime{
		newSession: agent.Session{ID: "s1", CWD: "E:/repo"},
	}
	adapter := newAgentAdapter(runtime, CapabilitiesOptions{})

	resp, err := adapter.NewSession(context.Background(), sdk.NewSessionRequest{Cwd: "E:/repo"})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if resp.SessionId != "s1" {
		t.Fatalf("SessionId = %q", resp.SessionId)
	}
	if runtime.newSessionReq.CWD != "E:/repo" {
		t.Fatalf("runtime NewSession CWD = %q", runtime.newSessionReq.CWD)
	}

	if err := adapter.Cancel(context.Background(), sdk.CancelNotification{SessionId: "s1"}); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if runtime.cancelSessionID != "s1" {
		t.Fatalf("cancel session id = %q", runtime.cancelSessionID)
	}
}

func TestAgentAdapterPromptStreamsUpdates(t *testing.T) {
	events := make(chan agent.Event, 4)
	events <- agent.Event{Type: agent.EventRunStarted, SessionID: "s1"}
	events <- agent.Event{Type: agent.EventMessageDelta, SessionID: "s1", Content: "hello"}
	events <- agent.Event{Type: agent.EventRunCompleted, SessionID: "s1"}
	close(events)

	runtime := &fakeRuntime{promptEvents: events}
	updater := &fakeUpdater{}
	adapter := newAgentAdapter(runtime, CapabilitiesOptions{})
	adapter.setConnection(updater)

	messageID := "11111111-1111-1111-1111-111111111111"
	resp, err := adapter.Prompt(context.Background(), sdk.PromptRequest{
		SessionId: "s1",
		MessageId: &messageID,
		Prompt:    []sdk.ContentBlock{sdk.TextBlock("hello")},
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if runtime.promptReq.SessionID != "s1" || runtime.promptReq.Prompt != "hello" {
		t.Fatalf("runtime prompt request = %#v", runtime.promptReq)
	}
	if resp.StopReason != sdk.StopReasonEndTurn {
		t.Fatalf("StopReason = %s", resp.StopReason)
	}
	if resp.UserMessageId == nil || *resp.UserMessageId != messageID {
		t.Fatalf("UserMessageId = %#v", resp.UserMessageId)
	}
	if len(updater.notifications) != 3 {
		t.Fatalf("notifications = %d", len(updater.notifications))
	}
	if updater.notifications[1].Update.AgentMessageChunk == nil {
		t.Fatalf("second notification = %#v", updater.notifications[1])
	}
}

func TestAgentAdapterPromptCancelledStopReason(t *testing.T) {
	events := make(chan agent.Event, 1)
	events <- agent.Event{Type: agent.EventRunCancelled, SessionID: "s1", Error: "context canceled"}
	close(events)

	adapter := newAgentAdapter(&fakeRuntime{promptEvents: events}, CapabilitiesOptions{})
	adapter.setConnection(&fakeUpdater{})

	resp, err := adapter.Prompt(context.Background(), sdk.PromptRequest{
		SessionId: "s1",
		Prompt:    []sdk.ContentBlock{sdk.TextBlock("stop")},
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if resp.StopReason != sdk.StopReasonCancelled {
		t.Fatalf("StopReason = %s", resp.StopReason)
	}
}

func TestAgentAdapterPromptPropagatesRuntimeError(t *testing.T) {
	want := errors.New("boom")
	adapter := newAgentAdapter(&fakeRuntime{promptErr: want}, CapabilitiesOptions{})
	adapter.setConnection(&fakeUpdater{})

	_, err := adapter.Prompt(context.Background(), sdk.PromptRequest{SessionId: "s1"})
	if !errors.Is(err, want) {
		t.Fatalf("Prompt() error = %v, want %v", err, want)
	}
}

func TestNewServerValidatesOptions(t *testing.T) {
	if _, err := NewServer(ServerOptions{}); err == nil {
		t.Fatal("expected missing runtime error")
	}
	if _, err := NewServer(ServerOptions{Runtime: &fakeRuntime{}}); err == nil {
		t.Fatal("expected missing input error")
	}
}

type fakeRuntime struct {
	newSession      agent.Session
	newSessionReq   agent.NewSessionRequest
	promptReq       agent.PromptRequest
	promptEvents    <-chan agent.Event
	promptErr       error
	cancelSessionID string
}

func (r *fakeRuntime) NewSession(ctx context.Context, req agent.NewSessionRequest) (agent.Session, error) {
	r.newSessionReq = req
	if r.newSession.ID == "" {
		r.newSession.ID = "s1"
		r.newSession.CreatedAt = time.Now().UTC()
		r.newSession.UpdatedAt = r.newSession.CreatedAt
	}
	return r.newSession, nil
}

func (r *fakeRuntime) Prompt(ctx context.Context, req agent.PromptRequest) (<-chan agent.Event, error) {
	r.promptReq = req
	if r.promptErr != nil {
		return nil, r.promptErr
	}
	if r.promptEvents != nil {
		return r.promptEvents, nil
	}
	events := make(chan agent.Event)
	close(events)
	return events, nil
}

func (r *fakeRuntime) Cancel(ctx context.Context, sessionID string) error {
	r.cancelSessionID = sessionID
	return nil
}

func (r *fakeRuntime) LoadSession(ctx context.Context, sessionID string) (agent.Session, error) {
	return agent.Session{ID: sessionID}, nil
}

type fakeUpdater struct {
	notifications []sdk.SessionNotification
}

func (u *fakeUpdater) SessionUpdate(ctx context.Context, params sdk.SessionNotification) error {
	u.notifications = append(u.notifications, params)
	return nil
}
