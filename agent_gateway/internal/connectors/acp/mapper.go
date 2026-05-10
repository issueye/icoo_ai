package acp

import (
	"fmt"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
)

type rpcRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      string         `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
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

func initializeParams(req connector.InitializeRequest) map[string]any {
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

func newSessionParams(req connector.NewSessionRequest) map[string]any {
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

func promptParams(req connector.PromptRequest) map[string]any {
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

func cancelParams(req connector.CancelRequest) map[string]any {
	return map[string]any{
		"sessionId": req.SessionID,
	}
}

func mapInitializeResponse(result map[string]any) connector.InitializeResponse {
	serverName := stringField(result, "serverName")
	serverVersion := stringField(result, "serverVersion")
	agentInfo := mapField(result, "agentInfo")
	if serverName == "" {
		serverName = stringField(agentInfo, "name")
	}
	if serverVersion == "" {
		serverVersion = stringField(agentInfo, "version")
	}
	return connector.InitializeResponse{
		ServerName:    serverName,
		ServerVersion: serverVersion,
	}
}

func mapNewSessionResponse(result map[string]any) connector.NewSessionResponse {
	return connector.NewSessionResponse{
		SessionID: stringField(result, "sessionId"),
	}
}

func mapPromptResponse(result map[string]any) connector.PromptResponse {
	resp := connector.PromptResponse{
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
		resp.Approvals = append(resp.Approvals, connector.ApprovalRequest{
			RequestID: stringField(item, "requestId"),
			Action:    stringField(item, "action"),
			Message:   stringField(item, "message"),
		})
	}
	return resp
}

func mapCancelResponse(result map[string]any) connector.CancelResponse {
	resp := connector.CancelResponse{
		RunID:  stringField(result, "runId"),
		Status: stringField(result, "status"),
	}
	if resp.Status == "" {
		resp.Status = "cancelled"
	}
	return resp
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func mapSessionUpdateToEnvelope(eventID string, params map[string]any) (events.Envelope, bool) {
	if params == nil {
		return events.Envelope{}, false
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
		return events.Envelope{
			ID:        eventID,
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
	return events.Envelope{
		ID:        eventID,
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
