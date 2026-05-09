package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJSONLStorePersistsAndRestores(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	now := time.Date(2026, 5, 9, 14, 0, 0, 0, time.UTC)

	s, err := NewJSONLStore(ctx, JSONLConfig{Dir: dir})
	if err != nil {
		t.Fatalf("new jsonl store: %v", err)
	}

	if err := s.UpsertConversation(ctx, Conversation{
		ID:        "conv_001",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_001",
		Status:    "active",
		Summary:   "session created",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("upsert conversation: %v", err)
	}
	if err := s.AppendMessage(ctx, MessageEvent{
		ID:        "msg_001",
		Type:      "message",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_001",
		RunID:     "run_001",
		Role:      "user",
		Summary:   "hello",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("append message: %v", err)
	}
	if err := s.UpsertRun(ctx, RunSummary{
		ID:        "run_001",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_001",
		RunID:     "run_001",
		Status:    "running",
		Summary:   "accepted",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("upsert run: %v", err)
	}
	if err := s.AppendAudit(ctx, AuditEvent{
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

	s2, err := NewJSONLStore(ctx, JSONLConfig{Dir: dir})
	if err != nil {
		t.Fatalf("reload jsonl store: %v", err)
	}

	conversations, err := s2.ListConversations(ctx)
	if err != nil || len(conversations) != 1 || conversations[0].SessionID != "sess_001" {
		t.Fatalf("conversations = %+v, err=%v", conversations, err)
	}
	messages, err := s2.ListMessages(ctx, "sess_001")
	if err != nil || len(messages) != 1 || messages[0].ID != "msg_001" {
		t.Fatalf("messages = %+v, err=%v", messages, err)
	}
	runs, err := s2.ListRuns(ctx, "sess_001")
	if err != nil || len(runs) != 1 || runs[0].RunID != "run_001" {
		t.Fatalf("runs = %+v, err=%v", runs, err)
	}
	audit, err := s2.ListAuditEvents(ctx)
	if err != nil || len(audit) != 1 || audit[0].ID != "audit_001" {
		t.Fatalf("audit = %+v, err=%v", audit, err)
	}
}

func TestJSONLStoreSkipsCorruptedLinesAndRecordsIssue(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, messagesFile)

	content := strings.Join([]string{
		`{"id":"msg_001","type":"message","agentId":"a","sessionId":"sess_001","runId":"run_001","summary":"ok","createdAt":"2026-05-09T14:00:00Z"}`,
		`{"id":`,
		`{"id":"msg_002","type":"message","agentId":"a","sessionId":"sess_001","runId":"run_001","summary":"ok2","createdAt":"2026-05-09T14:00:01Z"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write corrupted jsonl: %v", err)
	}

	s, err := NewJSONLStore(ctx, JSONLConfig{Dir: dir})
	if err != nil {
		t.Fatalf("new jsonl store: %v", err)
	}

	messages, err := s.ListMessages(ctx, "sess_001")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(messages))
	}

	issues := s.LoadIssues()
	if len(issues) != 1 {
		t.Fatalf("issues len = %d, want 1", len(issues))
	}
	if issues[0].File != messagesFile || issues[0].Line != 2 || issues[0].Err == "" {
		t.Fatalf("unexpected issue: %+v", issues[0])
	}
}

func TestJSONLStoreRespectsContextCancellationOnWrite(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSONLStore(context.Background(), JSONLConfig{Dir: dir})
	if err != nil {
		t.Fatalf("new jsonl store: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = s.AppendMessage(ctx, MessageEvent{
		ID:        "msg_001",
		Type:      "message",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_001",
		RunID:     "run_001",
		Summary:   "blocked",
		CreatedAt: time.Now().UTC(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context canceled", err)
	}

	path := filepath.Join(dir, messagesFile)
	_, statErr := os.Stat(path)
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("messages file should not exist, stat err=%v", statErr)
	}
}
