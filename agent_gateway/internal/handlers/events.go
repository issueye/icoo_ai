package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
)

func (h *Handler) handleEventStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_unsupported", "streaming unsupported")
		return
	}

	lastEventID := r.Header.Get("Last-Event-ID")
	if lastEventID == "" {
		lastEventID = r.URL.Query().Get("lastEventId")
	}
	sessionFilter := strings.TrimSpace(r.URL.Query().Get("sessionId"))
	agentFilter := strings.TrimSpace(r.URL.Query().Get("agentId"))
	sub, buffered := h.bus.Subscribe(r.Context(), lastEventID)
	defer sub.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for _, event := range buffered {
		if !matchesEventFilter(event, sessionFilter, agentFilter) {
			continue
		}
		if err := writeSSEEnvelope(w, event); err != nil {
			return
		}
	}
	flusher.Flush()

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-sub.Events():
			if !ok {
				return
			}
			if !matchesEventFilter(event, sessionFilter, agentFilter) {
				continue
			}
			if err := writeSSEEnvelope(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func matchesEventFilter(event events.Envelope, sessionID string, agentID string) bool {
	if sessionID != "" && event.SessionID != sessionID {
		return false
	}
	if agentID != "" && event.AgentID != agentID {
		return false
	}
	return true
}

func writeSSEEnvelope(w http.ResponseWriter, event any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	return nil
}
