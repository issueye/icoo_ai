package acp

import (
	"fmt"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type rpcRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      string         `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      string         `json:"id"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *rpcError      `json:"error,omitempty"`
}

func initializeParams(req models.ConnectorInitializeRequest) map[string]any {
	return map[string]any{
		"protocolVersion": 1,
		"clientInfo": map[string]any{
			"name":    req.ClientName,
			"version": req.ClientVersion,
		},
		"clientCapabilities": map[string]any{
			"fs": map[string]any{
				"readTextFile":  false,
				"writeTextFile": false,
			},
			"terminal": false,
		},
	}
}

func newSessionParams(req models.ConnectorNewSessionRequest) map[string]any {
	params := map[string]any{
		"cwd":        req.CWD,
		"mcpServers": []any{},
	}
	if len(req.Metadata) > 0 {
		params["_meta"] = cloneMap(req.Metadata)
		if rawAdditional, ok := req.Metadata["additional_directories"]; ok {
			if additional, ok := rawAdditional.([]string); ok {
				params["additionalDirectories"] = append([]string(nil), additional...)
			}
		}
	}
	return params
}

func promptParams(req models.ConnectorPromptRequest) map[string]any {
	params := map[string]any{
		"sessionId": req.SessionID,
		"prompt": []any{
			map[string]any{
				"type": "text",
				"text": req.Content,
			},
		},
	}
	if req.RequestID != "" {
		params["_meta"] = map[string]any{
			"requestId": req.RequestID,
		}
	}
	return params
}

func cancelParams(req models.ConnectorCancelRequest) map[string]any {
	return map[string]any{
		"sessionId": req.SessionID,
	}
}

func listSessionsParams(req models.ConnectorListSessionsRequest) map[string]any {
	params := map[string]any{}
	if req.CWD != "" {
		params["cwd"] = req.CWD
	}
	if len(req.AdditionalDirectories) > 0 {
		params["additionalDirectories"] = append([]string(nil), req.AdditionalDirectories...)
	}
	return params
}

func resumeSessionParams(req models.ConnectorResumeSessionRequest) map[string]any {
	params := map[string]any{
		"sessionId": req.SessionID,
		"cwd":       req.CWD,
	}
	if len(req.AdditionalDirectories) > 0 {
		params["additionalDirectories"] = append([]string(nil), req.AdditionalDirectories...)
	}
	return params
}

func closeSessionParams(req models.ConnectorCloseSessionRequest) map[string]any {
	return map[string]any{
		"sessionId": req.SessionID,
	}
}

func setSessionModeParams(req models.ConnectorSetSessionModeRequest) map[string]any {
	return map[string]any{
		"sessionId": req.SessionID,
		"modeId":    req.ModeID,
	}
}

func setSessionConfigOptionParams(req models.ConnectorSetSessionConfigOptionRequest) map[string]any {
	params := map[string]any{
		"sessionId": req.SessionID,
		"configId":  req.ConfigID,
	}
	if req.BooleanValue != nil {
		params["type"] = "boolean"
		params["value"] = *req.BooleanValue
		return params
	}
	params["value"] = req.ValueID
	return params
}

func mapInitializeResponse(result map[string]any) models.ConnectorInitializeResponse {
	serverName := stringField(result, "serverName")
	serverVersion := stringField(result, "serverVersion")
	agentInfo := mapField(result, "agentInfo")
	if serverName == "" {
		serverName = stringField(agentInfo, "name")
	}
	if serverVersion == "" {
		serverVersion = stringField(agentInfo, "version")
	}
	return models.ConnectorInitializeResponse{
		ServerName:    serverName,
		ServerVersion: serverVersion,
	}
}

func mapNewSessionResponse(result map[string]any) models.ConnectorNewSessionResponse {
	return models.ConnectorNewSessionResponse{
		SessionID: stringField(result, "sessionId"),
	}
}

func mapPromptResponse(result map[string]any) models.ConnectorPromptResponse {
	resp := models.ConnectorPromptResponse{
		RunID:  stringField(result, "runId"),
		Output: stringField(result, "output"),
	}
	if resp.RunID == "" {
		resp.RunID = stringField(result, "userMessageId")
	}
	if endedAt := stringField(result, "endedAt"); endedAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, endedAt); err == nil {
			resp.EndedAt = &parsed
		}
	} else if stopReason := stringField(result, "stopReason"); stopReason != "" {
		now := time.Now().UTC()
		resp.EndedAt = &now
	}
	rawApprovals, _ := result["approvals"].([]any)
	for _, raw := range rawApprovals {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		resp.Approvals = append(resp.Approvals, models.ConnectorApprovalRequest{
			RequestID: stringField(item, "requestId"),
			Action:    stringField(item, "action"),
			Message:   stringField(item, "message"),
		})
	}
	return resp
}

func mapCancelResponse(result map[string]any) models.ConnectorCancelResponse {
	resp := models.ConnectorCancelResponse{
		RunID:  stringField(result, "runId"),
		Status: stringField(result, "status"),
	}
	if resp.Status == "" {
		resp.Status = "cancelled"
	}
	return resp
}

func mapListSessionsResponse(result map[string]any) models.ConnectorListSessionsResponse {
	resp := models.ConnectorListSessionsResponse{}
	rawSessions, _ := result["sessions"].([]any)
	for _, raw := range rawSessions {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		resp.Sessions = append(resp.Sessions, models.ConnectorSessionInfo{
			SessionID:             stringField(item, "sessionId"),
			CWD:                   stringField(item, "cwd"),
			Title:                 stringField(item, "title"),
			AdditionalDirectories: stringSliceField(item, "additionalDirectories"),
		})
	}
	return resp
}

func mapResumeSessionResponse(_ map[string]any) models.ConnectorResumeSessionResponse {
	return models.ConnectorResumeSessionResponse{}
}

func mapCloseSessionResponse(_ map[string]any) models.ConnectorCloseSessionResponse {
	return models.ConnectorCloseSessionResponse{}
}

func mapSetSessionModeResponse(_ map[string]any) models.ConnectorSetSessionModeResponse {
	return models.ConnectorSetSessionModeResponse{}
}

func mapSetSessionConfigOptionResponse(_ map[string]any) models.ConnectorSetSessionConfigOptionResponse {
	return models.ConnectorSetSessionConfigOptionResponse{}
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func stringSliceField(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	raw, ok := m[key]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case []string:
		return append([]string(nil), value...)
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				continue
			}
			out = append(out, text)
		}
		return out
	default:
		return nil
	}
}

func mapSessionUpdateToEnvelope(eventID string, params map[string]any) (models.EventEnvelope, bool) {
	if params == nil {
		return models.EventEnvelope{}, false
	}
	sessionID := stringField(params, "sessionId")
	runID := stringField(params, "runId")

	if update := mapField(params, "update"); update != nil {
		updateType := stringField(update, "sessionUpdate")
		if updateType == "" {
			updateType = stringField(update, "type")
		}
		payload := update
		eventType := updateType
		switch updateType {
		case "agent_message_chunk":
			eventType = "message.created"
			payload = map[string]any{
				"role":    "assistant",
				"content": textFromContentBlock(mapField(update, "content")),
			}
		case "agent_thought_chunk":
			eventType = "run.updated"
			payload = map[string]any{
				"status":  "running",
				"thought": textFromContentBlock(mapField(update, "content")),
			}
		case "tool_call":
			eventType = "run.updated"
		case "tool_call_status":
			eventType = "run.updated"
		}
		if eventType == "" {
			eventType = "run.updated"
		}
		return models.EventEnvelope{
			BaseModel: models.BaseModel{ID: eventID},
			Type:      eventType,
			AgentID:   stringField(params, "agentId"),
			SessionID: sessionID,
			RunID:     runID,
			Payload:   payload,
			CreatedAt: parseEventCreatedAt(params),
		}, true
	}

	eventType := stringField(params, "type")
	if eventType == "" {
		eventType = "run.updated"
	}
	payload := mapField(params, "payload")
	if payload == nil {
		payload = cloneMap(params)
		delete(payload, "agentId")
		delete(payload, "sessionId")
		delete(payload, "runId")
		delete(payload, "type")
		delete(payload, "createdAt")
	}
	return models.EventEnvelope{
		BaseModel: models.BaseModel{ID: eventID},
		Type:      eventType,
		AgentID:   stringField(params, "agentId"),
		SessionID: sessionID,
		RunID:     runID,
		Payload:   payload,
		CreatedAt: parseEventCreatedAt(params),
	}, true
}

func parseEventCreatedAt(params map[string]any) time.Time {
	createdAtRaw := stringField(params, "createdAt")
	if createdAtRaw != "" {
		if ts, err := time.Parse(time.RFC3339Nano, createdAtRaw); err == nil {
			return ts.UTC()
		}
	}
	return time.Now().UTC()
}

func mapField(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	raw, ok := m[key]
	if !ok {
		return nil
	}
	out, ok := raw.(map[string]any)
	if ok {
		return out
	}
	return nil
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func textFromContentBlock(content map[string]any) string {
	if content == nil {
		return ""
	}
	if text := stringField(content, "text"); text != "" {
		return text
	}
	return ""
}

func nextEventID(n uint64) string {
	return fmt.Sprintf("evt_acp_%d", n)
}
