package handlers

import (
	"net/http"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/pkg/httpx"
)

func (h *Handler) handleAgents(c *httpx.Context) {
	r := c.Request
	agents, err := h.service.ListAgents(r.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, agents)
}

func (h *Handler) handleSkills(c *httpx.Context) {
	r := c.Request
	skills, err := h.service.ListSkills(r.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, skills)
}

func (h *Handler) handleManagementSettings(c *httpx.Context) {
	r := c.Request
	settings, err := h.service.GetManagementSettings(r.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, settings)
}

func (h *Handler) handleManagementSettingsUpdate(c *httpx.Context) {
	r := c.Request
	var req models.ManagementSettings
	if err := decodeJSON(c, &req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	updated, err := h.service.ReplaceManagementSettings(r.Context(), req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, updated)
}
