package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
)

type Handler struct {
	service service.GatewayService
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewRouter(gateway service.GatewayService) http.Handler {
	h := &Handler{service: gateway}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/agents", h.handleAgents)
	mux.HandleFunc("/v1/sessions", h.handleSessions)
	mux.HandleFunc("/v1/sessions/", h.handleSessionAction)
	mux.HandleFunc("/v1/runs", h.handleRuns)
	mux.HandleFunc("/v1/approvals", h.handleApprovals)
	mux.HandleFunc("/v1/approvals/", h.handleApprovalAction)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{Code: code, Message: message})
}

func writeServiceError(w http.ResponseWriter, err error) {
	var serviceErr *service.Error
	if errors.As(err, &serviceErr) {
		writeError(w, statusForServiceCode(serviceErr.Code), serviceErr.Code, serviceErr.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
}

func statusForServiceCode(code string) int {
	switch code {
	case "agent_not_found", "session_not_found", "approval_not_found":
		return http.StatusNotFound
	case "invalid_prompt", "invalid_decision":
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func sessionSubpath(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, "/v1/sessions/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func approvalSubpath(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, "/v1/approvals/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
