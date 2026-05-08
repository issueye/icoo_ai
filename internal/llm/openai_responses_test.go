package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/tools"
)

func TestOpenAIResponsesProviderStreamsText(t *testing.T) {
	var gotAuth string
	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/responses" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(t, w, `{"type":"response.output_text.delta","delta":"hel"}`)
		writeSSE(t, w, `{"type":"response.output_text.delta","delta":"lo"}`)
		writeSSE(t, w, `{"type":"response.completed"}`)
	}))
	defer server.Close()

	provider := newTestOpenAIResponsesProvider(t, server.URL)
	events, err := provider.Stream(context.Background(), CompletionRequest{
		Model:    "gpt-test",
		Messages: []Message{{Role: "user", Content: "hi"}},
		Options: CompletionOptions{
			MaxOutputTokens: 12,
			ReasoningEffort: "low",
			StructuredOutput: map[string]any{
				"type": "json_schema",
				"name": "result",
			},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	got := collectCompletionEvents(t, events)
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if text := completionText(got); text != "hello" {
		t.Fatalf("text = %q", text)
	}
	if got[len(got)-1].Type != CompletionEventCompleted {
		t.Fatalf("last event = %s", got[len(got)-1].Type)
	}
	if gotRequest["model"] != "gpt-test" {
		t.Fatalf("model = %v", gotRequest["model"])
	}
	if gotRequest["stream"] != true {
		t.Fatalf("stream = %v", gotRequest["stream"])
	}
	if _, ok := gotRequest["reasoning"].(map[string]any); !ok {
		t.Fatalf("reasoning was not sent: %#v", gotRequest["reasoning"])
	}
	if _, ok := gotRequest["text"].(map[string]any); !ok {
		t.Fatalf("text config was not sent: %#v", gotRequest["text"])
	}
}

func TestOpenAIResponsesProviderStreamsToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		toolsValue, ok := got["tools"].([]any)
		if !ok || len(toolsValue) != 1 {
			t.Fatalf("tools = %#v", got["tools"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(t, w, `{"type":"response.function_call_arguments.delta","item_id":"fc_item","delta":"{\"path\""}`)
		writeSSE(t, w, `{"type":"response.function_call_arguments.delta","item_id":"fc_item","delta":":\"README.md\"}"}`)
		writeSSE(t, w, `{"type":"response.output_item.done","item":{"type":"function_call","id":"fc_item","call_id":"call_123","name":"read_file","arguments":"{\"path\":\"README.md\"}"}}`)
		writeSSE(t, w, `{"type":"response.completed"}`)
	}))
	defer server.Close()

	provider := newTestOpenAIResponsesProvider(t, server.URL)
	events, err := provider.Stream(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "read"}},
		Tools: []tools.ToolDefinition{{
			Name:        "read_file",
			Description: "Read a file",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	got := collectCompletionEvents(t, events)
	call := firstToolCall(got)
	if call == nil {
		t.Fatalf("tool call not emitted: %#v", got)
	}
	if call.ID != "call_123" || call.Name != "read_file" {
		t.Fatalf("tool call = %#v", call)
	}
	if call.ItemID != "fc_item" {
		t.Fatalf("item id = %q", call.ItemID)
	}
	if string(call.Arguments) != `{"path":"README.md"}` {
		t.Fatalf("arguments = %s", call.Arguments)
	}
}

func TestOpenAIResponsesProviderSendsFunctionCallContext(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(t, w, `{"type":"response.output_text.delta","delta":"ok"}`)
		writeSSE(t, w, `{"type":"response.completed"}`)
	}))
	defer server.Close()

	provider := newTestOpenAIResponsesProvider(t, server.URL)
	events, err := provider.Stream(context.Background(), CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "read"},
			{Role: "assistant", ToolCalls: []tools.ToolCall{{
				ID:        "call_123",
				ItemID:    "fc_item",
				Name:      "read_file",
				Arguments: json.RawMessage(`{"path":"README.md"}`),
			}}},
			{Role: "tool", Content: `{"ok":true}`, Metadata: map[string]any{"tool_call_id": "call_123"}},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	_ = collectCompletionEvents(t, events)
	input := got["input"].([]any)
	if len(input) != 3 {
		t.Fatalf("input len = %d, input=%#v", len(input), input)
	}
	callItem := input[1].(map[string]any)
	if callItem["type"] != "function_call" || callItem["id"] != "fc_item" || callItem["call_id"] != "call_123" {
		t.Fatalf("function call item = %#v", callItem)
	}
	outputItem := input[2].(map[string]any)
	if outputItem["type"] != "function_call_output" || outputItem["call_id"] != "call_123" {
		t.Fatalf("function call output item = %#v", outputItem)
	}
}

func TestOpenAIResponsesProviderErrorResponseRedactsAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad request"}}`, http.StatusBadRequest)
	}))
	defer server.Close()

	provider := newTestOpenAIResponsesProvider(t, server.URL)
	_, err := provider.Stream(context.Background(), CompletionRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Fatal("Stream() error = nil")
	}
	if strings.Contains(err.Error(), "test-key") {
		t.Fatalf("error leaked API key: %v", err)
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("error = %v", err)
	}
}

func TestOpenAIResponsesProviderCancelBeforeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider, err := NewOpenAIResponsesProvider(OpenAIResponsesConfig{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		Model:      "gpt-default",
		HTTPClient: &http.Client{Timeout: 10 * time.Millisecond},
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesProvider() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = provider.Stream(ctx, CompletionRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Fatal("Stream() error = nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stream() error = %v, want deadline exceeded", err)
	}
	if strings.Contains(err.Error(), "test-key") {
		t.Fatalf("error leaked API key: %v", err)
	}
}

func newTestOpenAIResponsesProvider(t *testing.T, baseURL string) *OpenAIResponsesProvider {
	t.Helper()
	provider, err := NewOpenAIResponsesProvider(OpenAIResponsesConfig{
		APIKey:  "test-key",
		BaseURL: baseURL,
		Model:   "gpt-default",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesProvider() error = %v", err)
	}
	return provider
}

func writeSSE(t *testing.T, w http.ResponseWriter, data string) {
	t.Helper()
	if _, err := w.Write([]byte("data: " + data + "\n\n")); err != nil {
		t.Fatalf("write SSE: %v", err)
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func collectCompletionEvents(t *testing.T, events <-chan CompletionEvent) []CompletionEvent {
	t.Helper()
	var got []CompletionEvent
	for event := range events {
		got = append(got, event)
	}
	return got
}

func completionText(events []CompletionEvent) string {
	var text strings.Builder
	for _, event := range events {
		if event.Type == CompletionEventMessageDelta {
			text.WriteString(event.Delta)
		}
	}
	return text.String()
}

func firstToolCall(events []CompletionEvent) *tools.ToolCall {
	for _, event := range events {
		if event.Type == CompletionEventToolCall {
			return event.ToolCall
		}
	}
	return nil
}
