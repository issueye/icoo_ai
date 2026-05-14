package acp

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/google/uuid"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

const (
	ApprovalStatusPending   = "pending"
	ApprovalStatusSelected  = "selected"
	ApprovalStatusCancelled = "cancelled"
)

var (
	ErrApprovalNotFound = errors.New("approval not found")
	ErrApprovalClosed   = errors.New("approval is already decided")
	ErrInvalidOption    = errors.New("approval option is invalid")
)

type ApprovalBroker struct {
	mu      sync.RWMutex
	events  *events.Bus
	records map[string]*approvalEntry
}

type ApprovalRecord struct {
	ID        string                    `json:"id"`
	AgentID   string                    `json:"agentId"`
	SessionID string                    `json:"sessionId"`
	ToolCall  acpsdk.ToolCallUpdate     `json:"toolCall"`
	Options   []acpsdk.PermissionOption `json:"options"`
	Status    string                    `json:"status"`
	OptionID  string                    `json:"optionId,omitempty"`
	Message   string                    `json:"message,omitempty"`
	CreatedAt time.Time                 `json:"createdAt"`
	DecidedAt *time.Time                `json:"decidedAt,omitempty"`
}

type ApprovalDecision struct {
	OptionID  string `json:"optionId,omitempty"`
	Decision  string `json:"decision,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
	Message   string `json:"message,omitempty"`
}

type approvalEntry struct {
	record ApprovalRecord
	done   chan ApprovalRecord
}

func NewApprovalBroker(bus *events.Bus) *ApprovalBroker {
	return &ApprovalBroker{
		events:  bus,
		records: make(map[string]*approvalEntry),
	}
}

func (b *ApprovalBroker) Request(ctx context.Context, agentID string, req acpsdk.RequestPermissionRequest) (acpsdk.RequestPermissionResponse, error) {
	if b == nil {
		return acpsdk.RequestPermissionResponse{}, acpsdk.NewInternalError(map[string]any{"error": "approval broker is not connected"})
	}
	record := ApprovalRecord{
		ID:        uuid.NewString(),
		AgentID:   agentID,
		SessionID: string(req.SessionId),
		ToolCall:  req.ToolCall,
		Options:   append([]acpsdk.PermissionOption(nil), req.Options...),
		Status:    ApprovalStatusPending,
		CreatedAt: time.Now(),
	}
	entry := &approvalEntry{record: record, done: make(chan ApprovalRecord, 1)}

	b.mu.Lock()
	b.records[record.ID] = entry
	b.mu.Unlock()
	b.publish("approval.requested", record)

	select {
	case <-ctx.Done():
		_, _ = b.Decide(record.ID, ApprovalDecision{Cancelled: true, Message: ctx.Err().Error()})
		return acpsdk.RequestPermissionResponse{}, acpsdk.NewRequestCancelled(map[string]any{"approvalId": record.ID})
	case decided := <-entry.done:
		return approvalResponse(decided), nil
	}
}

func (b *ApprovalBroker) List() []ApprovalRecord {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]ApprovalRecord, 0, len(b.records))
	for _, entry := range b.records {
		out = append(out, entry.record)
	}
	return out
}

func (b *ApprovalBroker) Get(id string) (ApprovalRecord, bool) {
	if b == nil {
		return ApprovalRecord{}, false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	entry := b.records[strings.TrimSpace(id)]
	if entry == nil {
		return ApprovalRecord{}, false
	}
	return entry.record, true
}

func (b *ApprovalBroker) Decide(id string, decision ApprovalDecision) (ApprovalRecord, error) {
	if b == nil {
		return ApprovalRecord{}, ErrApprovalNotFound
	}
	id = strings.TrimSpace(id)
	b.mu.Lock()
	entry := b.records[id]
	if entry == nil {
		b.mu.Unlock()
		return ApprovalRecord{}, ErrApprovalNotFound
	}
	if entry.record.Status != ApprovalStatusPending {
		record := entry.record
		b.mu.Unlock()
		return record, ErrApprovalClosed
	}
	now := time.Now()
	entry.record.DecidedAt = &now
	entry.record.Message = strings.TrimSpace(decision.Message)
	if decision.Cancelled {
		entry.record.Status = ApprovalStatusCancelled
	} else {
		optionID := strings.TrimSpace(decision.OptionID)
		if optionID == "" {
			optionID = optionIDFromDecision(entry.record.Options, decision.Decision)
		}
		if !hasOption(entry.record.Options, optionID) {
			b.mu.Unlock()
			return ApprovalRecord{}, ErrInvalidOption
		}
		entry.record.Status = ApprovalStatusSelected
		entry.record.OptionID = optionID
	}
	record := entry.record
	entry.done <- record
	b.mu.Unlock()

	b.publish("approval.decided", record)
	return record, nil
}

func optionIDFromDecision(options []acpsdk.PermissionOption, decision string) string {
	decision = strings.ToLower(strings.TrimSpace(decision))
	if decision == "" {
		return ""
	}
	var preferred []acpsdk.PermissionOptionKind
	switch decision {
	case "approved", "approved_once", "allow", "allow_once", "selected":
		preferred = []acpsdk.PermissionOptionKind{acpsdk.PermissionOptionKindAllowOnce, acpsdk.PermissionOptionKindAllowAlways}
	case "always_approved", "allow_always":
		preferred = []acpsdk.PermissionOptionKind{acpsdk.PermissionOptionKindAllowAlways, acpsdk.PermissionOptionKindAllowOnce}
	case "rejected", "denied", "reject", "reject_once":
		preferred = []acpsdk.PermissionOptionKind{acpsdk.PermissionOptionKindRejectOnce, acpsdk.PermissionOptionKindRejectAlways}
	case "reject_always":
		preferred = []acpsdk.PermissionOptionKind{acpsdk.PermissionOptionKindRejectAlways, acpsdk.PermissionOptionKindRejectOnce}
	default:
		return decision
	}
	for _, kind := range preferred {
		for _, option := range options {
			if option.Kind == kind {
				return string(option.OptionId)
			}
		}
	}
	return ""
}

func hasOption(options []acpsdk.PermissionOption, optionID string) bool {
	if optionID == "" {
		return false
	}
	for _, option := range options {
		if string(option.OptionId) == optionID {
			return true
		}
	}
	return false
}

func (b *ApprovalBroker) publish(eventType string, record ApprovalRecord) {
	if b == nil || b.events == nil {
		return
	}
	b.events.Publish(models.EventEnvelope{
		BaseModel: models.BaseModel{ID: uuid.NewString()},
		Type:      eventType,
		AgentID:   record.AgentID,
		SessionID: record.SessionID,
		Payload:   record,
		CreatedAt: time.Now(),
	})
}

func approvalResponse(record ApprovalRecord) acpsdk.RequestPermissionResponse {
	if record.Status == ApprovalStatusCancelled {
		return acpsdk.RequestPermissionResponse{
			Outcome: acpsdk.RequestPermissionOutcome{
				Cancelled: &acpsdk.RequestPermissionOutcomeCancelled{},
			},
		}
	}
	return acpsdk.RequestPermissionResponse{
		Outcome: acpsdk.RequestPermissionOutcome{
			Selected: &acpsdk.RequestPermissionOutcomeSelected{
				OptionId: acpsdk.PermissionOptionId(record.OptionID),
			},
		},
	}
}
