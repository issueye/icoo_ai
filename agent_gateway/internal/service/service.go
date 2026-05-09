package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type GatewayService interface {
	ListAgents(ctx context.Context) ([]AgentProfile, error)
	CreateSession(ctx context.Context, req CreateSessionRequest) (Session, error)
	ListSessions(ctx context.Context) ([]Session, error)
	ListMessages(ctx context.Context, sessionID string) ([]Message, error)
	Prompt(ctx context.Context, sessionID string, req PromptRequest) (PromptResponse, error)
	Cancel(ctx context.Context, sessionID string) (Run, error)
	ListRuns(ctx context.Context) ([]Run, error)
	ListApprovals(ctx context.Context) ([]Approval, error)
	DecideApproval(ctx context.Context, approvalID string, req ApprovalDecisionRequest) (Approval, error)
}

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func NewError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

type MockGatewayService struct {
	mu     sync.Mutex
	now    func() time.Time
	nextID int
	agents []AgentProfile
	store  store.Store
}

func NewMockGatewayService() *MockGatewayService {
	return NewMockGatewayServiceWithStore(store.NewMemoryStore())
}

func NewMockGatewayServiceWithStore(st store.Store) *MockGatewayService {
	return NewMockGatewayServiceWithAgentsAndStore(defaultAgents(), st)
}

func NewMockGatewayServiceWithAgentsAndStore(agents []AgentProfile, st store.Store) *MockGatewayService {
	if len(agents) == 0 {
		agents = defaultAgents()
	}
	return &MockGatewayService{
		now:    time.Now,
		nextID: 1,
		agents: agents,
		store: st,
	}
}

func defaultAgents() []AgentProfile {
	return []AgentProfile{
		{
			ID:          "icoo-ai-acp",
			Name:        "Icoo AI",
			Protocol:    "acp",
			Models:      []string{"mock-gpt"},
			Description: "Mock gateway agent for local API development.",
		},
	}
}

func (s *MockGatewayService) ListAgents(ctx context.Context) ([]AgentProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	agents := make([]AgentProfile, len(s.agents))
	copy(agents, s.agents)
	return agents, nil
}

func (s *MockGatewayService) CreateSession(ctx context.Context, req CreateSessionRequest) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		agentID = s.agents[0].ID
	}
	if !s.hasAgentLocked(agentID) {
		return Session{}, NewError("agent_not_found", fmt.Sprintf("agent %q was not found", agentID))
	}

	now := s.now()
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "New Agent Session"
	}
	session := Session{
		ID:        s.idLocked("sess"),
		Title:     title,
		CWD:       req.CWD,
		AgentID:   agentID,
		Model:     req.Model,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.UpsertConversation(ctx, toStoreConversation(session)); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *MockGatewayService) ListSessions(ctx context.Context) ([]Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conversations, err := s.store.ListConversations(ctx)
	if err != nil {
		return nil, err
	}
	sessions := make([]Session, 0, len(conversations))
	for _, conversation := range conversations {
		sessions = append(sessions, fromStoreConversation(conversation))
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.Before(sessions[j].CreatedAt)
	})
	return sessions, nil
}

func (s *MockGatewayService) ListMessages(ctx context.Context, sessionID string) ([]Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, ok, err := s.store.GetConversation(ctx, sessionID); err != nil {
		return nil, err
	} else if !ok {
		return nil, NewError("session_not_found", fmt.Sprintf("session %q was not found", sessionID))
	}

	events, err := s.store.ListMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	messages := make([]Message, 0, len(events))
	for _, event := range events {
		messages = append(messages, fromStoreMessageEvent(event))
	}
	return messages, nil
}

func (s *MockGatewayService) Prompt(ctx context.Context, sessionID string, req PromptRequest) (PromptResponse, error) {
	if err := ctx.Err(); err != nil {
		return PromptResponse{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	conversation, ok, err := s.store.GetConversation(ctx, sessionID)
	if err != nil {
		return PromptResponse{}, err
	}
	if !ok {
		return PromptResponse{}, NewError("session_not_found", fmt.Sprintf("session %q was not found", sessionID))
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return PromptResponse{}, NewError("invalid_prompt", "prompt content is required")
	}

	session := fromStoreConversation(conversation)
	startedAt := s.now()
	endedAt := startedAt
	run := Run{
		ID:        s.idLocked("run"),
		SessionID: sessionID,
		AgentID:   session.AgentID,
		Status:    "completed",
		StartedAt: startedAt,
		EndedAt:   &endedAt,
	}
	userMessage := Message{
		ID:        s.idLocked("msg"),
		SessionID: sessionID,
		RunID:     run.ID,
		Role:      "user",
		Content:   content,
		CreatedAt: startedAt,
	}
	assistantMessage := Message{
		ID:        s.idLocked("msg"),
		SessionID: sessionID,
		RunID:     run.ID,
		Role:      "assistant",
		Content:   "Mock response from agent_gateway.",
		CreatedAt: endedAt,
	}
	approval := Approval{
		ID:                 s.idLocked("appr"),
		AgentID:            session.AgentID,
		SessionID:          sessionID,
		RunID:              run.ID,
		ConnectorRequestID: s.idLocked("connreq"),
		Status:             "pending",
		Action:             "mock_tool",
		Message:            "Mock approval request",
		CreatedAt:          startedAt,
	}

	if err := s.store.UpsertRun(ctx, toStoreRun(run)); err != nil {
		return PromptResponse{}, err
	}
	if err := s.store.AppendMessage(ctx, toStoreMessageEvent(userMessage)); err != nil {
		return PromptResponse{}, err
	}
	if err := s.store.AppendMessage(ctx, toStoreMessageEvent(assistantMessage)); err != nil {
		return PromptResponse{}, err
	}
	if err := s.store.UpsertApproval(ctx, toStoreApproval(approval)); err != nil {
		return PromptResponse{}, err
	}
	session.UpdatedAt = endedAt
	if err := s.store.UpsertConversation(ctx, toStoreConversation(session)); err != nil {
		return PromptResponse{}, err
	}

	return PromptResponse{
		Run:      run,
		Messages: []Message{userMessage, assistantMessage},
		Approval: &approval,
	}, nil
}

func (s *MockGatewayService) Cancel(ctx context.Context, sessionID string) (Run, error) {
	if err := ctx.Err(); err != nil {
		return Run{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	conversation, ok, err := s.store.GetConversation(ctx, sessionID)
	if err != nil {
		return Run{}, err
	}
	if !ok {
		return Run{}, NewError("session_not_found", fmt.Sprintf("session %q was not found", sessionID))
	}
	session := fromStoreConversation(conversation)
	now := s.now()
	run := Run{
		ID:        s.idLocked("run"),
		SessionID: sessionID,
		AgentID:   session.AgentID,
		Status:    "cancelled",
		StartedAt: now,
		EndedAt:   &now,
	}
	if err := s.store.UpsertRun(ctx, toStoreRun(run)); err != nil {
		return Run{}, err
	}
	storedApprovals, err := s.store.ListApprovals(ctx)
	if err != nil {
		return Run{}, err
	}
	for _, storedApproval := range storedApprovals {
		if storedApproval.SessionID != sessionID || storedApproval.Status != "pending" {
			continue
		}
		approval := fromStoreApproval(storedApproval)
		approval.Status = "expired"
		approval.Decision = "rejected"
		approval.DecidedAt = &now
		approval.Message = "Approval expired because session was cancelled."
		if err := s.store.UpsertApproval(ctx, toStoreApproval(approval)); err != nil {
			return Run{}, err
		}
	}
	session.UpdatedAt = now
	if err := s.store.UpsertConversation(ctx, toStoreConversation(session)); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (s *MockGatewayService) ListRuns(ctx context.Context) ([]Run, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	storedRuns, err := s.store.ListRuns(ctx, "")
	if err != nil {
		return nil, err
	}
	runs := make([]Run, 0, len(storedRuns))
	for _, storedRun := range storedRuns {
		runs = append(runs, fromStoreRun(storedRun))
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.Before(runs[j].StartedAt)
	})
	return runs, nil
}

func (s *MockGatewayService) ListApprovals(ctx context.Context) ([]Approval, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	storedApprovals, err := s.store.ListApprovals(ctx)
	if err != nil {
		return nil, err
	}
	approvals := make([]Approval, 0, len(storedApprovals))
	for _, storedApproval := range storedApprovals {
		approvals = append(approvals, fromStoreApproval(storedApproval))
	}
	sort.Slice(approvals, func(i, j int) bool {
		return approvals[i].CreatedAt.Before(approvals[j].CreatedAt)
	})
	return approvals, nil
}

func (s *MockGatewayService) DecideApproval(ctx context.Context, approvalID string, req ApprovalDecisionRequest) (Approval, error) {
	if err := ctx.Err(); err != nil {
		return Approval{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	storedApprovals, err := s.store.ListApprovals(ctx)
	if err != nil {
		return Approval{}, err
	}

	var approval Approval
	found := false
	for _, storedApproval := range storedApprovals {
		if storedApproval.ID == approvalID {
			approval = fromStoreApproval(storedApproval)
			found = true
			break
		}
	}
	if !found {
		return Approval{}, NewError("approval_not_found", fmt.Sprintf("approval %q was not found", approvalID))
	}
	if approval.Status != "pending" {
		return Approval{}, NewError("invalid_decision", "approval is no longer pending")
	}

	decision := strings.ToLower(strings.TrimSpace(req.Decision))
	if decision != "approved" && decision != "rejected" && decision != "allow" && decision != "deny" {
		return Approval{}, NewError("invalid_decision", "decision must be approved, rejected, allow, or deny")
	}
	if decision == "allow" {
		decision = "approved"
	}
	if decision == "deny" {
		decision = "rejected"
	}

	now := s.now()
	approval.Status = decision
	approval.Decision = decision
	approval.DecidedAt = &now
	if strings.TrimSpace(req.Message) != "" {
		approval.Message = req.Message
	}
	if err := s.store.UpsertApproval(ctx, toStoreApproval(approval)); err != nil {
		return Approval{}, err
	}
	return approval, nil
}

func (s *MockGatewayService) hasAgentLocked(agentID string) bool {
	for _, agent := range s.agents {
		if agent.ID == agentID {
			return true
		}
	}
	return false
}

func (s *MockGatewayService) idLocked(prefix string) string {
	id := fmt.Sprintf("%s_%06d", prefix, s.nextID)
	s.nextID++
	return id
}

func toStoreConversation(session Session) store.Conversation {
	return store.Conversation{
		ID:        session.ID,
		AgentID:   session.AgentID,
		SessionID: session.ID,
		Title:     session.Title,
		Status:    session.Status,
		Model:     session.Model,
		CWD:       session.CWD,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}
}

func fromStoreConversation(conversation store.Conversation) Session {
	return Session{
		ID:        conversation.SessionID,
		Title:     conversation.Title,
		CWD:       conversation.CWD,
		AgentID:   conversation.AgentID,
		Model:     conversation.Model,
		Status:    conversation.Status,
		CreatedAt: conversation.CreatedAt,
		UpdatedAt: conversation.UpdatedAt,
	}
}

func toStoreMessageEvent(message Message) store.MessageEvent {
	return store.MessageEvent{
		ID:        message.ID,
		Type:      "message",
		SessionID: message.SessionID,
		RunID:     message.RunID,
		Role:      message.Role,
		Summary:   message.Content,
		CreatedAt: message.CreatedAt,
	}
}

func fromStoreMessageEvent(event store.MessageEvent) Message {
	return Message{
		ID:        event.ID,
		SessionID: event.SessionID,
		RunID:     event.RunID,
		Role:      event.Role,
		Content:   event.Summary,
		CreatedAt: event.CreatedAt,
	}
}

func toStoreRun(run Run) store.RunSummary {
	return store.RunSummary{
		ID:          run.ID,
		AgentID:     run.AgentID,
		SessionID:   run.SessionID,
		RunID:       run.ID,
		Status:      run.Status,
		CreatedAt:   run.StartedAt,
		UpdatedAt:   timePointerValue(run.EndedAt, run.StartedAt),
		CompletedAt: run.EndedAt,
	}
}

func fromStoreRun(run store.RunSummary) Run {
	return Run{
		ID:        run.RunID,
		SessionID: run.SessionID,
		AgentID:   run.AgentID,
		Status:    run.Status,
		StartedAt: run.CreatedAt,
		EndedAt:   run.CompletedAt,
	}
}

func toStoreApproval(approval Approval) store.ApprovalDecision {
	return store.ApprovalDecision{
		ID:                 approval.ID,
		AgentID:            approval.AgentID,
		SessionID:          approval.SessionID,
		RunID:              approval.RunID,
		ConnectorRequestID: approval.ConnectorRequestID,
		Status:             approval.Status,
		Decision:           approval.Decision,
		Summary:            approval.Message,
		CreatedAt:          approval.CreatedAt,
		UpdatedAt:          timePointerValue(approval.DecidedAt, approval.CreatedAt),
		DecidedAt:          approval.DecidedAt,
		SafeMeta: store.SafeMeta{
			"action": approval.Action,
		},
	}
}

func fromStoreApproval(approval store.ApprovalDecision) Approval {
	action, _ := approval.SafeMeta["action"].(string)
	return Approval{
		ID:                 approval.ID,
		AgentID:            approval.AgentID,
		SessionID:          approval.SessionID,
		RunID:              approval.RunID,
		ConnectorRequestID: approval.ConnectorRequestID,
		Status:             approval.Status,
		Action:             action,
		Message:            approval.Summary,
		Decision:           approval.Decision,
		DecidedAt:          approval.DecidedAt,
		CreatedAt:          approval.CreatedAt,
	}
}

func timePointerValue(in *time.Time, fallback time.Time) time.Time {
	if in == nil {
		return fallback
	}
	return *in
}
