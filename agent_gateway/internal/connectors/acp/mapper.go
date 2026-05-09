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
		"clientName":    req.ClientName,
		"clientVersion": req.ClientVersion,
	}
}

func newSessionParams(req connector.NewSessionRequest) map[string]any {
	return map[string]any{
		"agentId":  req.AgentID,
		"model":    req.Model,
		"cwd":      req.CWD,
		"metadata": req.Metadata,
	}
}

func promptParams(req connector.PromptRequest) map[string]any {
	return map[string]any{
		"sessionId": req.SessionID,
		"content":   req.Content,
		"requestId": req.RequestID,
	}
}

func cancelParams(req connector.CancelRequest) map[string]any {
	return map[string]any{
		"sessionId": req.SessionID,
		"runId":     req.RunID,
		"reason":    req.Reason,
	}
}

func mapInitializeResponse(result map[string]any) connector.InitializeResponse {
	return connector.InitializeResponse{
		ServerName:    stringField(result, "serverName"),
		ServerVersion: stringField(result, "serverVersion"),
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
	if endedAt := stringField(result, "endedAt"); endedAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, endedAt); err == nil {
			resp.EndedAt = &parsed
		}
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
	return connector.CancelResponse{
		RunID:  stringField(result, "runId"),
		Status: stringField(result, "status"),
	}
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
		SessionID: stringField(params, "sessionId"),
		RunID:     stringField(params, "runId"),
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

func nextEventID(n uint64) string {
	return fmt.Sprintf("evt_acp_%d", n)
}
