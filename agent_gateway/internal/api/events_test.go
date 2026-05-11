package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/api"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/projection"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

func TestEventStreamReceivesEnvelope(t *testing.T) {
	router := api.NewRouter(service.NewGatewayService())
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

func TestEventStreamConcurrentSubscribersKeepSessionIdentity(t *testing.T) {
	router := api.NewRouter(service.NewGatewayService())
	srv := httptest.NewServer(router)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	type result struct {
		name string
		evt  events.Envelope
		err  error
	}
	targetID := "evt_test_stream_concurrent_1"
	readOne := func(name string) result {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/events/stream", nil)
		if err != nil {
			return result{name: name, err: err}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return result{name: name, err: err}
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var got events.Envelope
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &got); err != nil {
				return result{name: name, err: err}
			}
			if got.ID == targetID {
				return result{name: name, evt: got}
			}
		}
		if err := scanner.Err(); err != nil {
			return result{name: name, err: err}
		}
		return result{name: name, err: context.DeadlineExceeded}
	}

	results := make(chan result, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		results <- readOne("sub-a")
	}()
	go func() {
		defer wg.Done()
		results <- readOne("sub-b")
	}()

	time.Sleep(50 * time.Millisecond)
	want := events.Envelope{
		ID:        targetID,
		Type:      "message.created",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_concurrent_1",
		RunID:     "run_concurrent_1",
		Payload: map[string]any{
			"content": "hello",
		},
		CreatedAt: time.Now().UTC(),
	}
	events.DefaultBus().Publish(want)

	wg.Wait()
	close(results)

	for res := range results {
		if res.err != nil {
			t.Fatalf("%s read stream: %v", res.name, res.err)
		}
		if res.evt.ID != want.ID || res.evt.SessionID != want.SessionID || res.evt.RunID != want.RunID {
			t.Fatalf("%s unexpected event identity: got %#v want %#v", res.name, res.evt, want)
		}
	}
}

func TestEventStreamFiltersBySessionID(t *testing.T) {
	router := api.NewRouter(service.NewGatewayService())
	srv := httptest.NewServer(router)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/events/stream?sessionId=sess_target", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer resp.Body.Close()

	events.DefaultBus().Publish(events.Envelope{ID: "evt_other", Type: "message.created", SessionID: "sess_other", AgentID: "agent_a", CreatedAt: time.Now().UTC()})
	want := events.Envelope{ID: "evt_target", Type: "message.created", SessionID: "sess_target", AgentID: "agent_a", CreatedAt: time.Now().UTC()}
	events.DefaultBus().Publish(want)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var got events.Envelope
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &got); err != nil {
			t.Fatalf("unmarshal envelope: %v", err)
		}
		if got.ID != want.ID {
			t.Fatalf("expected %s, got %s", want.ID, got.ID)
		}
		if got.SessionID != "sess_target" {
			t.Fatalf("expected session sess_target, got %s", got.SessionID)
		}
		return
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	t.Fatal("stream closed before receiving filtered event")
}

func TestEventStreamFiltersByAgentID(t *testing.T) {
	router := api.NewRouter(service.NewGatewayService())
	srv := httptest.NewServer(router)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/events/stream?agentId=agent_target", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer resp.Body.Close()

	events.DefaultBus().Publish(events.Envelope{ID: "evt_other_agent", Type: "message.created", SessionID: "sess_1", AgentID: "agent_other", CreatedAt: time.Now().UTC()})
	want := events.Envelope{ID: "evt_target_agent", Type: "message.created", SessionID: "sess_1", AgentID: "agent_target", CreatedAt: time.Now().UTC()}
	events.DefaultBus().Publish(want)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var got events.Envelope
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &got); err != nil {
			t.Fatalf("unmarshal envelope: %v", err)
		}
		if got.ID != want.ID {
			t.Fatalf("expected %s, got %s", want.ID, got.ID)
		}
		if got.AgentID != "agent_target" {
			t.Fatalf("expected agent agent_target, got %s", got.AgentID)
		}
		return
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	t.Fatal("stream closed before receiving filtered event")
}

func TestRunAndMessageEventsProjectedToHistoryAPI(t *testing.T) {
	memStore := store.NewMemoryStore()
	router := api.NewRouter(service.NewGatewayServiceWithStore(memStore))
	session := doJSON[service.Session](t, router, http.MethodPost, "/v1/sessions", map[string]string{
		"title": "projection-history",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	sub, _ := events.DefaultBus().Subscribe(ctx, "")
	defer sub.Close()

	runEvent := events.Envelope{
		ID:        "evt_projection_run",
		Type:      "run.updated",
		AgentID:   session.AgentID,
		SessionID: session.ID,
		RunID:     "run_projection_1",
		Payload: map[string]any{
			"status": "completed",
		},
		CreatedAt: time.Date(2026, 5, 9, 16, 20, 0, 0, time.UTC),
	}
	messageEvent := events.Envelope{
		ID:        "evt_projection_message",
		Type:      "message.created",
		AgentID:   session.AgentID,
		SessionID: session.ID,
		RunID:     "run_projection_1",
		Payload: map[string]any{
			"role":    "assistant",
			"content": "projected message content",
		},
		CreatedAt: time.Date(2026, 5, 9, 16, 20, 1, 0, time.UTC),
	}

	projected := make(chan error, 1)
	go func() {
		targets := map[string]struct{}{
			runEvent.ID:     {},
			messageEvent.ID: {},
		}
		seen := make(map[string]struct{}, len(targets))
		for {
			select {
			case <-ctx.Done():
				projected <- ctx.Err()
				return
			case envelope, ok := <-sub.Events():
				if !ok {
					projected <- context.Canceled
					return
				}
				if _, wanted := targets[envelope.ID]; !wanted {
					continue
				}
				if _, err := projection.Apply(context.Background(), memStore, envelope); err != nil {
					projected <- err
					return
				}
				seen[envelope.ID] = struct{}{}
				if len(seen) == len(targets) {
					projected <- nil
					return
				}
			}
		}
	}()

	events.DefaultBus().Publish(runEvent)
	events.DefaultBus().Publish(messageEvent)

	if err := <-projected; err != nil {
		t.Fatalf("event projection failed: %v", err)
	}

	messages := doJSON[[]service.Message](t, router, http.MethodGet, "/v1/sessions/"+session.ID+"/messages", nil)
	if len(messages) != 2 {
		t.Fatalf("expected 2 projected history messages, got %d", len(messages))
	}
	byID := make(map[string]service.Message, len(messages))
	for _, message := range messages {
		byID[message.ID] = message
	}
	if msg := byID[runEvent.ID]; msg.ID == "" || msg.RunID != runEvent.RunID || msg.SessionID != session.ID {
		t.Fatalf("expected projected run event message, got %#v", msg)
	}
	if msg := byID[messageEvent.ID]; msg.ID == "" || msg.Role != "assistant" || msg.RunID != messageEvent.RunID {
		t.Fatalf("expected projected assistant message event, got %#v", msg)
	}

	runs := doJSON[[]service.Run](t, router, http.MethodGet, "/v1/runs", nil)
	if len(runs) != 1 {
		t.Fatalf("expected 1 projected run, got %d", len(runs))
	}
	if runs[0].ID != runEvent.RunID || runs[0].SessionID != session.ID || runs[0].Status != "completed" {
		t.Fatalf("unexpected projected run: %#v", runs[0])
	}
}
