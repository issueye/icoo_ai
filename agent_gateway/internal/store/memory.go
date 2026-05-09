package store

import (
	"context"
	"sync"
)

type MemoryStore struct {
	mu sync.RWMutex

	conversations     map[string]Conversation
	conversationOrder []string
	messages          []MessageEvent
	runs              map[string]RunSummary
	runOrder          []string
	approvals         map[string]ApprovalDecision
	approvalOrder     []string
	auditEvents       []AuditEvent
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		conversations: make(map[string]Conversation),
		runs:          make(map[string]RunSummary),
		approvals:     make(map[string]ApprovalDecision),
	}
}

func (s *MemoryStore) UpsertConversation(ctx context.Context, conversation Conversation) error {
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

func (s *MemoryStore) ListConversations(ctx context.Context) ([]Conversation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Conversation, 0, len(s.conversationOrder))
	for _, sessionID := range s.conversationOrder {
		out = append(out, cloneConversation(s.conversations[sessionID]))
	}
	return out, nil
}

func (s *MemoryStore) GetConversation(ctx context.Context, sessionID string) (Conversation, bool, error) {
	if err := ctx.Err(); err != nil {
		return Conversation{}, false, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	conversation, ok := s.conversations[sessionID]
	if !ok {
		return Conversation{}, false, nil
	}
	return cloneConversation(conversation), true, nil
}

func (s *MemoryStore) AppendMessage(ctx context.Context, event MessageEvent) error {
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

func (s *MemoryStore) ListMessages(ctx context.Context, sessionID string) ([]MessageEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]MessageEvent, 0, len(s.messages))
	for _, event := range s.messages {
		if event.SessionID == sessionID {
			out = append(out, cloneMessageEvent(event))
		}
	}
	return out, nil
}

func (s *MemoryStore) UpsertRun(ctx context.Context, run RunSummary) error {
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

func (s *MemoryStore) ListRuns(ctx context.Context, sessionID string) ([]RunSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]RunSummary, 0, len(s.runOrder))
	for _, runID := range s.runOrder {
		run := s.runs[runID]
		if sessionID == "" || run.SessionID == sessionID {
			out = append(out, cloneRunSummary(run))
		}
	}
	return out, nil
}

func (s *MemoryStore) UpsertApproval(ctx context.Context, approval ApprovalDecision) error {
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

func (s *MemoryStore) ListApprovals(ctx context.Context) ([]ApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]ApprovalDecision, 0, len(s.approvalOrder))
	for _, approvalID := range s.approvalOrder {
		out = append(out, cloneApprovalDecision(s.approvals[approvalID]))
	}
	return out, nil
}

func (s *MemoryStore) AppendAudit(ctx context.Context, event AuditEvent) error {
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

func (s *MemoryStore) ListAuditEvents(ctx context.Context) ([]AuditEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]AuditEvent, 0, len(s.auditEvents))
	for _, event := range s.auditEvents {
		out = append(out, cloneAuditEvent(event))
	}
	return out, nil
}

func cloneConversation(in Conversation) Conversation {
	in.SafeMeta = cloneSafeMeta(in.SafeMeta)
	return in
}

func cloneMessageEvent(in MessageEvent) MessageEvent {
	in.SafeMeta = cloneSafeMeta(in.SafeMeta)
	return in
}

func cloneRunSummary(in RunSummary) RunSummary {
	in.SafeMeta = cloneSafeMeta(in.SafeMeta)
	return in
}

func cloneApprovalDecision(in ApprovalDecision) ApprovalDecision {
	in.SafeMeta = cloneSafeMeta(in.SafeMeta)
	return in
}

func cloneAuditEvent(in AuditEvent) AuditEvent {
	in.SafeMeta = cloneSafeMeta(in.SafeMeta)
	return in
}

func cloneSafeMeta(in SafeMeta) SafeMeta {
	if len(in) == 0 {
		return nil
	}

	out := make(SafeMeta, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
