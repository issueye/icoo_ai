package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/api"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
)

func TestEventStreamReceivesEnvelope(t *testing.T) {
	router := api.NewRouter(service.NewMockGatewayService())
	srv := httptest.NewServer(router)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/events/stream", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", got)
	}

	want := events.Envelope{
		ID:        "evt_test_stream_1",
		Type:      "run.updated",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_1",
		RunID:     "run_1",
		Payload: map[string]any{
			"status": "completed",
		},
		CreatedAt: time.Now().UTC(),
	}
	events.DefaultBus().Publish(want)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		raw := strings.TrimPrefix(line, "data: ")

		var got events.Envelope
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatalf("unmarshal envelope: %v", err)
		}

		if got.ID != want.ID || got.Type != want.Type || got.AgentID != want.AgentID || got.SessionID != want.SessionID || got.RunID != want.RunID {
			t.Fatalf("unexpected envelope identity fields: got %#v want %#v", got, want)
		}
		if got.CreatedAt.IsZero() {
			t.Fatal("expected non-zero createdAt")
		}
		return
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	t.Fatal("stream closed before receiving event")
}
