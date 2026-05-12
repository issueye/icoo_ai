package handlers

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/pkg/wshub"
)

type eventSource struct {
	bus *events.Bus
}

type eventSubscription struct {
	sub  *events.Subscription
	ch   chan any
	done chan struct{}
	once sync.Once
}

func newEventSource(bus *events.Bus) wshub.EventSource {
	if bus == nil {
		bus = events.DefaultBus()
	}
	return eventSource{bus: bus}
}

func (s eventSource) Subscribe(ctx context.Context, lastEventID string) (wshub.Subscription, []any) {
	sub, buffered := s.bus.Subscribe(ctx, lastEventID)
	out := make([]any, 0, len(buffered))
	for _, event := range buffered {
		out = append(out, event)
	}
	wrapped := &eventSubscription{
		sub:  sub,
		ch:   make(chan any, 32),
		done: make(chan struct{}),
	}
	go wrapped.forward()
	return wrapped, out
}

func (s *eventSubscription) Events() <-chan any {
	return s.ch
}

func (s *eventSubscription) Close() {
	s.once.Do(func() {
		close(s.done)
		s.sub.Close()
	})
}

func (s *eventSubscription) forward() {
	defer close(s.ch)
	for {
		select {
		case <-s.done:
			return
		case event, ok := <-s.sub.Events():
			if !ok {
				return
			}
			select {
			case s.ch <- event:
			case <-s.done:
				return
			}
		}
	}
}

func eventFilter(event any, r *http.Request) bool {
	envelope, ok := event.(models.EventEnvelope)
	if !ok {
		return true
	}
	sessionID := strings.TrimSpace(r.URL.Query().Get("sessionId"))
	if sessionID != "" && envelope.SessionID != sessionID {
		return false
	}
	agentID := strings.TrimSpace(r.URL.Query().Get("agentId"))
	if agentID != "" && envelope.AgentID != agentID {
		return false
	}
	return true
}
