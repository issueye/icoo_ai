package acp

import (
	"context"
	"errors"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/scheduler"
)

type SchedulerRunner struct {
	manager *Manager
}

func NewSchedulerRunner(manager *Manager) *SchedulerRunner {
	return &SchedulerRunner{manager: manager}
}

func (r *SchedulerRunner) RunAgentPrompt(ctx context.Context, payload scheduler.Payload) error {
	if r == nil || r.manager == nil {
		return errors.New("ACP manager is not configured")
	}
	agentID := strings.TrimSpace(payload.AgentID)
	if agentID == "" {
		return errors.New("schedule payload agentId is required")
	}
	prompt := strings.TrimSpace(payload.Prompt)
	if prompt == "" {
		return errors.New("schedule payload prompt is required")
	}

	session, err := r.manager.NewSession(ctx, agentID, acpsdk.NewSessionRequest{})
	if err != nil {
		return err
	}
	defer func() {
		_, _ = r.manager.CloseSession(context.Background(), agentID, session.SessionId)
	}()

	_, err = r.manager.PromptText(ctx, agentID, session.SessionId, prompt)
	return err
}
