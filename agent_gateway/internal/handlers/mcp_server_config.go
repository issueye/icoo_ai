package handlers

import (
	"net/http"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

func (h *Handler) handleMCPServerConfigAction(w http.ResponseWriter, r *http.Request) {
	action, ok := singleAction(r.URL.Path, "/v1/mcp-server-configs/")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	svc := h.service.MCPServerConfigs()
	handleCRUDAction[models.MCPServerConfig](w, r, action, svc.Create, svc.Update, svc.Delete, svc.Page, svc.List, svc.GetByID, svc.Status)
}
