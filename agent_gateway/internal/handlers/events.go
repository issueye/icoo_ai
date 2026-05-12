package handlers

import "github.com/icoo-ai/icoo-ai/agent_gateway/pkg/httpx"

func (h *Handler) handleEventWebSocket(c *httpx.Context) {
	h.wsHub.Serve(c.Request.Context(), c.Writer, c.Request)
}
