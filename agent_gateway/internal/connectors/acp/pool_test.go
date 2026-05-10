package acp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
)

type fakePoolBackend struct {
	id string

	mu sync.Mutex

	initializeCalls int
	newSessionCalls int
	promptCalls     []connector.PromptRequest
	cancelCalls     []connector.CancelRequest
	closeCalls      int

	initializeResp connector.InitializeResponse
	initializeErr  error

	newSessionIDs []string
	newSessionErr error

	promptResp connector.PromptResponse
	promptErr  error

	cancelResp connector.CancelResponse
	cancelErr  error

	closeErr error
}

func (f *fakePoolBackend) Initialize(context.Context, connector.InitializeRequest) (connector.InitializeResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.initializeCalls++
	if f.initializeErr != nil {
		return connector.InitializeResponse{}, f.initializeErr
	}
	return f.initializeResp, nil
}

func (f *fakePoolBackend) NewSession(context.Context, connector.NewSessionRequest) (connector.NewSessionResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.newSessionCalls++
	if f.newSessionErr != nil {
		return connector.NewSessionResponse{}, f.newSessionErr
	}
	var sessionID string
	if len(f.newSessionIDs) > 0 {
		sessionID = f.newSessionIDs[0]
		f.newSessionIDs = f.newSessionIDs[1:]
	}
	if sessionID == "" {
		sessionID = fmt.Sprintf("%s-session-%d", f.id, f.newSessionCalls)
	}
	return connector.NewSessionResponse{SessionID: sessionID}, nil
}

func (f *fakePoolBackend) Prompt(_ context.Context, req connector.PromptRequest) (connector.PromptResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.promptCalls = append(f.promptCalls, req)
	if f.promptErr != nil {
		return connector.PromptResponse{}, f.promptErr
	}
	if f.promptResp.RunID == "" {
		return connector.PromptResponse{RunID: "run-" + f.id}, nil
	}
	return f.promptResp, nil
}

func (f *fakePoolBackend) Cancel(_ context.Context, req connector.CancelRequest) (connector.CancelResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelCalls = append(f.cancelCalls, req)
	if f.cancelErr != nil {
		return connector.CancelResponse{}, f.cancelErr
	}
	if f.cancelResp.Status == "" {
		return connector.CancelResponse{RunID: req.RunID, Status: "cancelled"}, nil
	}
	return f.cancelResp, nil
}

func (f *fakePoolBackend) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeCalls++
	return f.closeErr
}

func (f *fakePoolBackend) snapshot() fakePoolBackend {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *f
	cp.promptCalls = append([]connector.PromptRequest(nil), f.promptCalls...)
	cp.cancelCalls = append([]connector.CancelRequest(nil), f.cancelCalls...)
	return cp
}

func TestPoolInitializeBroadcastIdempotent(t *testing.T) {
	first := &fakePoolBackend{
		id:             "a",
		initializeResp: connector.InitializeResponse{ServerName: "acp-a", ServerVersion: "1.0.0"},
	}
	second := &fakePoolBackend{
		id:             "b",
		initializeResp: connector.InitializeResponse{ServerName: "acp-b", ServerVersion: "1.0.0"},
	}
	pool, err := NewPool([]connector.AgentConnector{first, second})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	gotFirst, err := pool.Initialize(context.Background(), connector.InitializeRequest{ClientName: "gateway", ClientVersion: "1.0.0"})
	if err != nil {
		t.Fatalf("Initialize() first call error = %v", err)
	}
	gotSecond, err := pool.Initialize(context.Background(), connector.InitializeRequest{ClientName: "gateway", ClientVersion: "1.0.0"})
	if err != nil {
		t.Fatalf("Initialize() second call error = %v", err)
	}
	if gotFirst != gotSecond {
		t.Fatalf("Initialize() idempotent response mismatch: first=%#v second=%#v", gotFirst, gotSecond)
	}
	if gotFirst.ServerName != "acp-a" {
		t.Fatalf("Initialize() expected first backend response, got %#v", gotFirst)
	}

	firstState := first.snapshot()
	secondState := second.snapshot()
	if firstState.initializeCalls != 1 || secondState.initializeCalls != 1 {
		t.Fatalf("expected initialize broadcast once, got first=%d second=%d", firstState.initializeCalls, secondState.initializeCalls)
	}
}

func TestPoolNewSessionRoundRobinAndSessionRouting(t *testing.T) {
	first := &fakePoolBackend{id: "a", newSessionIDs: []string{"sess-a-1", "sess-a-2"}}
	second := &fakePoolBackend{id: "b", newSessionIDs: []string{"sess-b-1"}}
	pool, err := NewPool([]connector.AgentConnector{first, second})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	ctx := context.Background()
	s1, err := pool.NewSession(ctx, connector.NewSessionRequest{AgentID: "icoo-ai-acp", Model: "gpt-5.4"})
	if err != nil {
		t.Fatalf("NewSession() #1 error = %v", err)
	}
	s2, err := pool.NewSession(ctx, connector.NewSessionRequest{AgentID: "icoo-ai-acp", Model: "gpt-5.4"})
	if err != nil {
		t.Fatalf("NewSession() #2 error = %v", err)
	}
	s3, err := pool.NewSession(ctx, connector.NewSessionRequest{AgentID: "icoo-ai-acp", Model: "gpt-5.4"})
	if err != nil {
		t.Fatalf("NewSession() #3 error = %v", err)
	}

	if s1.SessionID != "sess-a-1" || s2.SessionID != "sess-b-1" || s3.SessionID != "sess-a-2" {
		t.Fatalf("unexpected round robin session order: %q %q %q", s1.SessionID, s2.SessionID, s3.SessionID)
	}

	p1, err := pool.Prompt(ctx, connector.PromptRequest{SessionID: s1.SessionID, Content: "hello"})
	if err != nil {
		t.Fatalf("Prompt() #1 error = %v", err)
	}
	p2, err := pool.Prompt(ctx, connector.PromptRequest{SessionID: s2.SessionID, Content: "world"})
	if err != nil {
		t.Fatalf("Prompt() #2 error = %v", err)
	}
	if p1.RunID != "run-a" || p2.RunID != "run-b" {
		t.Fatalf("unexpected routed prompt responses: %#v %#v", p1, p2)
	}

	_, err = pool.Cancel(ctx, connector.CancelRequest{SessionID: s2.SessionID, RunID: "run-b-1", Reason: "user_cancel"})
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	firstState := first.snapshot()
	secondState := second.snapshot()
	if firstState.newSessionCalls != 2 || secondState.newSessionCalls != 1 {
		t.Fatalf("unexpected session distribution: first=%d second=%d", firstState.newSessionCalls, secondState.newSessionCalls)
	}
	if len(firstState.promptCalls) != 1 || firstState.promptCalls[0].SessionID != "sess-a-1" {
		t.Fatalf("unexpected first backend prompt calls: %#v", firstState.promptCalls)
	}
	if len(secondState.promptCalls) != 1 || secondState.promptCalls[0].SessionID != "sess-b-1" {
		t.Fatalf("unexpected second backend prompt calls: %#v", secondState.promptCalls)
	}
	if len(firstState.cancelCalls) != 0 {
		t.Fatalf("unexpected first backend cancel calls: %#v", firstState.cancelCalls)
	}
	if len(secondState.cancelCalls) != 1 || secondState.cancelCalls[0].SessionID != "sess-b-1" {
		t.Fatalf("unexpected second backend cancel calls: %#v", secondState.cancelCalls)
	}
}

func TestPoolCloseClosesAllBackendsAndAggregatesErrors(t *testing.T) {
	errFirst := errors.New("close first failed")
	errThird := errors.New("close third failed")
	first := &fakePoolBackend{id: "a", closeErr: errFirst}
	second := &fakePoolBackend{id: "b"}
	third := &fakePoolBackend{id: "c", closeErr: errThird}

	pool, err := NewPool([]connector.AgentConnector{first, second, third})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	closeErr := pool.Close()
	if closeErr == nil {
		t.Fatal("Close() expected aggregated error, got nil")
	}
	if !errors.Is(closeErr, errFirst) || !errors.Is(closeErr, errThird) {
		t.Fatalf("Close() expected aggregated causes, got %v", closeErr)
	}

	if first.snapshot().closeCalls != 1 || second.snapshot().closeCalls != 1 || third.snapshot().closeCalls != 1 {
		t.Fatalf("expected all backends closed once, got first=%d second=%d third=%d",
			first.snapshot().closeCalls, second.snapshot().closeCalls, third.snapshot().closeCalls)
	}

	_ = pool.Close()
	if first.snapshot().closeCalls != 1 || second.snapshot().closeCalls != 1 || third.snapshot().closeCalls != 1 {
		t.Fatalf("expected Close() idempotent, got first=%d second=%d third=%d",
			first.snapshot().closeCalls, second.snapshot().closeCalls, third.snapshot().closeCalls)
	}
}

func TestPoolErrorPaths(t *testing.T) {
	_, err := NewPool(nil)
	if err == nil {
		t.Fatal("NewPool(nil) expected error")
	}
	structured, ok := err.(*connector.Error)
	if !ok {
		t.Fatalf("expected *connector.Error, got %T", err)
	}
	if structured.Code != connector.ErrCodeInvalidConnectorConfig {
		t.Fatalf("unexpected error code: %q", structured.Code)
	}

	pool, err := NewPool([]connector.AgentConnector{&fakePoolBackend{id: "a"}})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	_, err = pool.Prompt(context.Background(), connector.PromptRequest{SessionID: "unknown-session", Content: "hello"})
	if err == nil {
		t.Fatal("Prompt() with unknown session expected error")
	}
	promptErr, ok := err.(*connector.Error)
	if !ok {
		t.Fatalf("expected *connector.Error, got %T", err)
	}
	if promptErr.Code != connector.ErrCodeProtocolError {
		t.Fatalf("unexpected prompt error code: %q", promptErr.Code)
	}

	_, err = pool.Cancel(context.Background(), connector.CancelRequest{SessionID: "unknown-session", RunID: "run-1"})
	if err == nil {
		t.Fatal("Cancel() with unknown session expected error")
	}
	cancelErr, ok := err.(*connector.Error)
	if !ok {
		t.Fatalf("expected *connector.Error, got %T", err)
	}
	if cancelErr.Code != connector.ErrCodeProtocolError {
		t.Fatalf("unexpected cancel error code: %q", cancelErr.Code)
	}
}
