package testutil

import (
	"context"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/agent"
)

func TestCollectRuntimeEvents(t *testing.T) {
	ch := make(chan agent.Event, 2)
	ch <- agent.Event{Type: agent.EventRunStarted}
	ch <- agent.Event{Type: agent.EventMessageDelta, Content: "chunk"}
	close(ch)

	events, err := CollectRuntimeEvents(context.Background(), ch)
	if err != nil {
		t.Fatalf("CollectRuntimeEvents returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[1].Content != "chunk" {
		t.Fatalf("content = %q, want chunk", events[1].Content)
	}
}

func TestRuntimeEventCollectorContents(t *testing.T) {
	collector := NewRuntimeEventCollector()
	collector.Add(agent.Event{Type: agent.EventRunStarted})
	collector.Add(agent.Event{Type: agent.EventMessageDelta, Content: "a"})
	collector.Add(agent.Event{Type: agent.EventMessageDelta, Content: "b"})

	contents := collector.Contents()
	if len(contents) != 2 || contents[0] != "a" || contents[1] != "b" {
		t.Fatalf("contents = %#v, want [a b]", contents)
	}
}
