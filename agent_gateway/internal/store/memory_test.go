package store

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreUpsertAndList(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 9, 14, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	later := now.Add(time.Minute)
	store := NewMemoryStore()

	if err := store.UpsertConversation(ctx, Conversation{
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_001",
		Title:     "new session",
		Status:    "active",
		CreatedAt: now,
		SafeMeta:  SafeMeta{"workspace": "icoo_ai"},
	}); err != nil {
		t.Fatalf("upsert conversation: %v", err)
	}
	if err := store.UpsertConversation(ctx, Conversation{
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_001",
		RunID:     "run_001",
		Title:     "new session",
		Status:    "running",
		CreatedAt: now,
		UpdatedAt: later,
	}); err != nil {
		t.Fatalf("update conversation: %v", err)
	}

	conversations, err := store.ListConversations(ctx)
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if len(conversations) != 1 {
		t.Fatalf("conversations len = %d, want 1", len(conversations))
	}
	if conversations[0].Status != "running" || conversations[0].RunID != "run_001" {
		t.Fatalf("conversation update not reflected: %+v", conversations[0])
	}

	conversations[0].SafeMeta = SafeMeta{"mutated": true}
	got, ok, err := store.GetConversation(ctx, "sess_001")
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if !ok {
		t.Fatal("conversation not found")
	}
	if got.SafeMeta["mutated"] == true {
		t.Fatal("conversation list returned mutable internal state")
	}

	completedAt := later.Add(time.Minute)
	if err := store.UpsertRun(ctx, RunSummary{
		AgentID:     "icoo-ai-acp",
		SessionID:   "sess_001",
		RunID:       "run_001",
		Status:      "running",
		Summary:     "prompt accepted",
		CreatedAt:   now,
		CompletedAt: &completedAt,
	}); err != nil {
		t.Fatalf("upsert run: %v", err)
	}
	runs, err := store.ListRuns(ctx, "sess_001")
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != "run_001" {
		t.Fatalf("runs = %+v, want run_001", runs)
	}

	if err := store.AppendAudit(ctx, AuditEvent{
		ID:        "audit_001",
		Type:      "session.created",
		Level:     "info",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_001",
		Summary:   "session created",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("append audit: %v", err)
	}
	auditEvents, err := store.ListAuditEvents(ctx)
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(auditEvents) != 1 || auditEvents[0].ID != "audit_001" {
		t.Fatalf("auditEvents = %+v, want audit_001", auditEvents)
	}
}

func TestMemoryStoreListMessagesFiltersBySession(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 9, 14, 0, 0, 0, time.UTC)
	store := NewMemoryStore()

	events := []MessageEvent{
		{ID: "evt_001", Type: "message", AgentID: "agent_a", SessionID: "sess_001", RunID: "run_001", Role: "user", Summary: "hello", CreatedAt: now},
		{ID: "evt_002", Type: "message", AgentID: "agent_a", SessionID: "sess_002", RunID: "run_002", Role: "user", Summary: "other", CreatedAt: now},
		{ID: "evt_003", Type: "message_delta", AgentID: "agent_a", SessionID: "sess_001", RunID: "run_001", Role: "assistant", Summary: "hi", CreatedAt: now},
	}
	for _, event := range events {
		if err := store.AppendMessage(ctx, event); err != nil {
			t.Fatalf("append message %s: %v", event.ID, err)
		}
	}

	messages, err := store.ListMessages(ctx, "sess_001")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(messages))
	}
	if messages[0].ID != "evt_001" || messages[1].ID != "evt_003" {
		t.Fatalf("messages order/filter = %+v", messages)
	}
}

func TestMemoryStoreUpsertApprovalUpdatesExisting(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 9, 14, 0, 0, 0, time.UTC)
	later := now.Add(30 * time.Second)
	store := NewMemoryStore()

	pending := ApprovalDecision{
		ID:                 "approval_001",
		AgentID:            "icoo-ai-acp",
		SessionID:          "sess_001",
		RunID:              "run_001",
		ConnectorRequestID: "tool_call_001",
		Status:             "pending",
		Summary:            "needs approval",
		CreatedAt:          now,
	}
	if err := store.UpsertApproval(ctx, pending); err != nil {
		t.Fatalf("upsert pending approval: %v", err)
	}

	pending.Status = "approved"
	pending.Decision = "once"
	pending.Actor = "user"
	pending.UpdatedAt = later
	pending.DecidedAt = &later
	pending.Summary = "approved by user"
	if err := store.UpsertApproval(ctx, pending); err != nil {
		t.Fatalf("update approval: %v", err)
	}

	approvals, err := store.ListApprovals(ctx)
	if err != nil {
		t.Fatalf("list approvals: %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("approvals len = %d, want 1", len(approvals))
	}
	got := approvals[0]
	if got.Status != "approved" || got.Decision != "once" || got.Actor != "user" {
		t.Fatalf("approval update not reflected: %+v", got)
	}
	if got.ConnectorRequestID != "tool_call_001" || got.AgentID != "icoo-ai-acp" || got.SessionID != "sess_001" || got.RunID != "run_001" {
		t.Fatalf("approval identity changed: %+v", got)
	}
}
