package handlers

import (
	"net/http"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/pkg/httpx"
)

func (h *Handler) handleScheduleTaskConfigAction(c *httpx.Context) {
	action, ok := singleAction(c.Request.URL.Path, "/v1/schedule-task-configs/")
	if !ok {
		writeError(c, http.StatusNotFound, "not_found", "route not found")
		return
	}
	svc := h.service.ScheduleTaskConfigs()
	handleCRUDAction[models.ScheduleTaskConfig](c, action, svc.Create, svc.Update, svc.Delete, svc.Page, svc.List, svc.GetByID, svc.Status)
}
