package service

import (
	"context"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

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

	resp, err := svc.Prompt(ctx, session.ID, PromptRequest{Content: "hello"})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if resp.Approval == nil {
		t.Fatal("expected approval")
	}
	if rec.upsertRunCalls == 0 || rec.appendMessageCalls < 2 || rec.upsertApprovalCalls == 0 {
		t.Fatalf("store write calls not hit, run=%d msg=%d approval=%d", rec.upsertRunCalls, rec.appendMessageCalls, rec.upsertApprovalCalls)
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
