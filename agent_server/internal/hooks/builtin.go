package hooks

import (
	"context"
	"fmt"

	"github.com/icoo-ai/icoo-ai/internal/policy"
)

const BuiltinSecurityHookName = "builtin-security-policy"

type SecurityHook struct {
	Policy policy.Policy
}

func NewSecurityHook(p policy.Policy) SecurityHook {
	if p == nil {
		p = policy.New(policy.DefaultPermissionMode)
	}
	return SecurityHook{Policy: p}
}

func (h SecurityHook) Name() string {
	return BuiltinSecurityHookName
}

func (h SecurityHook) Match(event Event) bool {
	switch event.Type {
	case EventBeforeFileWrite, EventBeforeShellCommand:
		return true
	default:
		return false
	}
}

func (h SecurityHook) Execute(ctx context.Context, event Event) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	req, ok, err := PolicyRequestFromEvent(event)
	if err != nil {
		return Block(err.Error()), nil
	}
	if !ok {
		return Continue(), nil
	}
	return resultFromDecision(evaluate(h.Policy, req)), nil
}

type PolicyGuard struct {
	Policy policy.Policy
}

func NewPolicyGuard(p policy.Policy) PolicyGuard {
	if p == nil {
		p = policy.New(policy.DefaultPermissionMode)
	}
	return PolicyGuard{Policy: p}
}

func (g PolicyGuard) Name() string {
	return "policy-guard"
}

func (g PolicyGuard) Match(event Event) bool {
	_, ok, _ := PolicyRequestFromEvent(event)
	return ok
}

func (g PolicyGuard) Execute(ctx context.Context, event Event) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	req, ok, err := PolicyRequestFromEvent(event)
	if err != nil {
		return Block(err.Error()), nil
	}
	if !ok {
		return Continue(), nil
	}
	return resultFromDecision(evaluate(g.Policy, req)), nil
}

func WithPolicyGuard(p policy.Policy, hooks ...Hook) *DefaultDispatcher {
	guard := NewPolicyGuard(p)
	all := make([]Hook, 0, len(hooks)*2+1)
	all = append(all, NewSecurityHook(p))
	for _, hook := range hooks {
		all = append(all, hook, guard)
	}
	return NewDispatcher(all...)
}

func PolicyRequestFromEvent(event Event) (PolicyRequest, bool, error) {
	switch event.Type {
	case EventBeforeShellCommand:
		command, ok := stringField(event.Data, "command")
		if !ok || command == "" {
			return PolicyRequest{}, false, fmt.Errorf("before shell command hook event requires data.command")
		}
		workingDir, _ := stringField(event.Data, "working_dir")
		if workingDir == "" {
			workingDir = event.CWD
		}
		return PolicyRequest{Command: &policy.CommandRequest{Command: command, WorkingDir: workingDir}}, true, nil
	case EventBeforeFileWrite:
		path, ok := stringField(event.Data, "path")
		if !ok || path == "" {
			return PolicyRequest{}, false, fmt.Errorf("before file write hook event requires data.path")
		}
		workspaceRoot, _ := stringField(event.Data, "workspace_root")
		if workspaceRoot == "" {
			workspaceRoot = event.CWD
		}
		return PolicyRequest{Path: &policy.PathRequest{
			Path:          path,
			WorkspaceRoot: workspaceRoot,
			Operation:     policy.PathOperationWrite,
		}}, true, nil
	default:
		if event.Data == nil {
			return PolicyRequest{}, false, nil
		}
		if req, ok := event.Data["policy_request"].(PolicyRequest); ok {
			return req, true, nil
		}
		return PolicyRequest{}, false, nil
	}
}

func evaluate(p policy.Policy, req PolicyRequest) policy.Decision {
	if req.Command != nil {
		return p.EvaluateCommand(*req.Command)
	}
	if req.Path != nil {
		return p.EvaluatePath(*req.Path)
	}
	if req.Network != nil {
		return p.EvaluateNetwork(*req.Network)
	}
	if req.MCP != nil {
		return p.EvaluateMCP(*req.MCP)
	}
	return policy.Decision{Action: policy.DecisionAllow}
}

func resultFromDecision(decision policy.Decision) Result {
	data := map[string]any{
		"policy_decision": decision,
	}
	switch decision.Action {
	case policy.DecisionAllow:
		return Result{Action: ActionContinue, Reason: decision.Reason, Data: data}
	case policy.DecisionRequestApproval:
		return Result{Action: ActionRequestApproval, Reason: decision.Reason, Data: data}
	case policy.DecisionBlock:
		return Result{Action: ActionBlock, Reason: decision.Reason, Data: data}
	default:
		return Result{Action: ActionRequestApproval, Reason: "unknown policy decision requires approval", Data: data}
	}
}

func stringField(data map[string]any, key string) (string, bool) {
	if data == nil {
		return "", false
	}
	value, ok := data[key]
	if !ok {
		return "", false
	}
	str, ok := value.(string)
	return str, ok
}
