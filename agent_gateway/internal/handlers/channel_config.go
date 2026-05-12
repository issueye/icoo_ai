package handlers

import (
	"net/http"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/pkg/httpx"
)

func (h *Handler) handleChannelConfigAction(c *httpx.Context) {
	action, ok := singleAction(c.Request.URL.Path, "/v1/channel-configs/")
	if !ok {
		writeError(c, http.StatusNotFound, "not_found", "route not found")
		return
	}
	svc := h.service.ChannelConfigs()
	handleCRUDAction[models.ChannelConfig](c, action, svc.Create, svc.Update, svc.Delete, svc.Page, svc.List, svc.GetByID, svc.Status)
}
