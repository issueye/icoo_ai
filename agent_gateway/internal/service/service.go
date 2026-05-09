package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
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
	mu        sync.Mutex
	now       func() time.Time
	nextID    int
	agents    []AgentProfile
	sessions  map[string]Session
	messages  map[string][]Message
	runs      map[string]Run
	approvals map[string]Approval
}

func NewMockGatewayService() *MockGatewayService {
	return &MockGatewayService{
		now:    time.Now,
		nextID: 1,
		agents: []AgentProfile{
			{
				ID:          "icoo-ai-acp",
				Name:        "Icoo AI",
				Protocol:    "acp",
				Models:      []string{"mock-gpt"},
				Description: "Mock gateway agent for local API development.",
			},
		},
		sessions:  make(map[string]Session),
		messages:  make(map[string][]Message),
		runs:      make(map[string]Run),
		approvals: make(map[string]Approval),
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
	s.sessions[session.ID] = session
	return session, nil
}

func (s *MockGatewayService) ListSessions(ctx context.Context) ([]Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions := make([]Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[sessionID]; !ok {
		return nil, NewError("session_not_found", fmt.Sprintf("session %q was not found", sessionID))
	}
	messages := append([]Message(nil), s.messages[sessionID]...)
	return messages, nil
}

func (s *MockGatewayService) Prompt(ctx context.Context, sessionID string, req PromptRequest) (PromptResponse, error) {
	if err := ctx.Err(); err != nil {
		return PromptResponse{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return PromptResponse{}, NewError("session_not_found", fmt.Sprintf("session %q was not found", sessionID))
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return PromptResponse{}, NewError("invalid_prompt", "prompt content is required")
	}

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

	s.runs[run.ID] = run
	s.messages[sessionID] = append(s.messages[sessionID], userMessage, assistantMessage)
	s.approvals[approval.ID] = approval
	session.UpdatedAt = endedAt
	s.sessions[sessionID] = session

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

	session, ok := s.sessions[sessionID]
	if !ok {
		return Run{}, NewError("session_not_found", fmt.Sprintf("session %q was not found", sessionID))
	}
	now := s.now()
	run := Run{
		ID:        s.idLocked("run"),
		SessionID: sessionID,
		AgentID:   session.AgentID,
		Status:    "cancelled",
		StartedAt: now,
		EndedAt:   &now,
	}
	s.runs[run.ID] = run
	session.UpdatedAt = now
	s.sessions[sessionID] = session
	return run, nil
}

func (s *MockGatewayService) ListRuns(ctx context.Context) ([]Run, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	runs := make([]Run, 0, len(s.runs))
	for _, run := range s.runs {
		runs = append(runs, run)
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
	s.mu.Lock()
	defer s.mu.Unlock()

	approvals := make([]Approval, 0, len(s.approvals))
	for _, approval := range s.approvals {
		approvals = append(approvals, approval)
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

	approval, ok := s.approvals[approvalID]
	if !ok {
		return Approval{}, NewError("approval_not_found", fmt.Sprintf("approval %q was not found", approvalID))
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
	s.approvals[approvalID] = approval
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
