package api

import (
	"net/http"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
)

func (h *Handler) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	agents, err := h.service.ListAgents(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (h *Handler) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	skills, err := h.service.ListSkills(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listSessions(w, r)
	case http.MethodPost:
		h.createSession(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	var req service.CreateSessionRequest
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
}

func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.service.ListSessions(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (h *Handler) handleSessionAction(w http.ResponseWriter, r *http.Request) {
	sessionID, action, ok := sessionPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	switch action {
	case "":
		if r.Method != http.MethodGet && r.Method != http.MethodDelete {
			w.Header().Set("Allow", "GET, DELETE")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if r.Method == http.MethodDelete {
			h.closeSession(w, r, sessionID)
			return
		}
		h.getSession(w, r, sessionID)
	case "messages":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		h.listMessages(w, r, sessionID)
	case "prompt":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		h.prompt(w, r, sessionID)
	case "cancel":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		h.cancel(w, r, sessionID)
	case "resume":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		h.resumeSession(w, r, sessionID)
	case "mode":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		h.setSessionMode(w, r, sessionID)
	case "config":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		h.setSessionConfigOption(w, r, sessionID)
	case "close":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		h.closeSession(w, r, sessionID)
	default:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

func (h *Handler) getSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	session, err := h.service.GetSession(r.Context(), sessionID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request, sessionID string) {
	messages, err := h.service.ListMessages(r.Context(), sessionID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func (h *Handler) prompt(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req service.PromptRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	response, err := h.service.Prompt(r.Context(), sessionID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request, sessionID string) {
	run, err := h.service.Cancel(r.Context(), sessionID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *Handler) resumeSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req service.ResumeSessionRequest
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

func (h *Handler) setSessionMode(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req service.SetSessionModeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	session, err := h.service.SetSessionMode(r.Context(), sessionID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) setSessionConfigOption(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req service.SetSessionConfigOptionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	session, err := h.service.SetSessionConfigOption(r.Context(), sessionID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) closeSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	session, err := h.service.CloseSession(r.Context(), sessionID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
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
