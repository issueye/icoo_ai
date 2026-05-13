package handlers

import (
	"net/http"

	"github.com/icoo-ai/icoo-ai/agent_gateway/pkg/httpx"
)

func (h *Handler) handleAgentAction(c *httpx.Context) {
	action, ok := singleAction(c.Request.URL.Path, "/v1/agents/")
	if !ok {
		writeError(c, http.StatusNotFound, "not_found", "route not found")
		return
	}
	handleCRUDAction(c, action, h.service.Agent())
}
