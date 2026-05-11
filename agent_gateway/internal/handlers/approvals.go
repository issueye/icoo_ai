package handlers

import (
	"net/http"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

func (h *Handler) handleApprovals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	approvals, err := h.service.ListApprovals(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, approvals)
}

func (h *Handler) handleApprovalAction(w http.ResponseWriter, r *http.Request) {
	parts, ok := splitPath(r.URL.Path, "/v1/approvals/")
	if !ok || len(parts) != 2 || parts[1] != "decision" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		w.Header().Set("Allow", "PUT, POST")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req models.ApprovalDecisionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	approval, err := h.service.UpdateApprovalDecision(r.Context(), parts[0], req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, approval)
}
