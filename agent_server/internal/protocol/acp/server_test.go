package acp

import (
	"context"
	"errors"
	"strings"
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
	if resp.AgentCapabilities.SessionCapabilities.Close == nil {
		t.Fatal("session close capability should be advertised")
	}
	if resp.AgentCapabilities.SessionCapabilities.List == nil {
		t.Fatal("session list capability should be advertised")
	}
	if resp.AgentCapabilities.SessionCapabilities.Resume == nil {
		t.Fatal("session resume capability should be advertised")
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

func TestAgentAdapterListSessionsSupportsFilters(t *testing.T) {
	now := time.Date(2026, 5, 10, 1, 2, 3, 0, time.UTC)
	runtime := &fakeRuntime{
		sessions: []agent.Session{
			{ID: "s1", CWD: "E:/repo/a", UpdatedAt: now},
			{ID: "s2", CWD: "E:/repo/b", UpdatedAt: now.Add(-time.Minute)},
		},
	}
	adapter := newAgentAdapter(runtime, CapabilitiesOptions{})
	adapter.setSessionState("s1", adapterSessionState{
		ModeID:                defaultSessionModeID,
		ApprovalMode:          defaultApprovalMode,
		EmitPlanUpdates:       true,
		AdditionalDirectories: []string{"E:/repo/shared"},
		CWD:                   "E:/repo/a",
	})

	resp, err := adapter.ListSessions(context.Background(), sdk.ListSessionsRequest{})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(resp.Sessions) != 2 {
		t.Fatalf("sessions len = %d, want 2", len(resp.Sessions))
	}
	if resp.Sessions[0].SessionId != "s1" {
		t.Fatalf("first session = %q, want s1", resp.Sessions[0].SessionId)
	}

	cwd := "E:/repo/b"
	filteredByCWD, err := adapter.ListSessions(context.Background(), sdk.ListSessionsRequest{
		Cwd: &cwd,
	})
	if err != nil {
		t.Fatalf("ListSessions(cwd) error = %v", err)
	}
	if len(filteredByCWD.Sessions) != 1 || filteredByCWD.Sessions[0].SessionId != "s2" {
		t.Fatalf("cwd filtered sessions = %#v", filteredByCWD.Sessions)
	}

	filteredByDirs, err := adapter.ListSessions(context.Background(), sdk.ListSessionsRequest{
		AdditionalDirectories: []string{"E:/repo/shared"},
	})
	if err != nil {
		t.Fatalf("ListSessions(additionalDirectories) error = %v", err)
	}
	if len(filteredByDirs.Sessions) != 1 || filteredByDirs.Sessions[0].SessionId != "s1" {
		t.Fatalf("directory filtered sessions = %#v", filteredByDirs.Sessions)
	}
}

func TestAgentAdapterCloseSessionCancelsAndClearsState(t *testing.T) {
	runtime := &fakeRuntime{}
	adapter := newAgentAdapter(runtime, CapabilitiesOptions{})
	adapter.setSessionState("s1", adapterSessionState{
		ModeID:          defaultSessionModeID,
		ApprovalMode:    defaultApprovalMode,
		EmitPlanUpdates: true,
	})

	if _, err := adapter.CloseSession(context.Background(), sdk.CloseSessionRequest{SessionId: "s1"}); err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
	if runtime.cancelSessionID != "s1" {
		t.Fatalf("Cancel() session id = %q, want s1", runtime.cancelSessionID)
	}
	if runtime.closeSessionID != "s1" {
		t.Fatalf("CloseSession() session id = %q, want s1", runtime.closeSessionID)
	}
	if _, ok := adapter.getSessionState("s1"); ok {
		t.Fatal("session state should be removed after close")
	}
}

func TestAgentAdapterResumeSessionUpdatesStateAndSessionCWD(t *testing.T) {
	runtime := &fakeRuntime{
		loadedSession: agent.Session{
			ID:  "s1",
			CWD: "E:/repo/old",
		},
	}
	adapter := newAgentAdapter(runtime, CapabilitiesOptions{})

	resp, err := adapter.ResumeSession(context.Background(), sdk.ResumeSessionRequest{
		SessionId:             "s1",
		Cwd:                   "E:/repo/new",
		AdditionalDirectories: []string{"E:/repo/shared"},
	})
	if err != nil {
		t.Fatalf("ResumeSession() error = %v", err)
	}
	if resp.Modes == nil || resp.Modes.CurrentModeId != defaultSessionModeID {
		t.Fatalf("resume modes = %#v", resp.Modes)
	}
	if len(resp.ConfigOptions) == 0 {
		t.Fatal("resume config options should not be empty")
	}
	if runtime.updatedSession.CWD != "E:/repo/new" {
		t.Fatalf("updated session cwd = %q, want E:/repo/new", runtime.updatedSession.CWD)
	}
	state, ok := adapter.getSessionState("s1")
	if !ok {
		t.Fatal("session state should exist after resume")
	}
	if state.CWD != "E:/repo/new" {
		t.Fatalf("state cwd = %q, want E:/repo/new", state.CWD)
	}
	if len(state.AdditionalDirectories) != 1 || state.AdditionalDirectories[0] != "E:/repo/shared" {
		t.Fatalf("state additional directories = %#v", state.AdditionalDirectories)
	}
}

func TestAgentAdapterSetSessionModeAndConfig(t *testing.T) {
	runtime := &fakeRuntime{}
	adapter := newAgentAdapter(runtime, CapabilitiesOptions{})

	if _, err := adapter.SetSessionMode(context.Background(), sdk.SetSessionModeRequest{
		SessionId: "s1",
		ModeId:    defaultSessionModeID,
	}); err != nil {
		t.Fatalf("SetSessionMode() error = %v", err)
	}
	state, ok := adapter.getSessionState("s1")
	if !ok || state.ModeID != defaultSessionModeID {
		t.Fatalf("mode state = %#v", state)
	}

	_, err := adapter.SetSessionMode(context.Background(), sdk.SetSessionModeRequest{
		SessionId: "s1",
		ModeId:    "unsupported-mode",
	})
	if err == nil {
		t.Fatal("SetSessionMode() should fail for unsupported mode")
	}
	if !strings.Contains(err.Error(), "unsupported modeId") {
		t.Fatalf("unexpected SetSessionMode() error: %v", err)
	}

	configResp, err := adapter.SetSessionConfigOption(context.Background(), sdk.SetSessionConfigOptionRequest{
		ValueId: &sdk.SetSessionConfigOptionValueId{
			SessionId: "s1",
			ConfigId:  configApprovalModeID,
			Value:     sdk.SessionConfigValueId("readonly"),
		},
	})
	if err != nil {
		t.Fatalf("SetSessionConfigOption(value) error = %v", err)
	}
	if len(configResp.ConfigOptions) != 2 {
		t.Fatalf("config option count = %d, want 2", len(configResp.ConfigOptions))
	}
	state, ok = adapter.getSessionState("s1")
	if !ok || state.ApprovalMode != sdk.SessionConfigValueId("readonly") {
		t.Fatalf("approval state = %#v", state)
	}

	_, err = adapter.SetSessionConfigOption(context.Background(), sdk.SetSessionConfigOptionRequest{
		Boolean: &sdk.SetSessionConfigOptionBoolean{
			SessionId: "s1",
			ConfigId:  configEmitPlanUpdatesID,
			Type:      "boolean",
			Value:     false,
		},
	})
	if err != nil {
		t.Fatalf("SetSessionConfigOption(boolean) error = %v", err)
	}
	state, ok = adapter.getSessionState("s1")
	if !ok || state.EmitPlanUpdates {
		t.Fatalf("plan update state = %#v", state)
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

	sessions       []agent.Session
	loadedSession  agent.Session
	loadErr        error
	updatedSession agent.Session
	closeSessionID string
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
	if r.loadErr != nil {
		return agent.Session{}, r.loadErr
	}
	if r.loadedSession.ID != "" {
		return r.loadedSession, nil
	}
	return agent.Session{ID: sessionID, CWD: "E:/repo/default"}, nil
}

func (r *fakeRuntime) ListSessions(ctx context.Context) ([]agent.Session, error) {
	if len(r.sessions) == 0 {
		return nil, nil
	}
	out := make([]agent.Session, 0, len(r.sessions))
	out = append(out, r.sessions...)
	return out, nil
}

func (r *fakeRuntime) UpdateSession(ctx context.Context, session agent.Session) error {
	r.updatedSession = session
	return nil
}

func (r *fakeRuntime) CloseSession(ctx context.Context, sessionID string) error {
	r.closeSessionID = sessionID
	return nil
}

type fakeUpdater struct {
	notifications []sdk.SessionNotification
}

func (u *fakeUpdater) SessionUpdate(ctx context.Context, params sdk.SessionNotification) error {
	u.notifications = append(u.notifications, params)
	return nil
}
