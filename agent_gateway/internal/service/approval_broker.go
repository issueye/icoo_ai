package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type approvalRouteKey struct {
	agentID            string
	sessionID          string
	runID              string
	connectorRequestID string
}

type ApprovalBroker struct {
	mu           sync.Mutex
	byApprovalID map[string]approvalRouteKey
	byRoute      map[approvalRouteKey]string
	bySession    map[string]map[string]struct{}
}

func NewApprovalBroker() *ApprovalBroker {
	return &ApprovalBroker{
		byApprovalID: make(map[string]approvalRouteKey),
		byRoute:      make(map[approvalRouteKey]string),
		bySession:    make(map[string]map[string]struct{}),
	}
}

func (b *ApprovalBroker) Register(approval models.Approval) error {
	key := keyFromApproval(approval)
	if err := validateApprovalRoute(approval.ID, key); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if existingKey, ok := b.byApprovalID[approval.ID]; ok {
		if existingKey != key {
			return NewError("invalid_decision", fmt.Sprintf("approval %q route changed unexpectedly", approval.ID))
		}
		return nil
	}
	if existingApprovalID, ok := b.byRoute[key]; ok && existingApprovalID != approval.ID {
		return NewError("invalid_decision", "approval route already bound to a different approval")
	}

	b.byApprovalID[approval.ID] = key
	b.byRoute[key] = approval.ID
	if _, ok := b.bySession[key.sessionID]; !ok {
		b.bySession[key.sessionID] = make(map[string]struct{})
	}
	b.bySession[key.sessionID][approval.ID] = struct{}{}
	return nil
}

func (b *ApprovalBroker) Decide(approvalID string, req models.ApprovalDecisionRequest, approvals map[string]models.Approval, now time.Time) (models.Approval, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key, ok := b.byApprovalID[approvalID]
	if !ok {
		if approval, exists := approvals[approvalID]; exists && approval.Status != "pending" {
			return models.Approval{}, NewError("invalid_decision", fmt.Sprintf("approval %q is already %s", approvalID, approval.Status))
		}
		return models.Approval{}, NewError("approval_not_found", fmt.Sprintf("approval %q was not found", approvalID))
	}

	approval, ok := approvals[approvalID]
	if !ok {
		return models.Approval{}, NewError("approval_not_found", fmt.Sprintf("approval %q was not found", approvalID))
	}
	if keyFromApproval(approval) != key {
		return models.Approval{}, NewError("approval_not_found", fmt.Sprintf("approval %q was not found", approvalID))
	}
	if approval.Status != "pending" {
		return models.Approval{}, NewError("invalid_decision", fmt.Sprintf("approval %q is already %s", approvalID, approval.Status))
	}

	decision, err := normalizeDecision(req.Decision)
	if err != nil {
		return models.Approval{}, err
	}
	approval.Status = decision
	approval.Decision = decision
	approval.DecidedAt = &now
	if strings.TrimSpace(req.Message) != "" {
		approval.Message = req.Message
	}
	approvals[approvalID] = approval
	b.cleanupLocked(approvalID, key)
	return approval, nil
}

func (b *ApprovalBroker) ExpirePendingBySession(sessionID string, approvals map[string]models.Approval, now time.Time) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	approvalIDs, ok := b.bySession[sessionID]
	if !ok {
		return 0
	}

	updated := 0
	for approvalID := range approvalIDs {
		key, ok := b.byApprovalID[approvalID]
		if !ok {
			continue
		}
		approval, ok := approvals[approvalID]
		if !ok {
			b.cleanupLocked(approvalID, key)
			continue
		}
		if approval.Status != "pending" {
			b.cleanupLocked(approvalID, key)
			continue
		}
		approval.Status = "expired"
		approval.Decision = "rejected"
		approval.DecidedAt = &now
		approval.Message = "Approval expired because session was cancelled"
		approvals[approvalID] = approval
		b.cleanupLocked(approvalID, key)
		updated++
	}
	return updated
}

func (b *ApprovalBroker) cleanupLocked(approvalID string, key approvalRouteKey) {
	delete(b.byApprovalID, approvalID)
	delete(b.byRoute, key)
	if sessionApprovals, ok := b.bySession[key.sessionID]; ok {
		delete(sessionApprovals, approvalID)
		if len(sessionApprovals) == 0 {
			delete(b.bySession, key.sessionID)
		}
	}
}

func validateApprovalRoute(approvalID string, key approvalRouteKey) error {
	if approvalID == "" || key.agentID == "" || key.sessionID == "" || key.runID == "" || key.connectorRequestID == "" {
		return NewError("invalid_decision", "approval route requires agentId/sessionId/runId/connectorRequestId")
	}
	return nil
}

func keyFromApproval(approval models.Approval) approvalRouteKey {
	return approvalRouteKey{
		agentID:            approval.AgentID,
		sessionID:          approval.SessionID,
		runID:              approval.RunID,
		connectorRequestID: approval.ConnectorRequestID,
	}
}

func normalizeDecision(raw string) (string, error) {
	decision := strings.ToLower(strings.TrimSpace(raw))
	switch decision {
	case "approved", "rejected":
		return decision, nil
	case "allow":
		return "approved", nil
	case "deny":
		return "rejected", nil
	default:
		return "", NewError("invalid_decision", "decision must be approved, rejected, allow, or deny")
	}
}
