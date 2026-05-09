package acp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

func TestStderrAuditSinkWritesSampledLines(t *testing.T) {
	t.Parallel()

	mem := store.NewMemoryStore()
	sink := NewStderrAuditSink(StderrAuditSinkOptions{
		Store:       mem,
		AgentID:     "agent-1",
		SessionID:   "sess-1",
		RunID:       "run-1",
		SampleEvery: 3,
		Now: func() time.Time {
			return time.Unix(1700000000, 0).UTC()
		},
	})

	payload := "l1\nl2\nl3\nl4\nl5\nl6\nl7\n"
	if _, err := sink.Write([]byte(payload)); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	events, err := mem.ListAuditEvents(context.Background())
	if err != nil {
		t.Fatalf("ListAuditEvents error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}
	if events[0].Summary != "l1" || events[1].Summary != "l4" || events[2].Summary != "l7" {
		t.Fatalf("unexpected sampled summaries: %+v", events)
	}
}

func TestStderrAuditSinkTruncatesLongLine(t *testing.T) {
	t.Parallel()

	mem := store.NewMemoryStore()
	sink := NewStderrAuditSink(StderrAuditSinkOptions{
		Store:           mem,
		AgentID:         "agent-1",
		MaxSummaryBytes: 8,
		SampleEvery:     1,
	})

	if _, err := sink.Write([]byte("1234567890\n")); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	events, err := mem.ListAuditEvents(context.Background())
	if err != nil {
		t.Fatalf("ListAuditEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if got := events[0].Summary; got != "12345678" {
		t.Fatalf("summary = %q", got)
	}
	v, ok := events[0].SafeMeta["truncated_bytes"].(int)
	if !ok || v != 2 {
		t.Fatalf("truncated_bytes = %v, ok=%v", v, ok)
	}
}

func TestStderrAuditSinkBuffersUntilNewline(t *testing.T) {
	t.Parallel()

	mem := store.NewMemoryStore()
	sink := NewStderrAuditSink(StderrAuditSinkOptions{Store: mem, SampleEvery: 1})
	if _, err := sink.Write([]byte("part")); err != nil {
		t.Fatalf("Write(part) error: %v", err)
	}
	if _, err := sink.Write([]byte("ial\n")); err != nil {
		t.Fatalf("Write(ial) error: %v", err)
	}

	events, err := mem.ListAuditEvents(context.Background())
	if err != nil {
		t.Fatalf("ListAuditEvents error: %v", err)
	}
	if len(events) != 1 || strings.TrimSpace(events[0].Summary) != "partial" {
		t.Fatalf("events = %+v", events)
	}
}

func TestStderrAuditSinkPersistsToJSONLAuditPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	jsonl, err := store.NewJSONLStore(context.Background(), store.JSONLConfig{Dir: dir})
	if err != nil {
		t.Fatalf("NewJSONLStore error: %v", err)
	}
	sink := NewStderrAuditSink(StderrAuditSinkOptions{Store: jsonl, SampleEvery: 1})
	if _, err := sink.Write([]byte("persist-me\n")); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	reloaded, err := store.NewJSONLStore(context.Background(), store.JSONLConfig{Dir: dir})
	if err != nil {
		t.Fatalf("reload store error: %v", err)
	}
	events, err := reloaded.ListAuditEvents(context.Background())
	if err != nil {
		t.Fatalf("ListAuditEvents error: %v", err)
	}
	if len(events) != 1 || events[0].Summary != "persist-me" {
		t.Fatalf("events = %+v", events)
	}

	auditPath := filepath.Join(dir, "audit.jsonl")
	if _, err := os.Stat(auditPath); err != nil {
		t.Fatalf("stat audit file: %v", err)
	}
}
