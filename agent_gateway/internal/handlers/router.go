package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/services"
)

type Handler struct {
	service services.GatewayCRUD
	bus     *events.Bus
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewRouter(gateway services.GatewayCRUD) http.Handler {
	h := &Handler{
		service: gateway,
		bus:     events.DefaultBus(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/agents", h.handleAgents)
	mux.HandleFunc("/v1/agent-configs/", h.handleAgentConfigAction)
	mux.HandleFunc("/v1/skills", h.handleSkills)
	mux.HandleFunc("/v1/channel-configs/", h.handleChannelConfigAction)
	mux.HandleFunc("/v1/mcp-server-configs/", h.handleMCPServerConfigAction)
	mux.HandleFunc("/v1/schedule-task-configs/", h.handleScheduleTaskConfigAction)
	mux.HandleFunc("/v1/management/settings", h.handleManagementSettings)
	mux.HandleFunc("/v1/sessions", h.handleSessions)
	mux.HandleFunc("/v1/sessions/", h.handleSessionAction)
	mux.HandleFunc("/v1/runs", h.handleRuns)
	mux.HandleFunc("/v1/approvals", h.handleApprovals)
	mux.HandleFunc("/v1/approvals/", h.handleApprovalAction)
	mux.HandleFunc("/v1/events/stream", h.handleEventStream)
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
	var gatewayErr *services.GatewayError
	if errors.As(err, &gatewayErr) {
		writeError(w, statusForServiceCode(gatewayErr.Code), gatewayErr.Code, gatewayErr.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
}

func statusForServiceCode(code string) int {
	switch code {
	case "agent_not_found", "session_not_found", "approval_not_found":
		return http.StatusNotFound
	case "not_found", "agent_config_not_found", "channel_config_not_found", "mcp_server_config_not_found", "schedule_task_config_not_found":
		return http.StatusNotFound
	case "invalid_prompt", "invalid_decision", "invalid_session", "invalid_session_config", "duplicate_id":
		return http.StatusBadRequest
	case "connector_unavailable":
		return http.StatusServiceUnavailable
	case "connector_request_failed":
		return http.StatusBadGateway
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

func splitPath(path, prefix string) ([]string, bool) {
	rest := strings.TrimPrefix(path, prefix)
	if rest == path {
		return nil, false
	}
	if strings.TrimSpace(rest) == "" {
		return []string{}, true
	}
	parts := strings.Split(rest, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		out = append(out, part)
	}
	return out, true
}
