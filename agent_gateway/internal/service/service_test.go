package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type fakeConnector struct {
	newSessionResp connector.NewSessionResponse
	newSessionErr  error
	lastNewSession connector.NewSessionRequest

	promptResp connector.PromptResponse
	promptErr  error

	cancelResp connector.CancelResponse
	cancelErr  error

	newSessionCalls int
	promptCalls     int
	cancelCalls     int
}

func (f *fakeConnector) Initialize(context.Context, connector.InitializeRequest) (connector.InitializeResponse, error) {
	return connector.InitializeResponse{}, nil
}
func (f *fakeConnector) NewSession(_ context.Context, req connector.NewSessionRequest) (connector.NewSessionResponse, error) {
	f.newSessionCalls++
	f.lastNewSession = req
	if f.newSessionErr != nil {
		return connector.NewSessionResponse{}, f.newSessionErr
	}
	return f.newSessionResp, nil
}
func (f *fakeConnector) Prompt(context.Context, connector.PromptRequest) (connector.PromptResponse, error) {
	f.promptCalls++
	if f.promptErr != nil {
		return connector.PromptResponse{}, f.promptErr
	}
	return f.promptResp, nil
}
func (f *fakeConnector) Cancel(context.Context, connector.CancelRequest) (connector.CancelResponse, error) {
	f.cancelCalls++
	if f.cancelErr != nil {
		return connector.CancelResponse{}, f.cancelErr
	}
	return f.cancelResp, nil
}
func (f *fakeConnector) Close() error { return nil }

type recordingStore struct {
	base *store.MemoryStore

	upsertConversationCalls int
	getConversationCalls    int
	appendMessageCalls      int
	listMessagesCalls       int
	upsertRunCalls          int
	listRunsCalls           int
	upsertApprovalCalls     int
	listApprovalsCalls      int
}

func newRecordingStore() *recordingStore {
	return &recordingStore{base: store.NewMemoryStore()}
}

func (r *recordingStore) UpsertConversation(ctx context.Context, conversation store.Conversation) error {
	r.upsertConversationCalls++
	return r.base.UpsertConversation(ctx, conversation)
}
func (r *recordingStore) ListConversations(ctx context.Context) ([]store.Conversation, error) {
	return r.base.ListConversations(ctx)
}
func (r *recordingStore) GetConversation(ctx context.Context, sessionID string) (store.Conversation, bool, error) {
	r.getConversationCalls++
	return r.base.GetConversation(ctx, sessionID)
}
func (r *recordingStore) AppendMessage(ctx context.Context, event store.MessageEvent) error {
	r.appendMessageCalls++
	return r.base.AppendMessage(ctx, event)
}
func (r *recordingStore) ListMessages(ctx context.Context, sessionID string) ([]store.MessageEvent, error) {
	r.listMessagesCalls++
	return r.base.ListMessages(ctx, sessionID)
}
func (r *recordingStore) UpsertRun(ctx context.Context, run store.RunSummary) error {
	r.upsertRunCalls++
	return r.base.UpsertRun(ctx, run)
}
func (r *recordingStore) ListRuns(ctx context.Context, sessionID string) ([]store.RunSummary, error) {
	r.listRunsCalls++
	return r.base.ListRuns(ctx, sessionID)
}
func (r *recordingStore) UpsertApproval(ctx context.Context, approval store.ApprovalDecision) error {
	r.upsertApprovalCalls++
	return r.base.UpsertApproval(ctx, approval)
}
func (r *recordingStore) ListApprovals(ctx context.Context) ([]store.ApprovalDecision, error) {
	r.listApprovalsCalls++
	return r.base.ListApprovals(ctx)
}
func (r *recordingStore) AppendAudit(ctx context.Context, event store.AuditEvent) error {
	return r.base.AppendAudit(ctx, event)
}
func (r *recordingStore) ListAuditEvents(ctx context.Context) ([]store.AuditEvent, error) {
	return r.base.ListAuditEvents(ctx)
}

func TestMockGatewayServiceWritesThroughStore(t *testing.T) {
	ctx := context.Background()
	rec := newRecordingStore()
	svc := NewMockGatewayServiceWithStore(rec)

	session, err := svc.CreateSession(ctx, CreateSessionRequest{Title: "store-writes"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if rec.upsertConversationCalls != 1 {
		t.Fatalf("upsertConversation calls = %d, want 1", rec.upsertConversationCalls)
	}

	_, err = svc.Prompt(ctx, session.ID, PromptRequest{Content: "hello"})
	if err == nil {
		t.Fatal("expected prompt to fail without connector")
	}
	serviceErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *service.Error, got %T", err)
	}
	if serviceErr.Code != "connector_unavailable" {
		t.Fatalf("unexpected error code: %q", serviceErr.Code)
	}
	if rec.upsertRunCalls != 0 || rec.appendMessageCalls != 0 || rec.upsertApprovalCalls != 0 {
		t.Fatalf("unexpected store writes, run=%d msg=%d approval=%d", rec.upsertRunCalls, rec.appendMessageCalls, rec.upsertApprovalCalls)
	}
}

func TestMockGatewayServiceReadsThroughStore(t *testing.T) {
	ctx := context.Background()
	rec := newRecordingStore()
	svc := NewMockGatewayServiceWithStore(rec)
	now := time.Date(2026, 5, 9, 14, 0, 0, 0, time.UTC)

	if err := rec.UpsertConversation(ctx, store.Conversation{
		ID:        "sess_001",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_001",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	if err := rec.AppendMessage(ctx, store.MessageEvent{
		ID:        "msg_001",
		Type:      "message",
		SessionID: "sess_001",
		RunID:     "run_001",
		Role:      "user",
		Summary:   "hello",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("seed message: %v", err)
	}
	if err := rec.UpsertRun(ctx, store.RunSummary{
		ID:        "run_001",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_001",
		RunID:     "run_001",
		Status:    "completed",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	if err := rec.UpsertApproval(ctx, store.ApprovalDecision{
		ID:                 "approval_001",
		AgentID:            "icoo-ai-acp",
		SessionID:          "sess_001",
		RunID:              "run_001",
		ConnectorRequestID: "tool_call_001",
		Status:             "pending",
		CreatedAt:          now,
		SafeMeta: store.SafeMeta{
			"action": "mock_tool",
		},
	}); err != nil {
		t.Fatalf("seed approval: %v", err)
	}

	messages, err := svc.ListMessages(ctx, "sess_001")
	if err != nil || len(messages) != 1 {
		t.Fatalf("list messages = %+v err=%v", messages, err)
	}
	runs, err := svc.ListRuns(ctx)
	if err != nil || len(runs) != 1 {
		t.Fatalf("list runs = %+v err=%v", runs, err)
	}
	approvals, err := svc.ListApprovals(ctx)
	if err != nil || len(approvals) != 1 {
		t.Fatalf("list approvals = %+v err=%v", approvals, err)
	}

	if rec.getConversationCalls == 0 || rec.listMessagesCalls == 0 || rec.listRunsCalls == 0 || rec.listApprovalsCalls == 0 {
		t.Fatalf("store read calls not hit, getConv=%d listMsg=%d listRun=%d listApproval=%d", rec.getConversationCalls, rec.listMessagesCalls, rec.listRunsCalls, rec.listApprovalsCalls)
	}
}

func TestMockGatewayServiceGetSessionAndListSkills(t *testing.T) {
	ctx := context.Background()
	svc := NewMockGatewayService()

	session, err := svc.CreateSession(ctx, CreateSessionRequest{Title: "lookup"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	got, err := svc.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.ID != session.ID {
		t.Fatalf("expected session id %q, got %q", session.ID, got.ID)
	}

	skills, err := svc.ListSkills(ctx)
	if err != nil {
		t.Fatalf("list skills: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected no skills, got %#v", skills)
	}
}

func TestConnectorBackedServiceUsesConnectorSessionAndPrompt(t *testing.T) {
	ctx := context.Background()
	rec := newRecordingStore()
	endedAt := time.Date(2026, 5, 9, 15, 0, 0, 0, time.UTC)
	fake := &fakeConnector{
		newSessionResp: connector.NewSessionResponse{SessionID: "sess_conn_1"},
		promptResp: connector.PromptResponse{
			RunID:   "run_conn_1",
			Output:  "connector output",
			EndedAt: &endedAt,
			Approvals: []connector.ApprovalRequest{
				{RequestID: "approval_req_1", Action: "write_file", Message: "allow write"},
			},
		},
		cancelResp: connector.CancelResponse{RunID: "run_conn_1", Status: "cancelled"},
	}

	svc := NewConnectorGatewayServiceWithAgentsAndStore(defaultAgents(), rec, fake)
	session, err := svc.CreateSession(ctx, CreateSessionRequest{
		Title:          "connector session",
		CWD:            "E:/code/issueye/icoo_ai",
		StartupCommand: "icoo-ai --profile dev",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.ID != "sess_conn_1" {
		t.Fatalf("session id = %q, want sess_conn_1", session.ID)
	}

	resp, err := svc.Prompt(ctx, session.ID, PromptRequest{Content: "hello connector"})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if resp.Run.ID != "run_conn_1" || resp.Run.Status != "completed" {
		t.Fatalf("unexpected run: %#v", resp.Run)
	}
	if len(resp.Messages) != 2 {
		t.Fatalf("expected 2 messages (user+assistant), got %d", len(resp.Messages))
	}
	if resp.Approval == nil || resp.Approval.ConnectorRequestID != "approval_req_1" {
		t.Fatalf("unexpected approval: %#v", resp.Approval)
	}
	if fake.newSessionCalls != 1 || fake.promptCalls != 1 {
		t.Fatalf("connector calls newSession=%d prompt=%d", fake.newSessionCalls, fake.promptCalls)
	}
	if fake.lastNewSession.CWD != "E:/code/issueye/icoo_ai" {
		t.Fatalf("connector newSession cwd = %q", fake.lastNewSession.CWD)
	}
	if got, _ := fake.lastNewSession.Metadata["startupCommand"].(string); got != "icoo-ai --profile dev" {
		t.Fatalf("connector newSession metadata.startupCommand = %q", got)
	}

	cancelled, err := svc.Cancel(ctx, session.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.ID != "run_conn_1" || cancelled.Status != "cancelled" {
		t.Fatalf("unexpected cancel result: %#v", cancelled)
	}
	if fake.cancelCalls != 1 {
		t.Fatalf("expected cancel call, got %d", fake.cancelCalls)
	}
}

func TestConnectorBackedServicePromptHistoryQueryConsistency(t *testing.T) {
	ctx := context.Background()
	rec := newRecordingStore()
	endedAt := time.Date(2026, 5, 9, 15, 30, 0, 0, time.UTC)
	fake := &fakeConnector{
		newSessionResp: connector.NewSessionResponse{SessionID: "sess_conn_history_1"},
		promptResp: connector.PromptResponse{
			RunID:   "run_conn_history_1",
			Output:  "history output",
			EndedAt: &endedAt,
			Approvals: []connector.ApprovalRequest{
				{RequestID: "approval_req_history_1", Action: "execute", Message: "approve execution"},
			},
		},
	}
	svc := NewConnectorGatewayServiceWithAgentsAndStore(defaultAgents(), rec, fake)

	session, err := svc.CreateSession(ctx, CreateSessionRequest{Title: "history consistency"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	resp, err := svc.Prompt(ctx, session.ID, PromptRequest{Content: "connector history"})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if resp.Approval == nil {
		t.Fatal("expected approval")
	}

	messages, err := svc.ListMessages(ctx, session.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != len(resp.Messages) {
		t.Fatalf("messages count mismatch, list=%d prompt=%d", len(messages), len(resp.Messages))
	}
	for idx := range resp.Messages {
		if messages[idx].ID != resp.Messages[idx].ID || messages[idx].RunID != resp.Run.ID || messages[idx].SessionID != session.ID {
			t.Fatalf("message[%d] mismatch, list=%#v prompt=%#v", idx, messages[idx], resp.Messages[idx])
		}
	}

	runs, err := svc.ListRuns(ctx)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].ID != resp.Run.ID || runs[0].SessionID != session.ID || runs[0].AgentID != session.AgentID {
		t.Fatalf("run mismatch, listed=%#v prompt=%#v session=%#v", runs[0], resp.Run, session)
	}

	approvals, err := svc.ListApprovals(ctx)
	if err != nil {
		t.Fatalf("list approvals: %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("expected 1 approval, got %d", len(approvals))
	}
	if approvals[0].ID != resp.Approval.ID || approvals[0].RunID != resp.Run.ID || approvals[0].SessionID != session.ID {
		t.Fatalf("approval mismatch, listed=%#v prompt=%#v", approvals[0], resp.Approval)
	}

	if rec.listMessagesCalls == 0 || rec.listRunsCalls == 0 || rec.listApprovalsCalls == 0 {
		t.Fatalf("expected list calls to hit store, listMsg=%d listRun=%d listApproval=%d", rec.listMessagesCalls, rec.listRunsCalls, rec.listApprovalsCalls)
	}
}

func TestConnectorBackedServiceMapsConnectorPromptError(t *testing.T) {
	ctx := context.Background()
	rec := newRecordingStore()
	fake := &fakeConnector{
		newSessionResp: connector.NewSessionResponse{SessionID: "sess_conn_2"},
		promptErr:      fmt.Errorf("connector down"),
	}
	svc := NewConnectorGatewayServiceWithAgentsAndStore(defaultAgents(), rec, fake)

	session, err := svc.CreateSession(ctx, CreateSessionRequest{Title: "connector error"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	_, err = svc.Prompt(ctx, session.ID, PromptRequest{Content: "hello"})
	if err == nil {
		t.Fatal("expected prompt error")
	}
	serviceErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *service.Error, got %T", err)
	}
	if serviceErr.Code != "connector_request_failed" {
		t.Fatalf("unexpected error code: %s", serviceErr.Code)
	}
}
