package store

import (
	"context"
	"errors"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

var ErrMissingID = errors.New("store: missing required id")
var ErrInvalidConfig = errors.New("store: invalid config")

type Store interface {
	UpsertConversation(ctx context.Context, conversation models.Conversation) error
	ListConversations(ctx context.Context) ([]models.Conversation, error)
	GetConversation(ctx context.Context, sessionID string) (models.Conversation, bool, error)
	AppendMessage(ctx context.Context, event models.MessageEvent) error
	ListMessages(ctx context.Context, sessionID string) ([]models.MessageEvent, error)
	UpsertRun(ctx context.Context, run models.RunSummary) error
	ListRuns(ctx context.Context, sessionID string) ([]models.RunSummary, error)
	UpsertApproval(ctx context.Context, approval models.ApprovalDecision) error
	ListApprovals(ctx context.Context) ([]models.ApprovalDecision, error)
	AppendAudit(ctx context.Context, event models.AuditEvent) error
	ListAuditEvents(ctx context.Context) ([]models.AuditEvent, error)
}

type JSONLConfig struct {
	Dir string
}
