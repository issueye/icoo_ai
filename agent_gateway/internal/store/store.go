package store

import (
	"context"
	"errors"
)

var ErrMissingID = errors.New("store: missing required id")
var ErrInvalidConfig = errors.New("store: invalid config")

type Store interface {
	UpsertConversation(ctx context.Context, conversation Conversation) error
	ListConversations(ctx context.Context) ([]Conversation, error)
	GetConversation(ctx context.Context, sessionID string) (Conversation, bool, error)
	AppendMessage(ctx context.Context, event MessageEvent) error
	ListMessages(ctx context.Context, sessionID string) ([]MessageEvent, error)
	UpsertRun(ctx context.Context, run RunSummary) error
	ListRuns(ctx context.Context, sessionID string) ([]RunSummary, error)
	UpsertApproval(ctx context.Context, approval ApprovalDecision) error
	ListApprovals(ctx context.Context) ([]ApprovalDecision, error)
	AppendAudit(ctx context.Context, event AuditEvent) error
	ListAuditEvents(ctx context.Context) ([]AuditEvent, error)
}

type JSONLConfig struct {
	Dir string
}
