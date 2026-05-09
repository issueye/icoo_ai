package testutil

import (
	"context"
	"errors"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/llm"
)

func TestMockLLMProviderStreamsScriptAndRecordsCalls(t *testing.T) {
	provider := NewMockLLMProvider("test",
		[]llm.CompletionEvent{
			{Type: llm.CompletionEventMessageDelta, Delta: "hello"},
			{Type: llm.CompletionEventCompleted},
		},
	)

	stream, err := provider.Stream(context.Background(), llm.CompletionRequest{Model: "model-a"})
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}

	var events []llm.CompletionEvent
	for event := range stream {
		events = append(events, event)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Delta != "hello" {
		t.Fatalf("first delta = %q, want hello", events[0].Delta)
	}

	calls := provider.Calls()
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	if calls[0].Model != "model-a" {
		t.Fatalf("model = %q, want model-a", calls[0].Model)
	}
}

func TestMockLLMProviderStreamError(t *testing.T) {
	want := errors.New("boom")
	provider := NewMockLLMProvider("test")
	provider.StreamErr = want

	_, err := provider.Stream(context.Background(), llm.CompletionRequest{})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
