package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type approvalBrokerFakeConnector struct {
	mu sync.Mutex
	n  int
}

func (f *approvalBrokerFakeConnector) Initialize(context.Context, connector.InitializeRequest) (connector.InitializeResponse, error) {
	return connector.InitializeResponse{}, nil
}

func (f *approvalBrokerFakeConnector) NewSession(context.Context, connector.NewSessionRequest) (connector.NewSessionResponse, error) {
	return connector.NewSessionResponse{}, nil
}

func (f *approvalBrokerFakeConnector) Prompt(context.Context, connector.PromptRequest) (connector.PromptResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	now := time.Now().UTC()
	return connector.PromptResponse{
		RunID:   fmt.Sprintf("run_approval_broker_%d", f.n),
		Output:  "ok",
		EndedAt: &now,
		Approvals: []connector.ApprovalRequest{
			{
				RequestID: fmt.Sprintf("req_approval_broker_%d", f.n),
				Action:    "write_file",
				Message:   "needs approval",
			},
		},
	}, nil
}

func (f *approvalBrokerFakeConnector) Cancel(context.Context, connector.CancelRequest) (connector.CancelResponse, error) {
	return connector.CancelResponse{Status: "cancelled"}, nil
}

func (f *approvalBrokerFakeConnector) Close() error {
	return nil
}

func newApprovalBrokerPromptService() *MockGatewayService {
	return NewConnectorGatewayServiceWithAgentsAndStore(defaultAgents(), store.NewMemoryStore(), &approvalBrokerFakeConnector{})
}

func TestApprovalBrokerDecisionDoesNotCrossSession(t *testing.T) {
	svc := newApprovalBrokerPromptService()
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
	svc := newApprovalBrokerPromptService()
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

func TestApprovalBrokerConcurrentSessionIsolation(t *testing.T) {
	svc := newApprovalBrokerPromptService()
	ctx := context.Background()

	sessionA, err := svc.CreateSession(ctx, CreateSessionRequest{Title: "A"})
	if err != nil {
		t.Fatalf("create session A: %v", err)
	}
	sessionB, err := svc.CreateSession(ctx, CreateSessionRequest{Title: "B"})
	if err != nil {
		t.Fatalf("create session B: %v", err)
	}

	var (
		promptA PromptResponse
		promptB PromptResponse
		errA    error
		errB    error
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		promptA, errA = svc.Prompt(ctx, sessionA.ID, PromptRequest{Content: "need approval a"})
	}()
	go func() {
		defer wg.Done()
		promptB, errB = svc.Prompt(ctx, sessionB.ID, PromptRequest{Content: "need approval b"})
	}()
	wg.Wait()

	if errA != nil {
		t.Fatalf("prompt A: %v", errA)
	}
	if errB != nil {
		t.Fatalf("prompt B: %v", errB)
	}
	if promptA.Approval == nil || promptB.Approval == nil {
		t.Fatal("expected approvals in both sessions")
	}

	var (
		cancelErr error
		decideErr error
		approved  Approval
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, cancelErr = svc.Cancel(ctx, sessionA.ID)
	}()
	go func() {
		defer wg.Done()
		approved, decideErr = svc.DecideApproval(ctx, promptB.Approval.ID, ApprovalDecisionRequest{Decision: "approved"})
	}()
	wg.Wait()

	if cancelErr != nil {
		t.Fatalf("cancel A: %v", cancelErr)
	}
	if decideErr != nil {
		t.Fatalf("decide B: %v", decideErr)
	}
	if approved.SessionID != sessionB.ID || approved.Status != "approved" {
		t.Fatalf("expected approved decision on session B, got %#v", approved)
	}

	approvals, err := svc.ListApprovals(ctx)
	if err != nil {
		t.Fatalf("list approvals: %v", err)
	}
	byID := make(map[string]Approval, len(approvals))
	for _, approval := range approvals {
		byID[approval.ID] = approval
	}

	expiredA := byID[promptA.Approval.ID]
	if expiredA.SessionID != sessionA.ID || expiredA.Status != "expired" || expiredA.Decision != "rejected" || expiredA.DecidedAt == nil {
		t.Fatalf("expected session A approval expired/rejected/decidedAt set, got %#v", expiredA)
	}
	approvedB := byID[promptB.Approval.ID]
	if approvedB.SessionID != sessionB.ID || approvedB.Status != "approved" || approvedB.Decision != "approved" || approvedB.DecidedAt == nil {
		t.Fatalf("expected session B approval approved/decidedAt set, got %#v", approvedB)
	}
}

func TestApprovalBrokerConcurrentSessionIsolationRepeated(t *testing.T) {
	const rounds = 20
	for i := 0; i < rounds; i++ {
		svc := newApprovalBrokerPromptService()
		ctx := context.Background()

		sessionA, err := svc.CreateSession(ctx, CreateSessionRequest{Title: "A"})
		if err != nil {
			t.Fatalf("round %d create session A: %v", i, err)
		}
		sessionB, err := svc.CreateSession(ctx, CreateSessionRequest{Title: "B"})
		if err != nil {
			t.Fatalf("round %d create session B: %v", i, err)
		}

		var (
			promptA PromptResponse
			promptB PromptResponse
			errA    error
			errB    error
		)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			promptA, errA = svc.Prompt(ctx, sessionA.ID, PromptRequest{Content: "need approval a"})
		}()
		go func() {
			defer wg.Done()
			promptB, errB = svc.Prompt(ctx, sessionB.ID, PromptRequest{Content: "need approval b"})
		}()
		wg.Wait()
		if errA != nil || errB != nil {
			t.Fatalf("round %d prompt errors: A=%v B=%v", i, errA, errB)
		}
		if promptA.Approval == nil || promptB.Approval == nil {
			t.Fatalf("round %d expected approvals in both sessions", i)
		}

		var cancelErr, decideErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, cancelErr = svc.Cancel(ctx, sessionA.ID)
		}()
		go func() {
			defer wg.Done()
			_, decideErr = svc.DecideApproval(ctx, promptB.Approval.ID, ApprovalDecisionRequest{Decision: "approved"})
		}()
		wg.Wait()
		if cancelErr != nil || decideErr != nil {
			t.Fatalf("round %d cancel/decide errors: cancel=%v decide=%v", i, cancelErr, decideErr)
		}

		approvals, err := svc.ListApprovals(ctx)
		if err != nil {
			t.Fatalf("round %d list approvals: %v", i, err)
		}
		byID := make(map[string]Approval, len(approvals))
		for _, approval := range approvals {
			byID[approval.ID] = approval
		}
		if got := byID[promptA.Approval.ID]; got.SessionID != sessionA.ID || got.Status != "expired" || got.Decision != "rejected" {
			t.Fatalf("round %d expected A expired/rejected, got %#v", i, got)
		}
		if got := byID[promptB.Approval.ID]; got.SessionID != sessionB.ID || got.Status != "approved" || got.Decision != "approved" {
			t.Fatalf("round %d expected B approved, got %#v", i, got)
		}
	}
}

func TestApprovalBrokerRouteIsolationByAgentAndConnectorProfile(t *testing.T) {
	broker := NewApprovalBroker()
	now := time.Now().UTC()
	approvals := map[string]Approval{
		"approval-agent-a": {
			ID:                 "approval-agent-a",
			AgentID:            "agent-a",
			SessionID:          "sess-shared",
			RunID:              "run-shared",
			ConnectorRequestID: "connector-profile-1",
			Status:             "pending",
		},
		"approval-agent-b": {
			ID:                 "approval-agent-b",
			AgentID:            "agent-b",
			SessionID:          "sess-shared",
			RunID:              "run-shared",
			ConnectorRequestID: "connector-profile-1",
			Status:             "pending",
		},
		"approval-connector-2": {
			ID:                 "approval-connector-2",
			AgentID:            "agent-a",
			SessionID:          "sess-shared",
			RunID:              "run-shared",
			ConnectorRequestID: "connector-profile-2",
			Status:             "pending",
		},
	}

	for _, id := range []string{"approval-agent-a", "approval-agent-b", "approval-connector-2"} {
		if err := broker.Register(approvals[id]); err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		updated, err := broker.Decide("approval-agent-a", ApprovalDecisionRequest{Decision: "approved"}, approvals, now)
		if err != nil {
			errs <- err
			return
		}
		if updated.AgentID != "agent-a" || updated.ConnectorRequestID != "connector-profile-1" || updated.Status != "approved" {
			errs <- NewError("invalid_decision", "agent-a approval identity/status changed unexpectedly")
		}
	}()
	go func() {
		defer wg.Done()
		updated, err := broker.Decide("approval-agent-b", ApprovalDecisionRequest{Decision: "rejected"}, approvals, now)
		if err != nil {
			errs <- err
			return
		}
		if updated.AgentID != "agent-b" || updated.ConnectorRequestID != "connector-profile-1" || updated.Status != "rejected" {
			errs <- NewError("invalid_decision", "agent-b approval identity/status changed unexpectedly")
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent decide failed: %v", err)
	}

	if got := approvals["approval-agent-a"]; got.Status != "approved" || got.Decision != "approved" {
		t.Fatalf("expected approval-agent-a approved, got %#v", got)
	}
	if got := approvals["approval-agent-b"]; got.Status != "rejected" || got.Decision != "rejected" {
		t.Fatalf("expected approval-agent-b rejected, got %#v", got)
	}
	if got := approvals["approval-connector-2"]; got.Status != "pending" || got.Decision != "" {
		t.Fatalf("expected approval-connector-2 still pending, got %#v", got)
	}
}
