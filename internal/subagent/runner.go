package subagent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/icoo-ai/icoo-ai/internal/agent"
	"github.com/icoo-ai/icoo-ai/internal/audit"
	"github.com/icoo-ai/icoo-ai/internal/hooks"
	"github.com/icoo-ai/icoo-ai/internal/llm"
	"github.com/icoo-ai/icoo-ai/internal/tools"
)

type Request struct {
	SessionID     string         `json:"session_id,omitempty"`
	CWD           string         `json:"cwd,omitempty"`
	Task          string         `json:"task"`
	Skill         *agent.Skill   `json:"skill,omitempty"`
	Context       []string       `json:"context,omitempty"`
	Model         string         `json:"model,omitempty"`
	MaxToolRounds int            `json:"max_tool_rounds,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Approver      agent.Approver `json:"-"`
}

type Result struct {
	Content string        `json:"content"`
	Events  []agent.Event `json:"events,omitempty"`
}

type Runner interface {
	Run(ctx context.Context, req Request) (Result, error)
}

type LocalRunnerOptions struct {
	Provider      llm.Provider
	Tools         []tools.Tool
	Model         string
	MaxToolRounds int
	Approver      agent.Approver
	Hooks         hooks.Dispatcher
	AuditLogger   audit.Logger
}

type LocalRunner struct {
	provider      llm.Provider
	tools         []tools.Tool
	model         string
	maxToolRounds int
	approver      agent.Approver
	hooks         hooks.Dispatcher
	auditLogger   audit.Logger
}

func NewLocalRunner(opts LocalRunnerOptions) (*LocalRunner, error) {
	if opts.Provider == nil {
		return nil, errors.New("subagent local runner requires provider")
	}
	maxToolRounds := opts.MaxToolRounds
	if maxToolRounds <= 0 {
		maxToolRounds = 6
	}
	return &LocalRunner{
		provider:      opts.Provider,
		tools:         append([]tools.Tool(nil), opts.Tools...),
		model:         opts.Model,
		maxToolRounds: maxToolRounds,
		approver:      opts.Approver,
		hooks:         opts.Hooks,
		auditLogger:   opts.AuditLogger,
	}, nil
}

func (r *LocalRunner) Run(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.Task) == "" {
		return Result{}, errors.New("subagent task is required")
	}
	loop, err := agent.NewReactLoop(agent.ReactLoopOptions{
		Provider:      r.provider,
		Tools:         r.tools,
		MaxToolRounds: firstPositive(req.MaxToolRounds, r.maxToolRounds),
	})
	if err != nil {
		return Result{}, err
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = r.model
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = "subagent"
	}
	cwd := strings.TrimSpace(req.CWD)

	events, err := loop.Run(ctx, agent.RunRequest{
		SessionID: sessionID,
		CWD:       cwd,
		Messages: []llm.Message{
			{Role: "system", Content: buildSystemPrompt(req.Skill)},
			{Role: "user", Content: buildUserPrompt(req.Task, req.Context)},
		},
		Context: agent.WorkspaceContext{Root: cwd},
		Options: agent.RunOptions{
			Model:       model,
			Approver:    firstApprover(req.Approver, r.approver),
			Hooks:       r.hooks,
			AuditLogger: r.auditLogger,
			Metadata:    req.Metadata,
		},
	})
	if err != nil {
		return Result{}, err
	}

	var result Result
	for event := range events {
		result.Events = append(result.Events, event)
		if event.Type == agent.EventMessageDelta {
			result.Content += event.Content
		}
		if event.Type == agent.EventRunFailed {
			return result, fmt.Errorf("subagent run failed: %s", event.Error)
		}
		if event.Type == agent.EventRunCancelled {
			return result, fmt.Errorf("subagent run cancelled: %s", event.Error)
		}
	}
	return result, nil
}

func buildSystemPrompt(skill *agent.Skill) string {
	var b strings.Builder
	b.WriteString("You are a focused subagent. Complete the delegated task and return a concise result to the parent agent.")
	if skill == nil {
		return b.String()
	}
	b.WriteString("\n\nUse the following skill instructions exactly when they are relevant.")
	b.WriteString("\n\nSkill: ")
	b.WriteString(skill.Name)
	if skill.Description != "" {
		b.WriteString("\nDescription: ")
		b.WriteString(skill.Description)
	}
	if skill.Path != "" {
		b.WriteString("\nPath: ")
		b.WriteString(skill.Path)
	}
	if strings.TrimSpace(skill.Body) != "" {
		b.WriteString("\n\n")
		b.WriteString(skill.Body)
	}
	return b.String()
}

func buildUserPrompt(task string, contextItems []string) string {
	task = strings.TrimSpace(task)
	if len(contextItems) == 0 {
		return task
	}
	var b strings.Builder
	b.WriteString(task)
	b.WriteString("\n\nAdditional context:")
	for _, item := range contextItems {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		b.WriteString("\n- ")
		b.WriteString(item)
	}
	return b.String()
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstApprover(values ...agent.Approver) agent.Approver {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
