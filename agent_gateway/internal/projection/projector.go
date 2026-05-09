package projection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

const MaxSummaryChars = 500

type StoreWriter interface {
	AppendMessage(ctx context.Context, event store.MessageEvent) error
	UpsertRun(ctx context.Context, run store.RunSummary) error
	UpsertApproval(ctx context.Context, approval store.ApprovalDecision) error
}

type Result struct {
	Ignored  bool
	Message  store.MessageEvent
	Run      *store.RunSummary
	Approval *store.ApprovalDecision
}

func Build(envelope events.Envelope) Result {
	sessionID := strings.TrimSpace(envelope.SessionID)
	if sessionID == "" {
		return Result{Ignored: true}
	}

	payloadMap := payloadAsMap(envelope.Payload)
	status := mapString(payloadMap, "status")
	role := mapString(payloadMap, "role")

	createdAt := envelope.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	eventID := strings.TrimSpace(envelope.ID)
	if eventID == "" {
		eventID = buildFallbackID(envelope, createdAt)
	}

	summary := summarizeEnvelope(envelope.Type, payloadMap)
	safeMeta := buildSafeMeta(envelope.Type, envelope.Payload, payloadMap, status)

	message := store.MessageEvent{
		ID:        eventID,
		Type:      defaultString(strings.TrimSpace(envelope.Type), "event"),
		AgentID:   strings.TrimSpace(envelope.AgentID),
		SessionID: sessionID,
		RunID:     resolveRunID(envelope, payloadMap, eventID),
		Role:      role,
		Status:    status,
		Summary:   summary,
		SafeMeta:  safeMeta,
		CreatedAt: createdAt,
	}

	result := Result{
		Message: message,
	}
	if status == "" {
		approval := buildApproval(envelope, payloadMap, createdAt, status)
		if approval != nil {
			result.Approval = approval
		}
		return result
	}

	runID := message.RunID
	if runID == "" {
		return result
	}
	run := store.RunSummary{
		ID:        runID,
		AgentID:   message.AgentID,
		SessionID: sessionID,
		RunID:     runID,
		Status:    status,
		Summary:   summary,
		SafeMeta:  safeMeta,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	if isTerminalStatus(status) {
		terminalAt := createdAt
		run.CompletedAt = &terminalAt
	}
	result.Run = &run
	approval := buildApproval(envelope, payloadMap, createdAt, status)
	if approval != nil {
		result.Approval = approval
	}
	return result
}

func Apply(ctx context.Context, writer StoreWriter, envelope events.Envelope) (Result, error) {
	result := Build(envelope)
	if result.Ignored {
		return result, nil
	}
	if err := writer.AppendMessage(ctx, result.Message); err != nil {
		return result, err
	}
	if result.Run != nil {
		if err := writer.UpsertRun(ctx, *result.Run); err != nil {
			return result, err
		}
	}
	if result.Approval != nil {
		if err := writer.UpsertApproval(ctx, *result.Approval); err != nil {
			return result, err
		}
	}
	return result, nil
}

func buildApproval(envelope events.Envelope, payloadMap map[string]any, createdAt time.Time, messageStatus string) *store.ApprovalDecision {
	eventType := strings.ToLower(strings.TrimSpace(envelope.Type))
	if !strings.HasPrefix(eventType, "approval.") {
		return nil
	}
	approvalID := firstNonEmptyString(payloadMap, "approvalId", "id")
	connectorRequestID := firstNonEmptyString(payloadMap, "connectorRequestId", "requestId")
	if approvalID == "" || connectorRequestID == "" || strings.TrimSpace(envelope.SessionID) == "" || strings.TrimSpace(envelope.RunID) == "" {
		return nil
	}

	status := firstNonEmptyString(payloadMap, "status")
	if status == "" {
		status = statusFromApprovalType(eventType)
	}
	if status == "" {
		status = messageStatus
	}
	decision := firstNonEmptyString(payloadMap, "decision")
	if decision == "" {
		decision = decisionFromStatus(status)
	}

	approval := &store.ApprovalDecision{
		ID:                 approvalID,
		AgentID:            strings.TrimSpace(envelope.AgentID),
		SessionID:          strings.TrimSpace(envelope.SessionID),
		RunID:              strings.TrimSpace(envelope.RunID),
		ConnectorRequestID: connectorRequestID,
		Status:             status,
		Decision:           decision,
		Summary:            firstNonEmptyString(payloadMap, "message", "summary"),
		CreatedAt:          createdAt,
		UpdatedAt:          createdAt,
	}
	action := firstNonEmptyString(payloadMap, "action")
	if action != "" {
		approval.SafeMeta = store.SafeMeta{"action": action}
	}
	if isTerminalApprovalStatus(status) {
		terminalAt := createdAt
		approval.DecidedAt = &terminalAt
	}
	return approval
}

func summarizeEnvelope(eventType string, payload map[string]any) string {
	parts := make([]string, 0, 4)
	if trimmedType := strings.TrimSpace(eventType); trimmedType != "" {
		parts = append(parts, "type="+trimmedType)
	}
	if status := mapString(payload, "status"); status != "" {
		parts = append(parts, "status="+status)
	}
	if role := mapString(payload, "role"); role != "" {
		parts = append(parts, "role="+role)
	}
	preview := firstNonEmptyString(payload, "summary", "content", "message", "text", "output", "error", "reason")
	if preview != "" {
		parts = append(parts, "preview="+compactWhitespace(preview))
	}
	if len(parts) == 0 {
		parts = append(parts, "event")
	}
	return limitChars(strings.Join(parts, " "), MaxSummaryChars)
}

func buildSafeMeta(eventType string, payload any, payloadMap map[string]any, status string) store.SafeMeta {
	meta := store.SafeMeta{}
	if trimmedType := strings.TrimSpace(eventType); trimmedType != "" {
		meta["eventType"] = trimmedType
	}
	if status != "" {
		meta["status"] = status
	}
	if len(payloadMap) > 0 {
		meta["payloadKeys"] = sortedKeys(payloadMap)
	}
	if raw, err := safeJSONMarshal(payload); err == nil {
		hash := sha256.Sum256(raw)
		meta["payloadDigest"] = hex.EncodeToString(hash[:])
		meta["payloadBytes"] = len(raw)
	} else {
		meta["payloadError"] = err.Error()
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

func resolveRunID(envelope events.Envelope, payloadMap map[string]any, fallback string) string {
	if id := strings.TrimSpace(envelope.RunID); id != "" {
		return id
	}
	for _, key := range []string{"runId", "runID", "id"} {
		if v := mapString(payloadMap, key); v != "" {
			return v
		}
	}
	return fallback
}

func payloadAsMap(payload any) map[string]any {
	direct, ok := payload.(map[string]any)
	if ok {
		return direct
	}
	raw, err := safeJSONMarshal(payload)
	if err != nil {
		return nil
	}
	var mapped map[string]any
	if err := json.Unmarshal(raw, &mapped); err != nil {
		return nil
	}
	return mapped
}

func mapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	raw, ok := m[key]
	if !ok {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func firstNonEmptyString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := mapString(m, key); value != "" {
			return value
		}
	}
	return ""
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func limitChars(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func safeJSONMarshal(value any) (raw []byte, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("payload marshal panic: %v", recovered)
		}
	}()
	return json.Marshal(value)
}

func buildFallbackID(envelope events.Envelope, createdAt time.Time) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%d", envelope.Type, envelope.AgentID, envelope.SessionID, envelope.RunID, createdAt.UnixNano())
	sum := sha256.Sum256([]byte(raw))
	return "evt_proj_" + hex.EncodeToString(sum[:8])
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func isTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func statusFromApprovalType(eventType string) string {
	switch eventType {
	case "approval.requested", "approval.created":
		return "pending"
	case "approval.approved":
		return "approved"
	case "approval.rejected":
		return "rejected"
	case "approval.expired":
		return "expired"
	default:
		return ""
	}
}

func decisionFromStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "approved":
		return "approved"
	case "rejected", "expired":
		return "rejected"
	default:
		return ""
	}
}

func isTerminalApprovalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "approved", "rejected", "expired":
		return true
	default:
		return false
	}
}
