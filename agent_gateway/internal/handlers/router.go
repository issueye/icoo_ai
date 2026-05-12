package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/services"
	"github.com/icoo-ai/icoo-ai/agent_gateway/pkg/httpx"
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
	engine := httpx.New()
	engine.Use(
		httpx.Recovery(),
		httpx.CORS(httpx.CORSConfig{}),
	)

	v1 := engine.Group("/v1")
	v1.GET("/agents", h.handleAgents)
	v1.GET("/skills", h.handleSkills)
	v1.GET("/management/settings", h.handleManagementSettings)
	v1.PUT("/management/settings", h.handleManagementSettingsUpdate)
	v1.GET("/sessions", h.handleSessions)
	v1.POST("/sessions", h.handleSessionCreate)
	v1.GET("/sessions/:sessionID", h.handleSessionGet)
	v1.DELETE("/sessions/:sessionID", h.handleSessionDelete)
	v1.GET("/sessions/:sessionID/messages", h.handleSessionMessages)
	v1.POST("/sessions/:sessionID/messages", h.handleSessionMessageCreate)
	v1.POST("/sessions/:sessionID/resume", h.handleSessionResume)
	v1.PUT("/sessions/:sessionID/mode", h.handleSessionMode)
	v1.PUT("/sessions/:sessionID/config", h.handleSessionConfig)
	v1.POST("/sessions/:sessionID/runs/cancel", h.handleSessionRunCancel)
	v1.POST("/sessions/:sessionID/prompt", h.handleSessionMessageCreate)
	v1.POST("/sessions/:sessionID/cancel", h.handleSessionRunCancel)
	v1.POST("/sessions/:sessionID/close", h.handleSessionClose)
	v1.GET("/runs", h.handleRuns)
	v1.GET("/approvals", h.handleApprovals)
	v1.PUT("/approvals/:approvalID/decision", h.handleApprovalAction)
	v1.POST("/approvals/:approvalID/decision", h.handleApprovalAction)
	v1.GET("/events/stream", h.handleEventStream)

	registerCRUDRoutes(v1, "/agent-configs", h.handleAgentConfigAction)
	registerCRUDRoutes(v1, "/channel-configs", h.handleChannelConfigAction)
	registerCRUDRoutes(v1, "/mcp-server-configs", h.handleMCPServerConfigAction)
	registerCRUDRoutes(v1, "/schedule-task-configs", h.handleScheduleTaskConfigAction)

	return engine
}

func registerCRUDRoutes(group *httpx.Group, path string, handler httpx.HandlerFunc) {
	group.POST(path+"/create", handler)
	group.PUT(path+"/update", handler)
	group.DELETE(path+"/delete", handler)
	group.GET(path+"/page", handler)
	group.GET(path+"/list", handler)
	group.GET(path+"/getById", handler)
	group.GET(path+"/status", handler)
}

func writeJSON(c *httpx.Context, status int, value any) {
	c.JSON(status, value)
}

func writeError(c *httpx.Context, status int, code, message string) {
	writeJSON(c, status, ErrorResponse{Code: code, Message: message})
}

func writeServiceError(c *httpx.Context, err error) {
	var gatewayErr *services.GatewayError
	if errors.As(err, &gatewayErr) {
		writeError(c, statusForServiceCode(gatewayErr.Code), gatewayErr.Code, gatewayErr.Message)
		return
	}
	writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
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

func decodeJSON(c *httpx.Context, dst any) error {
	return c.BindJSON(dst)
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
