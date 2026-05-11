package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type GatewayService interface {
	ListAgents(ctx context.Context) ([]AgentProfile, error)
	ListSkills(ctx context.Context) ([]Skill, error)
	GetManagementSettings(ctx context.Context) (ManagementSettings, error)
	UpdateManagementSettings(ctx context.Context, in ManagementSettings) (ManagementSettings, error)
	CreateSession(ctx context.Context, req CreateSessionRequest) (Session, error)
	ListSessions(ctx context.Context) ([]Session, error)
	GetSession(ctx context.Context, sessionID string) (Session, error)
	ResumeSession(ctx context.Context, sessionID string, req ResumeSessionRequest) (Session, error)
	CloseSession(ctx context.Context, sessionID string) (Session, error)
	SetSessionMode(ctx context.Context, sessionID string, req SetSessionModeRequest) (Session, error)
	SetSessionConfigOption(ctx context.Context, sessionID string, req SetSessionConfigOptionRequest) (Session, error)
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
	mu               sync.Mutex
	now              func() time.Time
	nextID           int
	agents           []AgentProfile
	bootstrapAgents  []AgentProfile
	skills           []Skill
	store            store.Store
	connector        connector.AgentConnector
	approvalBroker   *ApprovalBroker
	managementStore  ManagementSettingsStore
	managementLoaded bool
	management       ManagementSettings
}

func NewMockGatewayService() *MockGatewayService {
	return NewMockGatewayServiceWithStore(store.NewMemoryStore())
}

func NewMockGatewayServiceWithStore(st store.Store) *MockGatewayService {
	return NewMockGatewayServiceWithAgentsAndStore(defaultAgents(), st)
}

func NewMockGatewayServiceWithAgentsAndStore(agents []AgentProfile, st store.Store) *MockGatewayService {
	return NewMockGatewayServiceWithAgentsStoreAndSettingsStore(agents, st, NewMemoryManagementSettingsStore())
}

func NewMockGatewayServiceWithAgentsStoreAndSettingsStore(agents []AgentProfile, st store.Store, settingsStore ManagementSettingsStore) *MockGatewayService {
	if len(agents) == 0 {
		agents = defaultAgents()
	}
	if settingsStore == nil {
		settingsStore = NewMemoryManagementSettingsStore()
	}
	bootstrapAgents := cloneAgentProfiles(agents)
	return &MockGatewayService{
		now:             time.Now,
		nextID:          1,
		agents:          cloneAgentProfiles(agents),
		bootstrapAgents: bootstrapAgents,
		skills:          defaultSkills(),
		store:           st,
		approvalBroker:  NewApprovalBroker(),
		managementStore: settingsStore,
		management: ManagementSettings{
			MCPServers:    []MCPServerConfig{},
			ScheduleTasks: []ScheduleTaskConfig{},
			Agents:        toAgentConfigs(bootstrapAgents),
		},
	}
}

func NewConnectorGatewayServiceWithAgentsAndStore(agents []AgentProfile, st store.Store, conn connector.AgentConnector) *MockGatewayService {
	svc := NewMockGatewayServiceWithAgentsStoreAndSettingsStore(agents, st, NewMemoryManagementSettingsStore())
	svc.connector = conn
	return svc
}

func NewConnectorGatewayServiceWithAgentsStoreAndSettingsStore(agents []AgentProfile, st store.Store, settingsStore ManagementSettingsStore, conn connector.AgentConnector) *MockGatewayService {
	svc := NewMockGatewayServiceWithAgentsStoreAndSettingsStore(agents, st, settingsStore)
	svc.connector = conn
	return svc
}

func defaultAgents() []AgentProfile {
	return []AgentProfile{
		{
			ID:          "icoo-ai-acp",
			Name:        "Icoo AI",
			Protocol:    "acp",
			Models:      []string{"gpt-5.4"},
			Description: "Default ACP connector profile.",
		},
	}
}

func defaultSkills() []Skill {
	return []Skill{}
}

func (s *MockGatewayService) ListAgents(ctx context.Context) ([]AgentProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureManagementLoadedLocked(ctx); err != nil {
		return nil, err
	}

	return cloneAgentProfiles(s.agents), nil
}

func (s *MockGatewayService) ListSkills(ctx context.Context) ([]Skill, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	skills := make([]Skill, len(s.skills))
	copy(skills, s.skills)
	return skills, nil
}

func (s *MockGatewayService) GetManagementSettings(ctx context.Context) (ManagementSettings, error) {
	if err := ctx.Err(); err != nil {
		return ManagementSettings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureManagementLoadedLocked(ctx); err != nil {
		return ManagementSettings{}, err
	}
	return cloneManagementSettings(s.management), nil
}

func (s *MockGatewayService) UpdateManagementSettings(ctx context.Context, in ManagementSettings) (ManagementSettings, error) {
	if err := ctx.Err(); err != nil {
		return ManagementSettings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureManagementLoadedLocked(ctx); err != nil {
		return ManagementSettings{}, err
	}
	updated := normalizeManagementSettings(in)
	if err := s.managementStore.Save(ctx, updated); err != nil {
		return ManagementSettings{}, err
	}
	s.management = updated
	s.agents = toAgentProfiles(s.management.Agents)
	return cloneManagementSettings(s.management), nil
}

func (s *MockGatewayService) ensureManagementLoadedLocked(ctx context.Context) error {
	if s.managementLoaded {
		return nil
	}
	if s.managementStore == nil {
		s.management = normalizeManagementSettings(s.management)
		s.agents = toAgentProfiles(s.management.Agents)
		s.managementLoaded = true
		return nil
	}

	settings, err := s.managementStore.Load(ctx)
	if err != nil {
		if !errors.Is(err, ErrManagementSettingsNotFound) {
			return err
		}
		seed := normalizeManagementSettings(ManagementSettings{
			MCPServers:    []MCPServerConfig{},
			ScheduleTasks: []ScheduleTaskConfig{},
			Agents:        toAgentConfigs(s.bootstrapAgents),
		})
		if err := s.managementStore.Save(ctx, seed); err != nil {
			return err
		}
		s.management = seed
		s.agents = toAgentProfiles(s.management.Agents)
		s.managementLoaded = true
		return nil
	}

	s.management = normalizeManagementSettings(settings)
	s.agents = toAgentProfiles(s.management.Agents)
	s.managementLoaded = true
	return nil
}

func normalizeManagementSettings(in ManagementSettings) ManagementSettings {
	out := ManagementSettings{
		MCPServers:    make([]MCPServerConfig, 0, len(in.MCPServers)),
		ScheduleTasks: make([]ScheduleTaskConfig, 0, len(in.ScheduleTasks)),
		Agents:        make([]AgentConfig, 0, len(in.Agents)),
	}
	for index, item := range in.MCPServers {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = fmt.Sprintf("mcp_%d", index+1)
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = id
		}
		args := make([]string, 0, len(item.Args))
		for _, arg := range item.Args {
			text := strings.TrimSpace(arg)
			if text == "" {
				continue
			}
			args = append(args, text)
		}
		out.MCPServers = append(out.MCPServers, MCPServerConfig{
			ID: id, Name: name, Command: strings.TrimSpace(item.Command), Args: args, Enabled: item.Enabled,
		})
	}
	for index, item := range in.ScheduleTasks {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = fmt.Sprintf("task_%d", index+1)
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = id
		}
		spec := strings.TrimSpace(item.Spec)
		if spec == "" {
			spec = "*/5 * * * *"
		}
		args := make([]string, 0, len(item.Args))
		for _, arg := range item.Args {
			text := strings.TrimSpace(arg)
			if text == "" {
				continue
			}
			args = append(args, text)
		}
		out.ScheduleTasks = append(out.ScheduleTasks, ScheduleTaskConfig{
			ID: id, Name: name, Spec: spec, Command: strings.TrimSpace(item.Command), Args: args, Enabled: item.Enabled,
		})
	}
	for index, item := range in.Agents {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = fmt.Sprintf("agent_%d", index+1)
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = id
		}
		models := make([]string, 0, len(item.Models))
		for _, model := range item.Models {
			text := strings.TrimSpace(model)
			if text == "" {
				continue
			}
			models = append(models, text)
		}
		out.Agents = append(out.Agents, AgentConfig{
			ID: id, Name: name, Protocol: strings.TrimSpace(item.Protocol), Description: strings.TrimSpace(item.Description), Models: models, Enabled: item.Enabled,
		})
	}
	return out
}

func cloneManagementSettings(in ManagementSettings) ManagementSettings {
	out := ManagementSettings{
		MCPServers:    make([]MCPServerConfig, 0, len(in.MCPServers)),
		ScheduleTasks: make([]ScheduleTaskConfig, 0, len(in.ScheduleTasks)),
		Agents:        make([]AgentConfig, 0, len(in.Agents)),
	}
	out.MCPServers = append(out.MCPServers, in.MCPServers...)
	out.ScheduleTasks = append(out.ScheduleTasks, in.ScheduleTasks...)
	for _, item := range in.Agents {
		cp := item
		cp.Models = append([]string(nil), item.Models...)
		out.Agents = append(out.Agents, cp)
	}
	for i := range out.MCPServers {
		out.MCPServers[i].Args = append([]string(nil), out.MCPServers[i].Args...)
	}
	for i := range out.ScheduleTasks {
		out.ScheduleTasks[i].Args = append([]string(nil), out.ScheduleTasks[i].Args...)
	}
	return out
}

func toAgentConfigs(profiles []AgentProfile) []AgentConfig {
	out := make([]AgentConfig, 0, len(profiles))
	for _, item := range profiles {
		out = append(out, AgentConfig{
			ID:          strings.TrimSpace(item.ID),
			Name:        strings.TrimSpace(item.Name),
			Protocol:    strings.TrimSpace(item.Protocol),
			Description: strings.TrimSpace(item.Description),
			Models:      append([]string(nil), item.Models...),
			Enabled:     true,
		})
	}
	return out
}

func toAgentProfiles(configs []AgentConfig) []AgentProfile {
	out := make([]AgentProfile, 0, len(configs))
	for _, item := range configs {
		if !item.Enabled {
			continue
		}
		out = append(out, AgentProfile{
			ID:          strings.TrimSpace(item.ID),
			Name:        strings.TrimSpace(item.Name),
			Protocol:    strings.TrimSpace(item.Protocol),
			Description: strings.TrimSpace(item.Description),
			Models:      append([]string(nil), item.Models...),
		})
	}
	return out
}

func cloneAgentProfiles(in []AgentProfile) []AgentProfile {
	out := make([]AgentProfile, 0, len(in))
	for _, item := range in {
		cp := item
		cp.Models = append([]string(nil), item.Models...)
		cp.Args = append([]string(nil), item.Args...)
		out = append(out, cp)
	}
	return out
}

func (s *MockGatewayService) CreateSession(ctx context.Context, req CreateSessionRequest) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureManagementLoadedLocked(ctx); err != nil {
		return Session{}, err
	}

	agentID := strings.TrimSpace(req.AgentID)
	mode := strings.TrimSpace(req.Mode)
	if agentID == "" && mode != "" && !isGenericMode(mode) {
		agentID = mode
	}
	if agentID == "" {
		if len(s.agents) == 0 {
			return Session{}, NewError("agent_not_found", "no enabled agents configured")
		}
		agentID = s.agents[0].ID
	}
	if mode == "" {
		mode = agentID
	}
	if !s.hasAgentLocked(agentID) {
		return Session{}, NewError("agent_not_found", fmt.Sprintf("agent %q was not found", agentID))
	}

	now := s.now()
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "New Agent Session"
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	additionalDirectories := append([]string(nil), req.AdditionalDirectories...)
	startupCommand := strings.TrimSpace(req.StartupCommand)
	sessionID := s.idLocked("sess")
	if s.connector != nil {
		metadata := map[string]any{}
		if startupCommand != "" {
			metadata["startupCommand"] = startupCommand
		}
		if workspaceID != "" {
			metadata["workspaceId"] = workspaceID
		}
		if mode != "" {
			metadata["mode"] = mode
		}
		if len(additionalDirectories) > 0 {
			metadata["additional_directories"] = append([]string(nil), additionalDirectories...)
		}
		if len(metadata) == 0 {
			metadata = nil
		}
		connResp, err := s.connector.NewSession(ctx, connector.NewSessionRequest{
			AgentID:  agentID,
			Model:    req.Model,
			CWD:      req.CWD,
			Metadata: metadata,
		})
		if err != nil {
			return Session{}, NewError("connector_request_failed", fmt.Sprintf("connector newSession failed: %v", err))
		}
		if strings.TrimSpace(connResp.SessionID) != "" {
			sessionID = strings.TrimSpace(connResp.SessionID)
		}
	}
	session := Session{
		ID:                    sessionID,
		Title:                 title,
		WorkspaceID:           workspaceID,
		CWD:                   req.CWD,
		AdditionalDirectories: append([]string(nil), additionalDirectories...),
		StartupCommand:        startupCommand,
		Mode:                  mode,
		AgentID:               agentID,
		Model:                 req.Model,
		Status:                "active",
		CreatedAt:             now,
		UpdatedAt:             now,
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
	if s.connector != nil {
		if err := s.syncSessionsFromConnector(ctx); err != nil {
			return nil, err
		}
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

func (s *MockGatewayService) GetSession(ctx context.Context, sessionID string) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	conversation, ok, err := s.store.GetConversation(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if !ok {
		return Session{}, NewError("session_not_found", fmt.Sprintf("session %q was not found", sessionID))
	}
	return fromStoreConversation(conversation), nil
}

func (s *MockGatewayService) ResumeSession(ctx context.Context, sessionID string, req ResumeSessionRequest) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureManagementLoadedLocked(ctx); err != nil {
		return Session{}, err
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Session{}, NewError("invalid_session", "session id is required")
	}
	if s.connector == nil {
		return Session{}, NewError("connector_unavailable", "connector is not configured")
	}

	conversation, ok, err := s.store.GetConversation(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	var session Session
	if ok {
		session = fromStoreConversation(conversation)
	} else {
		now := s.now()
		agentID := "icoo-ai-acp"
		if len(s.agents) > 0 {
			agentID = s.agents[0].ID
		}
		session = Session{
			ID:        sessionID,
			Title:     "Resumed Session",
			AgentID:   agentID,
			Mode:      agentID,
			Status:    "active",
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	cwd := strings.TrimSpace(req.CWD)
	if cwd == "" {
		cwd = strings.TrimSpace(session.CWD)
	}
	if cwd == "" {
		return Session{}, NewError("invalid_session_config", "cwd is required to resume session")
	}
	connReq := connector.ResumeSessionRequest{
		SessionID:             sessionID,
		CWD:                   cwd,
		AdditionalDirectories: append([]string(nil), req.AdditionalDirectories...),
	}
	if _, err := s.connector.ResumeSession(ctx, connReq); err != nil {
		return Session{}, NewError("connector_request_failed", fmt.Sprintf("connector resumeSession failed: %v", err))
	}

	session.CWD = cwd
	if len(req.AdditionalDirectories) > 0 {
		session.AdditionalDirectories = append([]string(nil), req.AdditionalDirectories...)
	}
	if session.Status == "" {
		session.Status = "active"
	}
	session.UpdatedAt = s.now()
	if err := s.store.UpsertConversation(ctx, toStoreConversation(session)); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *MockGatewayService) CloseSession(ctx context.Context, sessionID string) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	conversation, ok, err := s.store.GetConversation(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if !ok {
		return Session{}, NewError("session_not_found", fmt.Sprintf("session %q was not found", sessionID))
	}
	session := fromStoreConversation(conversation)
	if s.connector != nil {
		if _, err := s.connector.CloseSession(ctx, connector.CloseSessionRequest{SessionID: sessionID}); err != nil {
			return Session{}, NewError("connector_request_failed", fmt.Sprintf("connector closeSession failed: %v", err))
		}
	}
	now := s.now()
	session.Status = "closed"
	session.UpdatedAt = now
	if err := s.store.UpsertConversation(ctx, toStoreConversation(session)); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *MockGatewayService) SetSessionMode(ctx context.Context, sessionID string, req SetSessionModeRequest) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureManagementLoadedLocked(ctx); err != nil {
		return Session{}, err
	}

	conversation, ok, err := s.store.GetConversation(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if !ok {
		return Session{}, NewError("session_not_found", fmt.Sprintf("session %q was not found", sessionID))
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		return Session{}, NewError("invalid_session_config", "mode is required")
	}
	if s.connector != nil {
		if _, err := s.connector.SetSessionMode(ctx, connector.SetSessionModeRequest{
			SessionID: sessionID,
			ModeID:    mode,
		}); err != nil {
			return Session{}, NewError("connector_request_failed", fmt.Sprintf("connector setSessionMode failed: %v", err))
		}
	}
	session := fromStoreConversation(conversation)
	session.Mode = mode
	session.UpdatedAt = s.now()
	if !isGenericMode(mode) && s.hasAgentLocked(mode) {
		session.AgentID = mode
	}
	if err := s.store.UpsertConversation(ctx, toStoreConversation(session)); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *MockGatewayService) SetSessionConfigOption(ctx context.Context, sessionID string, req SetSessionConfigOptionRequest) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	conversation, ok, err := s.store.GetConversation(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if !ok {
		return Session{}, NewError("session_not_found", fmt.Sprintf("session %q was not found", sessionID))
	}
	configID := strings.TrimSpace(req.ConfigID)
	if configID == "" {
		return Session{}, NewError("invalid_session_config", "configId is required")
	}
	if req.BooleanValue == nil && strings.TrimSpace(req.ValueID) == "" {
		return Session{}, NewError("invalid_session_config", "booleanValue or valueId is required")
	}
	if s.connector != nil {
		if _, err := s.connector.SetSessionConfigOption(ctx, connector.SetSessionConfigOptionRequest{
			SessionID:    sessionID,
			ConfigID:     configID,
			BooleanValue: req.BooleanValue,
			ValueID:      strings.TrimSpace(req.ValueID),
		}); err != nil {
			return Session{}, NewError("connector_request_failed", fmt.Sprintf("connector setSessionConfigOption failed: %v", err))
		}
	}
	session := fromStoreConversation(conversation)
	session.UpdatedAt = s.now()
	if err := s.store.UpsertConversation(ctx, toStoreConversation(session)); err != nil {
		return Session{}, err
	}
	return session, nil
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
	if err := s.ensureManagementLoadedLocked(ctx); err != nil {
		return PromptResponse{}, err
	}

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
	if workspaceID := strings.TrimSpace(req.WorkspaceID); workspaceID != "" {
		session.WorkspaceID = workspaceID
	}
	if cwd := strings.TrimSpace(req.CWD); cwd != "" {
		session.CWD = cwd
	}
	if mode := strings.TrimSpace(req.Mode); mode != "" {
		session.Mode = mode
		if !isGenericMode(mode) && s.hasAgentLocked(mode) {
			session.AgentID = mode
		}
	}
	if agentID := strings.TrimSpace(req.AgentID); agentID != "" {
		if !s.hasAgentLocked(agentID) {
			return PromptResponse{}, NewError("agent_not_found", fmt.Sprintf("agent %q was not found", agentID))
		}
		session.AgentID = agentID
	}
	if model := strings.TrimSpace(req.Model); model != "" {
		session.Model = model
	}
	if session.Mode == "" {
		session.Mode = session.AgentID
	}
	if s.connector != nil {
		return s.promptViaConnectorLocked(ctx, session, content)
	}
	return PromptResponse{}, NewError("connector_unavailable", "connector is not configured")
}

func (s *MockGatewayService) promptViaConnectorLocked(ctx context.Context, session Session, content string) (PromptResponse, error) {
	startedAt := s.now().UTC()
	connReqID := s.idLocked("connreq")
	connResp, err := s.connector.Prompt(ctx, connector.PromptRequest{
		SessionID: session.ID,
		Content:   content,
		RequestID: connReqID,
	})
	if err != nil {
		return PromptResponse{}, NewError("connector_request_failed", fmt.Sprintf("connector prompt failed: %v", err))
	}

	runID := strings.TrimSpace(connResp.RunID)
	if runID == "" {
		runID = s.idLocked("run")
	}
	runStatus := "completed"
	if connResp.EndedAt == nil {
		runStatus = "running"
	}
	run := Run{
		ID:        runID,
		SessionID: session.ID,
		AgentID:   session.AgentID,
		Status:    runStatus,
		StartedAt: startedAt,
		EndedAt:   connResp.EndedAt,
	}
	userMessage := Message{
		ID:        s.idLocked("msg"),
		SessionID: session.ID,
		RunID:     run.ID,
		Role:      "user",
		Content:   content,
		CreatedAt: startedAt,
	}

	responseMessages := []Message{userMessage}
	if output := strings.TrimSpace(connResp.Output); output != "" {
		assistantMessage := Message{
			ID:        s.idLocked("msg"),
			SessionID: session.ID,
			RunID:     run.ID,
			Role:      "assistant",
			Content:   output,
			CreatedAt: timePointerValue(connResp.EndedAt, startedAt),
		}
		responseMessages = append(responseMessages, assistantMessage)
	}

	if err := s.store.UpsertRun(ctx, toStoreRun(run)); err != nil {
		return PromptResponse{}, err
	}
	for _, message := range responseMessages {
		if err := s.store.AppendMessage(ctx, toStoreMessageEvent(message)); err != nil {
			return PromptResponse{}, err
		}
	}

	var firstApproval *Approval
	for _, item := range connResp.Approvals {
		requestID := strings.TrimSpace(item.RequestID)
		if requestID == "" {
			requestID = s.idLocked("connreq")
		}
		approval := Approval{
			ID:                 s.idLocked("appr"),
			AgentID:            session.AgentID,
			SessionID:          session.ID,
			RunID:              run.ID,
			ConnectorRequestID: requestID,
			Status:             "pending",
			Action:             item.Action,
			Message:            item.Message,
			CreatedAt:          startedAt,
		}
		if err := s.store.UpsertApproval(ctx, toStoreApproval(approval)); err != nil {
			return PromptResponse{}, err
		}
		if s.approvalBroker != nil {
			if err := s.approvalBroker.Register(approval); err != nil {
				return PromptResponse{}, err
			}
		}
		if firstApproval == nil {
			cp := approval
			firstApproval = &cp
		}
	}

	session.UpdatedAt = timePointerValue(connResp.EndedAt, startedAt)
	if err := s.store.UpsertConversation(ctx, toStoreConversation(session)); err != nil {
		return PromptResponse{}, err
	}

	return PromptResponse{
		Run:      run,
		Messages: responseMessages,
		Approval: firstApproval,
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
	runID := s.idLocked("run")
	status := "cancelled"
	if s.connector != nil {
		lastRunID, err := s.latestRunIDLocked(ctx, sessionID)
		if err != nil {
			return Run{}, err
		}
		if lastRunID == "" {
			lastRunID = runID
		}
		connResp, err := s.connector.Cancel(ctx, connector.CancelRequest{
			SessionID: sessionID,
			RunID:     lastRunID,
			Reason:    "user_cancelled",
		})
		if err != nil {
			return Run{}, NewError("connector_request_failed", fmt.Sprintf("connector cancel failed: %v", err))
		}
		if strings.TrimSpace(connResp.RunID) != "" {
			runID = strings.TrimSpace(connResp.RunID)
		} else {
			runID = lastRunID
		}
		if strings.TrimSpace(connResp.Status) != "" {
			status = strings.TrimSpace(connResp.Status)
		}
	}
	run := Run{
		ID:        runID,
		SessionID: sessionID,
		AgentID:   session.AgentID,
		Status:    status,
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
	approvalMap := make(map[string]Approval, len(storedApprovals))
	for _, storedApproval := range storedApprovals {
		approval := fromStoreApproval(storedApproval)
		approvalMap[approval.ID] = approval
	}
	if s.approvalBroker != nil {
		for _, approval := range approvalMap {
			if approval.Status == "pending" && approval.SessionID == sessionID {
				_ = s.approvalBroker.Register(approval)
			}
		}
		_ = s.approvalBroker.ExpirePendingBySession(sessionID, approvalMap, now)
	} else {
		for id, approval := range approvalMap {
			if approval.SessionID != sessionID || approval.Status != "pending" {
				continue
			}
			approval.Status = "expired"
			approval.Decision = "rejected"
			approval.DecidedAt = &now
			approval.Message = "Approval expired because session was cancelled."
			approvalMap[id] = approval
		}
	}
	for _, approval := range approvalMap {
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
	approvalMap := make(map[string]Approval, len(storedApprovals))
	for _, storedApproval := range storedApprovals {
		current := fromStoreApproval(storedApproval)
		approvalMap[current.ID] = current
		if current.ID == approvalID {
			approval = current
			found = true
			break
		}
	}
	if !found {
		return Approval{}, NewError("approval_not_found", fmt.Sprintf("approval %q was not found", approvalID))
	}
	now := s.now()
	if s.approvalBroker != nil {
		for _, item := range approvalMap {
			if item.Status == "pending" {
				_ = s.approvalBroker.Register(item)
			}
		}
		updated, err := s.approvalBroker.Decide(approvalID, req, approvalMap, now)
		if err != nil {
			return Approval{}, err
		}
		approval = updated
	} else {
		if approval.Status != "pending" {
			return Approval{}, NewError("invalid_decision", "approval is no longer pending")
		}
		decision, err := normalizeDecision(req.Decision)
		if err != nil {
			return Approval{}, err
		}
		approval.Status = decision
		approval.Decision = decision
		approval.DecidedAt = &now
		if strings.TrimSpace(req.Message) != "" {
			approval.Message = req.Message
		}
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

func (s *MockGatewayService) latestRunIDLocked(ctx context.Context, sessionID string) (string, error) {
	runs, err := s.store.ListRuns(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return "", nil
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.Before(runs[j].CreatedAt)
	})
	return runs[len(runs)-1].RunID, nil
}

func (s *MockGatewayService) syncSessionsFromConnector(ctx context.Context) error {
	resp, err := s.connector.ListSessions(ctx, connector.ListSessionsRequest{})
	if err != nil {
		return NewError("connector_request_failed", fmt.Sprintf("connector listSessions failed: %v", err))
	}
	defaultAgentID, err := s.defaultAgentID(ctx)
	if err != nil {
		return err
	}
	now := s.now()
	for _, item := range resp.Sessions {
		sessionID := strings.TrimSpace(item.SessionID)
		if sessionID == "" {
			continue
		}
		conversation, ok, err := s.store.GetConversation(ctx, sessionID)
		if err != nil {
			return err
		}
		var session Session
		if ok {
			session = fromStoreConversation(conversation)
		} else {
			agentID := defaultAgentID
			session = Session{
				ID:        sessionID,
				Title:     "Restored Session",
				AgentID:   agentID,
				Mode:      agentID,
				Status:    "active",
				CreatedAt: now,
			}
		}
		if title := strings.TrimSpace(item.Title); title != "" {
			session.Title = title
		}
		if cwd := strings.TrimSpace(item.CWD); cwd != "" {
			session.CWD = cwd
		}
		if len(item.AdditionalDirectories) > 0 {
			session.AdditionalDirectories = append([]string(nil), item.AdditionalDirectories...)
		}
		if strings.TrimSpace(session.Status) == "" {
			session.Status = "active"
		}
		session.UpdatedAt = now
		if err := s.store.UpsertConversation(ctx, toStoreConversation(session)); err != nil {
			return err
		}
	}
	return nil
}

func (s *MockGatewayService) defaultAgentID(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureManagementLoadedLocked(ctx); err != nil {
		return "", err
	}
	if len(s.agents) > 0 {
		return s.agents[0].ID, nil
	}
	return "icoo-ai-acp", nil
}

func (s *MockGatewayService) idLocked(prefix string) string {
	id := fmt.Sprintf("%s_%06d", prefix, s.nextID)
	s.nextID++
	return id
}

func toStoreConversation(session Session) store.Conversation {
	safeMeta := store.SafeMeta{}
	if strings.TrimSpace(session.StartupCommand) != "" {
		safeMeta["startupCommand"] = strings.TrimSpace(session.StartupCommand)
	}
	if strings.TrimSpace(session.WorkspaceID) != "" {
		safeMeta["workspaceId"] = strings.TrimSpace(session.WorkspaceID)
	}
	if strings.TrimSpace(session.Mode) != "" {
		safeMeta["mode"] = strings.TrimSpace(session.Mode)
	}
	if len(session.AdditionalDirectories) > 0 {
		safeMeta["additionalDirectories"] = append([]string(nil), session.AdditionalDirectories...)
	}
	if len(safeMeta) == 0 {
		safeMeta = nil
	}
	return store.Conversation{
		ID:        session.ID,
		AgentID:   session.AgentID,
		SessionID: session.ID,
		Title:     session.Title,
		Status:    session.Status,
		Model:     session.Model,
		CWD:       session.CWD,
		SafeMeta:  safeMeta,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}
}

func fromStoreConversation(conversation store.Conversation) Session {
	startupCommand, _ := conversation.SafeMeta["startupCommand"].(string)
	workspaceID, _ := conversation.SafeMeta["workspaceId"].(string)
	mode, _ := conversation.SafeMeta["mode"].(string)
	additionalDirectories := stringSliceMeta(conversation.SafeMeta["additionalDirectories"])
	if strings.TrimSpace(mode) == "" {
		mode = conversation.AgentID
	}
	return Session{
		ID:                    conversation.SessionID,
		Title:                 conversation.Title,
		WorkspaceID:           strings.TrimSpace(workspaceID),
		CWD:                   conversation.CWD,
		AdditionalDirectories: additionalDirectories,
		StartupCommand:        strings.TrimSpace(startupCommand),
		Mode:                  strings.TrimSpace(mode),
		AgentID:               conversation.AgentID,
		Model:                 conversation.Model,
		Status:                conversation.Status,
		CreatedAt:             conversation.CreatedAt,
		UpdatedAt:             conversation.UpdatedAt,
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

func stringSliceMeta(raw any) []string {
	switch value := raw.(type) {
	case []string:
		return append([]string(nil), value...)
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				continue
			}
			out = append(out, text)
		}
		return out
	default:
		return nil
	}
}

func isGenericMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "agent", "default", "main":
		return true
	default:
		return false
	}
}
