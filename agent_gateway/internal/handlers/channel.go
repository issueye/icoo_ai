package handlers

import (
	"net/http"

	"github.com/icoo-ai/icoo-ai/agent_gateway/pkg/httpx"
)

func (h *Handler) handleChannelAction(c *httpx.Context) {
	action, ok := singleAction(c.Request.URL.Path, "/v1/channels/")
	if !ok {
		writeError(c, http.StatusNotFound, "not_found", "route not found")
		return
	}
	handleCRUDAction(c, action, h.service.Channel())
}
