package handlers

import (
	"net/http"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sessions, err := h.service.ListSessions(r.Context())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sessions)
	case http.MethodPost:
		var req models.CreateSessionRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		session, err := h.service.CreateSession(r.Context(), req)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, session)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (h *Handler) handleSessionAction(w http.ResponseWriter, r *http.Request) {
	parts, ok := splitPath(r.URL.Path, "/v1/sessions/")
	if !ok || len(parts) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	sessionID := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			session, err := h.service.GetSession(r.Context(), sessionID)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, session)
		case http.MethodDelete:
			session, err := h.service.DeleteSession(r.Context(), sessionID)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, session)
		default:
			w.Header().Set("Allow", "GET, DELETE")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
		return
	}

	switch parts[1] {
	case "messages":
		h.handleSessionMessages(w, r, sessionID)
	case "resume":
		h.handleSessionResume(w, r, sessionID)
	case "mode":
		h.handleSessionMode(w, r, sessionID)
	case "config":
		h.handleSessionConfig(w, r, sessionID)
	case "runs":
		h.handleSessionRuns(w, r, sessionID, parts[2:])
	case "prompt":
		// Compatibility route: convert legacy prompt action to CRUD-style message create.
		h.handleSessionMessageCreate(w, r, sessionID)
	case "cancel":
		// Compatibility route: cancel current run for the session.
		h.handleSessionRunCancel(w, r, sessionID)
	case "close":
		// Compatibility route: delete/close session.
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		session, err := h.service.DeleteSession(r.Context(), sessionID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, session)
	default:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

func (h *Handler) handleSessionMessages(w http.ResponseWriter, r *http.Request, sessionID string) {
	switch r.Method {
	case http.MethodGet:
		messages, err := h.service.ListSessionMessages(r.Context(), sessionID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, messages)
	case http.MethodPost:
		h.handleSessionMessageCreate(w, r, sessionID)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (h *Handler) handleSessionMessageCreate(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req models.PromptRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	response, err := h.service.CreateSessionMessage(r.Context(), sessionID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) handleSessionResume(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req models.ResumeSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	session, err := h.service.ResumeSession(r.Context(), sessionID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) handleSessionMode(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", http.MethodPut)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req models.SetSessionModeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	session, err := h.service.UpdateSessionMode(r.Context(), sessionID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) handleSessionConfig(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", http.MethodPut)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req models.SetSessionConfigOptionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	session, err := h.service.UpdateSessionConfig(r.Context(), sessionID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) handleSessionRuns(w http.ResponseWriter, r *http.Request, sessionID string, parts []string) {
	if len(parts) == 1 && parts[0] == "cancel" {
		h.handleSessionRunCancel(w, r, sessionID)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "route not found")
}

func (h *Handler) handleSessionRunCancel(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	run, err := h.service.CancelSessionRun(r.Context(), sessionID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *Handler) handleRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	runs, err := h.service.ListRuns(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}
