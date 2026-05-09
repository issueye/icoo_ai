package service

import (
	"context"
	"testing"
	"time"
)

func TestApprovalBrokerDecisionDoesNotCrossSession(t *testing.T) {
	svc := NewMockGatewayService()
	ctx := context.Background()

	sessionA, err := svc.CreateSession(ctx, CreateSessionRequest{Title: "A"})
	if err != nil {
		t.Fatalf("create session A: %v", err)
	}
	sessionB, err := svc.CreateSession(ctx, CreateSessionRequest{Title: "B"})
	if err != nil {
		t.Fatalf("create session B: %v", err)
	}

	promptA, err := svc.Prompt(ctx, sessionA.ID, PromptRequest{Content: "need approval a"})
	if err != nil {
		t.Fatalf("prompt A: %v", err)
	}
	promptB, err := svc.Prompt(ctx, sessionB.ID, PromptRequest{Content: "need approval b"})
	if err != nil {
		t.Fatalf("prompt B: %v", err)
	}
	if promptA.Approval == nil || promptB.Approval == nil {
		t.Fatal("expected approvals in both sessions")
	}

	approved, err := svc.DecideApproval(ctx, promptA.Approval.ID, ApprovalDecisionRequest{Decision: "approved"})
	if err != nil {
		t.Fatalf("decide approval A: %v", err)
	}
	if approved.Status != "approved" {
		t.Fatalf("expected approval A approved, got %q", approved.Status)
	}

	approvals, err := svc.ListApprovals(ctx)
	if err != nil {
		t.Fatalf("list approvals: %v", err)
	}

	statusByID := make(map[string]string, len(approvals))
	for _, approval := range approvals {
		statusByID[approval.ID] = approval.Status
	}

	if statusByID[promptA.Approval.ID] != "approved" {
		t.Fatalf("expected approval A approved, got %q", statusByID[promptA.Approval.ID])
	}
	if statusByID[promptB.Approval.ID] != "pending" {
		t.Fatalf("expected approval B pending, got %q", statusByID[promptB.Approval.ID])
	}
}

func TestApprovalBrokerCancelExpiresPendingApprovalInSession(t *testing.T) {
	svc := NewMockGatewayService()
	ctx := context.Background()

	sessionA, err := svc.CreateSession(ctx, CreateSessionRequest{Title: "A"})
	if err != nil {
		t.Fatalf("create session A: %v", err)
	}
	sessionB, err := svc.CreateSession(ctx, CreateSessionRequest{Title: "B"})
	if err != nil {
		t.Fatalf("create session B: %v", err)
	}

	promptA, err := svc.Prompt(ctx, sessionA.ID, PromptRequest{Content: "need approval a"})
	if err != nil {
		t.Fatalf("prompt A: %v", err)
	}
	promptB, err := svc.Prompt(ctx, sessionB.ID, PromptRequest{Content: "need approval b"})
	if err != nil {
		t.Fatalf("prompt B: %v", err)
	}
	if promptA.Approval == nil || promptB.Approval == nil {
		t.Fatal("expected approvals in both sessions")
	}

	if _, err := svc.Cancel(ctx, sessionA.ID); err != nil {
		t.Fatalf("cancel session A: %v", err)
	}

	approvals, err := svc.ListApprovals(ctx)
	if err != nil {
		t.Fatalf("list approvals: %v", err)
	}
	byID := make(map[string]Approval, len(approvals))
	for _, approval := range approvals {
		byID[approval.ID] = approval
	}

	expired := byID[promptA.Approval.ID]
	if expired.Status != "expired" || expired.Decision != "rejected" || expired.DecidedAt == nil {
		t.Fatalf("expected session A approval expired/rejected/decidedAt set, got %#v", expired)
	}
	stillPending := byID[promptB.Approval.ID]
	if stillPending.Status != "pending" {
		t.Fatalf("expected session B approval still pending, got %q", stillPending.Status)
	}

	_, err = svc.DecideApproval(ctx, promptA.Approval.ID, ApprovalDecisionRequest{Decision: "approved"})
	if err == nil {
		t.Fatal("expected deciding expired approval to fail")
	}
	serviceErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected service error, got %T: %v", err, err)
	}
	if serviceErr.Code != "invalid_decision" {
		t.Fatalf("expected invalid_decision, got %q (%s)", serviceErr.Code, serviceErr.Message)
	}
}

func TestApprovalBrokerDecideCleansIndexesOnTerminal(t *testing.T) {
	broker := NewApprovalBroker()
	now := time.Now().UTC()
	approval := Approval{
		ID:                 "approval-1",
		AgentID:            "agent-1",
		SessionID:          "session-1",
		RunID:              "run-1",
		ConnectorRequestID: "req-1",
		Status:             "pending",
	}
	approvals := map[string]Approval{approval.ID: approval}

	if err := broker.Register(approval); err != nil {
		t.Fatalf("register: %v", err)
	}
	routeKey := keyFromApproval(approval)

	updated, err := broker.Decide(approval.ID, ApprovalDecisionRequest{Decision: "approved"}, approvals, now)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if updated.Status != "approved" {
		t.Fatalf("expected approved status, got %q", updated.Status)
	}
	if _, ok := broker.byApprovalID[approval.ID]; ok {
		t.Fatalf("expected approval id index cleaned for %q", approval.ID)
	}
	if _, ok := broker.byRoute[routeKey]; ok {
		t.Fatalf("expected route index cleaned for %+v", routeKey)
	}
	if _, ok := broker.bySession[approval.SessionID]; ok {
		t.Fatalf("expected session index cleaned for %q", approval.SessionID)
	}

	_, err = broker.Decide(approval.ID, ApprovalDecisionRequest{Decision: "approved"}, approvals, now)
	if err == nil {
		t.Fatal("expected deciding terminal approval to fail")
	}
	serviceErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected service error, got %T: %v", err, err)
	}
	if serviceErr.Code != "invalid_decision" {
		t.Fatalf("expected invalid_decision, got %q (%s)", serviceErr.Code, serviceErr.Message)
	}
}

func TestApprovalBrokerExpirePendingBySessionCleansIndexes(t *testing.T) {
	broker := NewApprovalBroker()
	now := time.Now().UTC()
	approval := Approval{
		ID:                 "approval-expire-1",
		AgentID:            "agent-1",
		SessionID:          "session-expire-1",
		RunID:              "run-1",
		ConnectorRequestID: "req-1",
		Status:             "pending",
	}
	approvals := map[string]Approval{approval.ID: approval}

	if err := broker.Register(approval); err != nil {
		t.Fatalf("register: %v", err)
	}
	routeKey := keyFromApproval(approval)

	updated := broker.ExpirePendingBySession(approval.SessionID, approvals, now)
	if updated != 1 {
		t.Fatalf("expected 1 expired approval, got %d", updated)
	}
	expired := approvals[approval.ID]
	if expired.Status != "expired" || expired.Decision != "rejected" || expired.DecidedAt == nil {
		t.Fatalf("expected expired/rejected/decidedAt set, got %#v", expired)
	}
	if _, ok := broker.byApprovalID[approval.ID]; ok {
		t.Fatalf("expected approval id index cleaned for %q", approval.ID)
	}
	if _, ok := broker.byRoute[routeKey]; ok {
		t.Fatalf("expected route index cleaned for %+v", routeKey)
	}
	if _, ok := broker.bySession[approval.SessionID]; ok {
		t.Fatalf("expected session index cleaned for %q", approval.SessionID)
	}

	_, err := broker.Decide(approval.ID, ApprovalDecisionRequest{Decision: "approved"}, approvals, now)
	if err == nil {
		t.Fatal("expected deciding expired approval to fail")
	}
	serviceErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected service error, got %T: %v", err, err)
	}
	if serviceErr.Code != "invalid_decision" {
		t.Fatalf("expected invalid_decision, got %q (%s)", serviceErr.Code, serviceErr.Message)
	}
}
