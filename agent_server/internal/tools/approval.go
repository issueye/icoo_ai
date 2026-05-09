package tools

import (
	"context"
	"encoding/json"
)

type ApprovalScope string

const (
	ApprovalScopeOnce   ApprovalScope = "once"
	ApprovalScopeAlways ApprovalScope = "always"
)

type ApprovalCapable interface {
	ApprovalKey(input json.RawMessage) (string, bool)
	ExecuteApproved(ctx context.Context, input json.RawMessage, scope ApprovalScope) (ToolResult, error)
}
