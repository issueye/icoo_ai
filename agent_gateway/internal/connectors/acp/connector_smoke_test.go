package acp

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
)

const (
	acpSmokeEnabledEnv = "ACP_SMOKE_TEST"
	acpSmokeCommandEnv = "ACP_SMOKE_COMMAND"
	acpSmokeArgsEnv    = "ACP_SMOKE_ARGS"
	acpSmokeModelEnv   = "ACP_SMOKE_MODEL"
	acpSmokeTimeoutEnv = "ACP_SMOKE_TIMEOUT"
)

func TestRealProcessSmoke(t *testing.T) {
	if os.Getenv(acpSmokeEnabledEnv) != "1" {
		t.Skipf("set %s=1 to enable real acp smoke test", acpSmokeEnabledEnv)
	}

	command := strings.TrimSpace(os.Getenv(acpSmokeCommandEnv))
	if command == "" {
		command = "icoo-ai"
	}

	argsRaw := strings.TrimSpace(os.Getenv(acpSmokeArgsEnv))
	if argsRaw == "" {
		argsRaw = "serve"
	}
	args := strings.Fields(argsRaw)

	timeout := 30 * time.Second
	if timeoutRaw := strings.TrimSpace(os.Getenv(acpSmokeTimeoutEnv)); timeoutRaw != "" {
		parsed, err := time.ParseDuration(timeoutRaw)
		if err != nil {
			t.Fatalf("invalid %s: %v", acpSmokeTimeoutEnv, err)
		}
		timeout = parsed
	}

	c, err := New(Options{
		Command: command,
		Args:    args,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	initResp, err := c.Initialize(ctx, connector.InitializeRequest{
		ClientName:    "agent-gateway-smoke",
		ClientVersion: "test",
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if strings.TrimSpace(initResp.ServerName) == "" {
		t.Fatalf("Initialize() server name is empty: %#v", initResp)
	}

	sessionResp, err := c.NewSession(ctx, connector.NewSessionRequest{
		AgentID: "icoo-ai-acp",
		Model:   strings.TrimSpace(os.Getenv(acpSmokeModelEnv)),
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if strings.TrimSpace(sessionResp.SessionID) == "" {
		t.Fatalf("NewSession() session id is empty: %#v", sessionResp)
	}

	promptResp, err := c.Prompt(ctx, connector.PromptRequest{
		SessionID: sessionResp.SessionID,
		RequestID: "smoke_req_1",
		Content:   "smoke test: reply with one short line",
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if strings.TrimSpace(promptResp.RunID) == "" {
		t.Fatalf("Prompt() run id is empty: %#v", promptResp)
	}

	cancelResp, err := c.Cancel(ctx, connector.CancelRequest{
		SessionID: sessionResp.SessionID,
		RunID:     promptResp.RunID,
		Reason:    "smoke_test",
	})
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if strings.TrimSpace(cancelResp.RunID) == "" {
		t.Fatalf("Cancel() run id is empty: %#v", cancelResp)
	}
}
