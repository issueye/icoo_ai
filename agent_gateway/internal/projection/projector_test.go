package projection

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

func TestBuildMessageEvent(t *testing.T) {
	now := time.Date(2026, 5, 9, 15, 4, 5, 0, time.UTC)
	longContent := strings.Repeat("x", 900)
	envelope := events.Envelope{
		ID:        "evt_1",
		Type:      "message.created",
		AgentID:   "agent_1",
		SessionID: "sess_1",
		RunID:     "run_1",
		Payload: map[string]any{
			"role":    "assistant",
			"content": longContent,
		},
		CreatedAt: now,
	}

	result := Build(envelope)
	if result.Ignored {
		t.Fatal("expected envelope to be projected")
	}
	if result.Run != nil {
		t.Fatal("expected no run upsert when payload has no status")
	}
	if result.Message.ID != "evt_1" || result.Message.Type != "message.created" {
		t.Fatalf("unexpected message identity: %#v", result.Message)
	}
	if result.Message.SessionID != "sess_1" || result.Message.RunID != "run_1" {
		t.Fatalf("unexpected message route fields: %#v", result.Message)
	}
	if result.Message.Role != "assistant" {
		t.Fatalf("unexpected role: %q", result.Message.Role)
	}
	if got := len([]rune(result.Message.Summary)); got != MaxSummaryChars {
		t.Fatalf("expected summary len %d, got %d", MaxSummaryChars, got)
	}
	if digest, ok := result.Message.SafeMeta["payloadDigest"].(string); !ok || digest == "" {
		t.Fatalf("expected payload digest in safe meta, got %#v", result.Message.SafeMeta)
	}
}

func TestBuildRunStatusEvent(t *testing.T) {
	now := time.Date(2026, 5, 9, 15, 10, 0, 0, time.UTC)
	envelope := events.Envelope{
		ID:        "evt_2",
		Type:      "run.updated",
		AgentID:   "agent_1",
		SessionID: "sess_2",
		RunID:     "run_2",
		Payload: map[string]any{
			"status": "completed",
		},
		CreatedAt: now,
	}

	result := Build(envelope)
	if result.Ignored {
		t.Fatal("expected envelope to be projected")
	}
	if result.Run == nil {
		t.Fatal("expected run upsert when payload has status")
	}
	if result.Run.RunID != "run_2" || result.Run.Status != "completed" {
		t.Fatalf("unexpected run projection: %#v", result.Run)
	}
	if result.Run.CompletedAt == nil {
		t.Fatal("expected completedAt for terminal status")
	}
}

func TestBuildIgnoreWhenMissingSessionID(t *testing.T) {
	result := Build(events.Envelope{
		ID:      "evt_3",
		Type:    "message.created",
		Payload: map[string]any{"content": "hello"},
	})
	if !result.Ignored {
		t.Fatal("expected event without sessionId to be ignored")
	}
}

func TestBuildPayloadAbnormalNoPanic(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Build should not panic, got %v", recovered)
		}
	}()

	result := Build(events.Envelope{
		ID:        "evt_4",
		Type:      "message.created",
		SessionID: "sess_4",
		Payload:   panicMarshaler{},
		CreatedAt: time.Date(2026, 5, 9, 16, 0, 0, 0, time.UTC),
	})
	if result.Ignored {
		t.Fatal("expected projection result")
	}
	if result.Message.ID == "" {
		t.Fatal("expected message id")
	}
}

func TestApplyWritesMessageAndOptionalRun(t *testing.T) {
	st := store.NewMemoryStore()
	ctx := context.Background()
	now := time.Date(2026, 5, 9, 16, 10, 0, 0, time.UTC)

	_, err := Apply(ctx, st, events.Envelope{
		ID:        "evt_5",
		Type:      "run.updated",
		AgentID:   "agent_1",
		SessionID: "sess_5",
		RunID:     "run_5",
		Payload: map[string]any{
			"status": "in_progress",
		},
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	messages, err := st.ListMessages(ctx, "sess_5")
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message event, got %d", len(messages))
	}

	runs, err := st.ListRuns(ctx, "sess_5")
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run summary, got %d", len(runs))
	}
	if runs[0].Status != "in_progress" {
		t.Fatalf("unexpected run status: %q", runs[0].Status)
	}
}

func TestApplyWritesApprovalForApprovalEvent(t *testing.T) {
	st := store.NewMemoryStore()
	ctx := context.Background()
	now := time.Date(2026, 5, 9, 16, 20, 0, 0, time.UTC)

	result, err := Apply(ctx, st, events.Envelope{
		ID:        "evt_appr_1",
		Type:      "approval.requested",
		AgentID:   "agent_1",
		SessionID: "sess_appr_1",
		RunID:     "run_appr_1",
		Payload: map[string]any{
			"id":        "approval_1",
			"requestId": "connreq_1",
			"action":    "write_file",
			"message":   "need approval",
		},
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.Approval == nil {
		t.Fatal("expected approval projection result")
	}

	approvals, err := st.ListApprovals(ctx)
	if err != nil {
		t.Fatalf("ListApprovals() error = %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("expected 1 approval, got %d", len(approvals))
	}
	if approvals[0].ID != "approval_1" || approvals[0].ConnectorRequestID != "connreq_1" || approvals[0].Status != "pending" {
		t.Fatalf("unexpected approval: %#v", approvals[0])
	}
}

type panicMarshaler struct{}

func (panicMarshaler) MarshalJSON() ([]byte, error) {
	panic("panic marshaler")
}
