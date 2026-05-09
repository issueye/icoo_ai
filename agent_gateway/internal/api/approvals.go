package api

import (
	"net/http"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
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
	approvalID, action, ok := approvalSubpath(r.URL.Path)
	if !ok || action != "decision" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req service.ApprovalDecisionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	approval, err := h.service.DecideApproval(r.Context(), approvalID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, approval)
}
