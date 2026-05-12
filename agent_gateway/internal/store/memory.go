package store

import (
	"context"
	"sync"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type MemoryStore struct {
	mu sync.RWMutex

	conversations     map[string]models.Conversation
	conversationOrder []string
	messages          []models.MessageEvent
	runs              map[string]models.RunSummary
	runOrder          []string
	approvals         map[string]models.ApprovalDecision
	approvalOrder     []string
	auditEvents       []models.AuditEvent
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		conversations: make(map[string]models.Conversation),
		runs:          make(map[string]models.RunSummary),
		approvals:     make(map[string]models.ApprovalDecision),
	}
}

func (s *MemoryStore) UpsertConversation(ctx context.Context, conversation models.Conversation) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	conversation = cloneConversation(conversation)
	if conversation.SessionID == "" {
		conversation.SessionID = conversation.ID
	}
	if conversation.ID == "" {
		conversation.ID = conversation.SessionID
	}
	if conversation.SessionID == "" {
		return ErrMissingID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.conversations[conversation.SessionID]; !ok {
		s.conversationOrder = append(s.conversationOrder, conversation.SessionID)
	}
	s.conversations[conversation.SessionID] = conversation
	return nil
}

func (s *MemoryStore) ListConversations(ctx context.Context) ([]models.Conversation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]models.Conversation, 0, len(s.conversationOrder))
	for _, sessionID := range s.conversationOrder {
		out = append(out, cloneConversation(s.conversations[sessionID]))
	}
	return out, nil
}

func (s *MemoryStore) GetConversation(ctx context.Context, sessionID string) (models.Conversation, bool, error) {
	if err := ctx.Err(); err != nil {
		return models.Conversation{}, false, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	conversation, ok := s.conversations[sessionID]
	if !ok {
		return models.Conversation{}, false, nil
	}
	return cloneConversation(conversation), true, nil
}

func (s *MemoryStore) AppendMessage(ctx context.Context, event models.MessageEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.ID == "" || event.SessionID == "" {
		return ErrMissingID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, cloneMessageEvent(event))
	return nil
}

func (s *MemoryStore) ListMessages(ctx context.Context, sessionID string) ([]models.MessageEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]models.MessageEvent, 0, len(s.messages))
	for _, event := range s.messages {
		if event.SessionID == sessionID {
			out = append(out, cloneMessageEvent(event))
		}
	}
	return out, nil
}

func (s *MemoryStore) UpsertRun(ctx context.Context, run models.RunSummary) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	run = cloneRunSummary(run)
	if run.RunID == "" {
		run.RunID = run.ID
	}
	if run.ID == "" {
		run.ID = run.RunID
	}
	if run.RunID == "" || run.SessionID == "" {
		return ErrMissingID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.runs[run.RunID]; !ok {
		s.runOrder = append(s.runOrder, run.RunID)
	}
	s.runs[run.RunID] = run
	return nil
}

func (s *MemoryStore) ListRuns(ctx context.Context, sessionID string) ([]models.RunSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]models.RunSummary, 0, len(s.runOrder))
	for _, runID := range s.runOrder {
		run := s.runs[runID]
		if sessionID == "" || run.SessionID == sessionID {
			out = append(out, cloneRunSummary(run))
		}
	}
	return out, nil
}

func (s *MemoryStore) UpsertApproval(ctx context.Context, approval models.ApprovalDecision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if approval.ID == "" || approval.SessionID == "" || approval.RunID == "" || approval.ConnectorRequestID == "" {
		return ErrMissingID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.approvals[approval.ID]; !ok {
		s.approvalOrder = append(s.approvalOrder, approval.ID)
	}
	s.approvals[approval.ID] = cloneApprovalDecision(approval)
	return nil
}

func (s *MemoryStore) ListApprovals(ctx context.Context) ([]models.ApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]models.ApprovalDecision, 0, len(s.approvalOrder))
	for _, approvalID := range s.approvalOrder {
		out = append(out, cloneApprovalDecision(s.approvals[approvalID]))
	}
	return out, nil
}

func (s *MemoryStore) AppendAudit(ctx context.Context, event models.AuditEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.ID == "" {
		return ErrMissingID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.auditEvents = append(s.auditEvents, cloneAuditEvent(event))
	return nil
}

func (s *MemoryStore) ListAuditEvents(ctx context.Context) ([]models.AuditEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]models.AuditEvent, 0, len(s.auditEvents))
	for _, event := range s.auditEvents {
		out = append(out, cloneAuditEvent(event))
	}
	return out, nil
}

func cloneConversation(in models.Conversation) models.Conversation {
	in.SafeMeta = cloneSafeMeta(in.SafeMeta)
	return in
}

func cloneMessageEvent(in models.MessageEvent) models.MessageEvent {
	in.SafeMeta = cloneSafeMeta(in.SafeMeta)
	return in
}

func cloneRunSummary(in models.RunSummary) models.RunSummary {
	in.SafeMeta = cloneSafeMeta(in.SafeMeta)
	return in
}

func cloneApprovalDecision(in models.ApprovalDecision) models.ApprovalDecision {
	in.SafeMeta = cloneSafeMeta(in.SafeMeta)
	return in
}

func cloneAuditEvent(in models.AuditEvent) models.AuditEvent {
	in.SafeMeta = cloneSafeMeta(in.SafeMeta)
	return in
}

func cloneSafeMeta(in models.SafeMeta) models.SafeMeta {
	if len(in) == 0 {
		return nil
	}

	out := make(models.SafeMeta, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
