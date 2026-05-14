package acp

import (
	"context"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
)

func TestApprovalBrokerWaitsForDecision(t *testing.T) {
	bus := events.NewBus(8)
	broker := NewApprovalBroker(bus)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result := make(chan acpsdk.RequestPermissionResponse, 1)
	errs := make(chan error, 1)
	go func() {
		resp, err := broker.Request(ctx, "agent-1", acpsdk.RequestPermissionRequest{
			SessionId: acpsdk.SessionId("session-1"),
			Options: []acpsdk.PermissionOption{
				{Name: "Allow", OptionId: acpsdk.PermissionOptionId("allow"), Kind: acpsdk.PermissionOptionKindAllowOnce},
			},
			ToolCall: acpsdk.ToolCallUpdate{ToolCallId: acpsdk.ToolCallId("tool-1")},
		})
		result <- resp
		errs <- err
	}()

	var approval ApprovalRecord
	deadline := time.After(time.Second)
	for {
		records := broker.List()
		if len(records) == 1 {
			approval = records[0]
			break
		}
		select {
		case <-deadline:
			t.Fatal("approval was not created")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	if _, err := broker.Decide(approval.ID, ApprovalDecision{OptionID: "allow"}); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	if err := <-errs; err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	resp := <-result
	if resp.Outcome.Selected == nil || resp.Outcome.Selected.OptionId != "allow" {
		t.Fatalf("response = %#v, want selected allow", resp)
	}
}

func TestApprovalBrokerMapsLegacyDecisionToOption(t *testing.T) {
	broker := NewApprovalBroker(nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	errs := make(chan error, 1)
	responses := make(chan acpsdk.RequestPermissionResponse, 1)
	go func() {
		resp, err := broker.Request(ctx, "agent-1", acpsdk.RequestPermissionRequest{
			SessionId: acpsdk.SessionId("session-1"),
			Options: []acpsdk.PermissionOption{
				{Name: "Allow once", OptionId: acpsdk.PermissionOptionId("opt-allow"), Kind: acpsdk.PermissionOptionKindAllowOnce},
				{Name: "Reject", OptionId: acpsdk.PermissionOptionId("opt-reject"), Kind: acpsdk.PermissionOptionKindRejectOnce},
			},
			ToolCall: acpsdk.ToolCallUpdate{ToolCallId: acpsdk.ToolCallId("tool-1")},
		})
		responses <- resp
		errs <- err
	}()

	var id string
	deadline := time.After(time.Second)
	for id == "" {
		records := broker.List()
		if len(records) == 1 {
			id = records[0].ID
			break
		}
		select {
		case <-deadline:
			t.Fatal("approval was not created")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if _, err := broker.Decide(id, ApprovalDecision{Decision: "approved_once"}); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if err := <-errs; err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	resp := <-responses
	if resp.Outcome.Selected == nil || resp.Outcome.Selected.OptionId != "opt-allow" {
		t.Fatalf("response = %#v, want opt-allow", resp)
	}
}
