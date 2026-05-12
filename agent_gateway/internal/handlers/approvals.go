package handlers

import (
	"net/http"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/pkg/httpx"
)

func (h *Handler) handleApprovals(c *httpx.Context) {
	r := c.Request
	approvals, err := h.service.ListApprovals(r.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, approvals)
}

func (h *Handler) handleApprovalAction(c *httpx.Context) {
	r := c.Request
	var req models.ApprovalDecisionRequest
	if err := decodeJSON(c, &req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	approval, err := h.service.UpdateApprovalDecision(r.Context(), c.Param("approvalID"), req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, approval)
}
