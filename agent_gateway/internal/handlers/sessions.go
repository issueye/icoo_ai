package handlers

import (
	"net/http"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/pkg/httpx"
)

func (h *Handler) handleSessions(c *httpx.Context) {
	sessions, err := h.service.ListSessions(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, sessions)
}

func (h *Handler) handleSessionCreate(c *httpx.Context) {
	var req models.CreateSessionRequest
	if err := decodeJSON(c, &req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	session, err := h.service.CreateSession(c.Request.Context(), req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, session)
}

func (h *Handler) handleSessionGet(c *httpx.Context) {
	session, err := h.service.GetSession(c.Request.Context(), c.Param("sessionID"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, session)
}

func (h *Handler) handleSessionDelete(c *httpx.Context) {
	session, err := h.service.DeleteSession(c.Request.Context(), c.Param("sessionID"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, session)
}

func (h *Handler) handleSessionMessages(c *httpx.Context) {
	messages, err := h.service.ListSessionMessages(c.Request.Context(), c.Param("sessionID"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, messages)
}

func (h *Handler) handleSessionMessageCreate(c *httpx.Context) {
	var req models.PromptRequest
	if err := decodeJSON(c, &req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	response, err := h.service.CreateSessionMessage(c.Request.Context(), c.Param("sessionID"), req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, response)
}

func (h *Handler) handleSessionResume(c *httpx.Context) {
	var req models.ResumeSessionRequest
	if err := decodeJSON(c, &req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	session, err := h.service.ResumeSession(c.Request.Context(), c.Param("sessionID"), req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, session)
}

func (h *Handler) handleSessionMode(c *httpx.Context) {
	var req models.SetSessionModeRequest
	if err := decodeJSON(c, &req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	session, err := h.service.UpdateSessionMode(c.Request.Context(), c.Param("sessionID"), req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, session)
}

func (h *Handler) handleSessionConfig(c *httpx.Context) {
	var req models.SetSessionConfigOptionRequest
	if err := decodeJSON(c, &req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	session, err := h.service.UpdateSessionConfig(c.Request.Context(), c.Param("sessionID"), req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, session)
}

func (h *Handler) handleSessionRunCancel(c *httpx.Context) {
	run, err := h.service.CancelSessionRun(c.Request.Context(), c.Param("sessionID"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, run)
}

func (h *Handler) handleSessionClose(c *httpx.Context) {
	session, err := h.service.DeleteSession(c.Request.Context(), c.Param("sessionID"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, session)
}

func (h *Handler) handleRuns(c *httpx.Context) {
	runs, err := h.service.ListRuns(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, runs)
}
