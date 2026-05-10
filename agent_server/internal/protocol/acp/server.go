package acp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	sdk "github.com/coder/acp-go-sdk"
	"github.com/icoo-ai/icoo-ai/internal/agent"
)

const (
	defaultSessionModeID sdk.SessionModeId = "agent"

	configApprovalModeID    sdk.SessionConfigId      = "approval_mode"
	configEmitPlanUpdatesID sdk.SessionConfigId      = "emit_plan_updates"
	defaultApprovalMode     sdk.SessionConfigValueId = "workspace-write"
)

var supportedApprovalModes = map[sdk.SessionConfigValueId]struct{}{
	sdk.SessionConfigValueId("readonly"):        {},
	sdk.SessionConfigValueId("suggest"):         {},
	sdk.SessionConfigValueId("workspace-write"): {},
	sdk.SessionConfigValueId("full-auto"):       {},
}

type ServerOptions struct {
	Runtime agent.Runtime
	Input   io.Reader
	Output  io.Writer
	Logger  *slog.Logger
	Name    string
	Version string
}

type Server struct {
	runtime agent.Runtime
	input   io.Reader
	output  io.Writer
	logger  *slog.Logger
	name    string
	version string
}

func NewServer(opts ServerOptions) (*Server, error) {
	if opts.Runtime == nil {
		return nil, errors.New("acp server requires runtime")
	}
	if opts.Input == nil {
		return nil, errors.New("acp server requires input")
	}
	if opts.Output == nil {
		return nil, errors.New("acp server requires output")
	}
	return &Server{
		runtime: opts.Runtime,
		input:   opts.Input,
		output:  opts.Output,
		logger:  opts.Logger,
		name:    opts.Name,
		version: opts.Version,
	}, nil
}

func (s *Server) Serve() error {
	adapter := newAgentAdapter(s.runtime, CapabilitiesOptions{Name: s.name, Version: s.version})
	conn := sdk.NewAgentSideConnection(adapter, s.output, s.input)
	if s.logger != nil {
		conn.SetLogger(s.logger)
	}
	adapter.setConnection(conn)
	<-conn.Done()
	return nil
}

type adapterSessionState struct {
	ModeID                sdk.SessionModeId
	ApprovalMode          sdk.SessionConfigValueId
	EmitPlanUpdates       bool
	AdditionalDirectories []string
	CWD                   string
}

type agentAdapter struct {
	runtime      agent.Runtime
	capabilities CapabilitiesOptions
	conn         sessionUpdater

	mu            sync.RWMutex
	sessionStates map[string]adapterSessionState
}

type sessionUpdater interface {
	SessionUpdate(ctx context.Context, params sdk.SessionNotification) error
}

type runtimeSessionLister interface {
	ListSessions(ctx context.Context) ([]agent.Session, error)
}

type runtimeSessionUpdater interface {
	UpdateSession(ctx context.Context, session agent.Session) error
}

type runtimeSessionCloser interface {
	CloseSession(ctx context.Context, sessionID string) error
}

var _ sdk.Agent = (*agentAdapter)(nil)

func newAgentAdapter(runtime agent.Runtime, capabilities CapabilitiesOptions) *agentAdapter {
	return &agentAdapter{
		runtime:       runtime,
		capabilities:  capabilities,
		sessionStates: map[string]adapterSessionState{},
	}
}

func (a *agentAdapter) setConnection(conn sessionUpdater) {
	a.conn = conn
}

func (a *agentAdapter) Authenticate(ctx context.Context, params sdk.AuthenticateRequest) (sdk.AuthenticateResponse, error) {
	return sdk.AuthenticateResponse{}, nil
}

func (a *agentAdapter) Initialize(ctx context.Context, params sdk.InitializeRequest) (sdk.InitializeResponse, error) {
	return InitializeResponse(a.capabilities), nil
}

func (a *agentAdapter) Cancel(ctx context.Context, params sdk.CancelNotification) error {
	return a.runtime.Cancel(ctx, string(params.SessionId))
}

func (a *agentAdapter) CloseSession(ctx context.Context, params sdk.CloseSessionRequest) (sdk.CloseSessionResponse, error) {
	sessionID := strings.TrimSpace(string(params.SessionId))
	if sessionID == "" {
		return sdk.CloseSessionResponse{}, sdk.NewInvalidParams("sessionId is required")
	}
	if _, err := a.runtime.LoadSession(ctx, sessionID); err != nil {
		return sdk.CloseSessionResponse{}, sdk.NewInvalidParams(err.Error())
	}
	if err := a.runtime.Cancel(ctx, sessionID); err != nil {
		return sdk.CloseSessionResponse{}, err
	}
	if closer, ok := a.runtime.(runtimeSessionCloser); ok {
		if err := closer.CloseSession(ctx, sessionID); err != nil {
			return sdk.CloseSessionResponse{}, err
		}
	}
	a.deleteSessionState(sessionID)
	return sdk.CloseSessionResponse{}, nil
}

func (a *agentAdapter) ListSessions(ctx context.Context, params sdk.ListSessionsRequest) (sdk.ListSessionsResponse, error) {
	lister, ok := a.runtime.(runtimeSessionLister)
	if !ok {
		return sdk.ListSessionsResponse{}, sdk.NewMethodNotFound(sdk.AgentMethodSessionList)
	}
	if params.Cursor != nil {
		return sdk.ListSessionsResponse{}, sdk.NewInvalidParams("cursor pagination is not supported")
	}

	sessions, err := lister.ListSessions(ctx)
	if err != nil {
		return sdk.ListSessionsResponse{}, err
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	var cwdFilter string
	if params.Cwd != nil {
		cwdFilter = strings.TrimSpace(*params.Cwd)
	}
	infos := make([]sdk.SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		state := a.sessionStateOrDefault(session.ID, session.CWD)
		cwd := strings.TrimSpace(state.CWD)
		if cwd == "" {
			cwd = strings.TrimSpace(session.CWD)
		}
		if cwdFilter != "" && cwd != cwdFilter {
			continue
		}
		if len(params.AdditionalDirectories) > 0 && !sameOrderedStrings(state.AdditionalDirectories, params.AdditionalDirectories) {
			continue
		}
		info := sdk.SessionInfo{
			SessionId:             sdk.SessionId(session.ID),
			Cwd:                   cwd,
			AdditionalDirectories: append([]string(nil), state.AdditionalDirectories...),
		}
		if !session.UpdatedAt.IsZero() {
			value := session.UpdatedAt.UTC().Format(time.RFC3339)
			info.UpdatedAt = &value
		}
		infos = append(infos, info)
	}
	return sdk.ListSessionsResponse{Sessions: infos}, nil
}

func (a *agentAdapter) NewSession(ctx context.Context, params sdk.NewSessionRequest) (sdk.NewSessionResponse, error) {
	session, err := a.runtime.NewSession(ctx, mapNewSessionRequest(params))
	if err != nil {
		return sdk.NewSessionResponse{}, err
	}
	state := defaultAdapterSessionState(session.CWD)
	if cwd := strings.TrimSpace(params.Cwd); cwd != "" {
		state.CWD = cwd
	}
	if len(params.AdditionalDirectories) > 0 {
		state.AdditionalDirectories = append([]string(nil), params.AdditionalDirectories...)
	}
	a.setSessionState(session.ID, state)

	return sdk.NewSessionResponse{
		SessionId:     sdk.SessionId(session.ID),
		ConfigOptions: configOptionsFromState(state),
		Modes:         modeStateFromSession(state),
	}, nil
}

func (a *agentAdapter) Prompt(ctx context.Context, params sdk.PromptRequest) (sdk.PromptResponse, error) {
	if a.conn == nil {
		return sdk.PromptResponse{}, errors.New("acp connection is not initialized")
	}
	request := mapPromptRequest(params)
	a.applySessionStateToPrompt(&request)

	events, err := a.runtime.Prompt(ctx, request)
	if err != nil {
		return sdk.PromptResponse{}, err
	}

	stopReason := sdk.StopReasonEndTurn
	for event := range events {
		if update, ok := mapSessionEvent(event); ok {
			if err := a.conn.SessionUpdate(ctx, sdk.SessionNotification{
				SessionId: params.SessionId,
				Update:    update,
			}); err != nil {
				return sdk.PromptResponse{}, err
			}
		}
		if reason, ok := stopReasonForEvent(event); ok {
			stopReason = reason
		}
	}
	if err := ctx.Err(); err != nil {
		return sdk.PromptResponse{StopReason: sdk.StopReasonCancelled}, nil
	}
	return sdk.PromptResponse{StopReason: stopReason, UserMessageId: params.MessageId}, nil
}

func (a *agentAdapter) ResumeSession(ctx context.Context, params sdk.ResumeSessionRequest) (sdk.ResumeSessionResponse, error) {
	sessionID := strings.TrimSpace(string(params.SessionId))
	if sessionID == "" {
		return sdk.ResumeSessionResponse{}, sdk.NewInvalidParams("sessionId is required")
	}
	session, err := a.runtime.LoadSession(ctx, sessionID)
	if err != nil {
		return sdk.ResumeSessionResponse{}, sdk.NewInvalidParams(err.Error())
	}

	state := a.sessionStateOrDefault(sessionID, session.CWD)
	if len(params.AdditionalDirectories) > 0 {
		state.AdditionalDirectories = append([]string(nil), params.AdditionalDirectories...)
	}
	if cwd := strings.TrimSpace(params.Cwd); cwd != "" {
		state.CWD = cwd
		if updater, ok := a.runtime.(runtimeSessionUpdater); ok && session.CWD != cwd {
			session.CWD = cwd
			if err := updater.UpdateSession(ctx, session); err != nil {
				return sdk.ResumeSessionResponse{}, err
			}
		}
	}
	a.setSessionState(sessionID, state)

	return sdk.ResumeSessionResponse{
		ConfigOptions: configOptionsFromState(state),
		Modes:         modeStateFromSession(state),
	}, nil
}

func (a *agentAdapter) SetSessionConfigOption(ctx context.Context, params sdk.SetSessionConfigOptionRequest) (sdk.SetSessionConfigOptionResponse, error) {
	switch {
	case params.Boolean != nil:
		return a.setSessionConfigBoolean(ctx, *params.Boolean)
	case params.ValueId != nil:
		return a.setSessionConfigValue(ctx, *params.ValueId)
	default:
		return sdk.SetSessionConfigOptionResponse{}, sdk.NewInvalidParams("setSessionConfigOption payload is empty")
	}
}

func (a *agentAdapter) SetSessionMode(ctx context.Context, params sdk.SetSessionModeRequest) (sdk.SetSessionModeResponse, error) {
	sessionID := strings.TrimSpace(string(params.SessionId))
	if sessionID == "" {
		return sdk.SetSessionModeResponse{}, sdk.NewInvalidParams("sessionId is required")
	}
	if _, err := a.runtime.LoadSession(ctx, sessionID); err != nil {
		return sdk.SetSessionModeResponse{}, sdk.NewInvalidParams(err.Error())
	}
	modeID := strings.TrimSpace(string(params.ModeId))
	if modeID == "" {
		return sdk.SetSessionModeResponse{}, sdk.NewInvalidParams("modeId is required")
	}
	if !isSupportedModeID(sdk.SessionModeId(modeID)) {
		return sdk.SetSessionModeResponse{}, sdk.NewInvalidParams("unsupported modeId: " + modeID)
	}
	state := a.sessionStateOrDefault(sessionID, "")
	state.ModeID = sdk.SessionModeId(modeID)
	a.setSessionState(sessionID, state)
	return sdk.SetSessionModeResponse{}, nil
}

func (a *agentAdapter) setSessionConfigBoolean(ctx context.Context, req sdk.SetSessionConfigOptionBoolean) (sdk.SetSessionConfigOptionResponse, error) {
	sessionID := strings.TrimSpace(string(req.SessionId))
	if sessionID == "" {
		return sdk.SetSessionConfigOptionResponse{}, sdk.NewInvalidParams("sessionId is required")
	}
	if _, err := a.runtime.LoadSession(ctx, sessionID); err != nil {
		return sdk.SetSessionConfigOptionResponse{}, sdk.NewInvalidParams(err.Error())
	}
	configID := strings.TrimSpace(string(req.ConfigId))
	if configID == "" {
		return sdk.SetSessionConfigOptionResponse{}, sdk.NewInvalidParams("configId is required")
	}

	state := a.sessionStateOrDefault(sessionID, "")
	switch sdk.SessionConfigId(configID) {
	case configEmitPlanUpdatesID:
		state.EmitPlanUpdates = req.Value
	default:
		return sdk.SetSessionConfigOptionResponse{}, sdk.NewInvalidParams("unsupported boolean configId: " + configID)
	}
	a.setSessionState(sessionID, state)
	return sdk.SetSessionConfigOptionResponse{ConfigOptions: configOptionsFromState(state)}, nil
}

func (a *agentAdapter) setSessionConfigValue(ctx context.Context, req sdk.SetSessionConfigOptionValueId) (sdk.SetSessionConfigOptionResponse, error) {
	sessionID := strings.TrimSpace(string(req.SessionId))
	if sessionID == "" {
		return sdk.SetSessionConfigOptionResponse{}, sdk.NewInvalidParams("sessionId is required")
	}
	if _, err := a.runtime.LoadSession(ctx, sessionID); err != nil {
		return sdk.SetSessionConfigOptionResponse{}, sdk.NewInvalidParams(err.Error())
	}
	configID := strings.TrimSpace(string(req.ConfigId))
	if configID == "" {
		return sdk.SetSessionConfigOptionResponse{}, sdk.NewInvalidParams("configId is required")
	}
	value := sdk.SessionConfigValueId(strings.TrimSpace(string(req.Value)))
	if value == "" {
		return sdk.SetSessionConfigOptionResponse{}, sdk.NewInvalidParams("value is required")
	}

	state := a.sessionStateOrDefault(sessionID, "")
	switch sdk.SessionConfigId(configID) {
	case configApprovalModeID:
		if _, ok := supportedApprovalModes[value]; !ok {
			return sdk.SetSessionConfigOptionResponse{}, sdk.NewInvalidParams("unsupported approval_mode value: " + string(value))
		}
		state.ApprovalMode = value
	default:
		return sdk.SetSessionConfigOptionResponse{}, sdk.NewInvalidParams("unsupported configId: " + configID)
	}
	a.setSessionState(sessionID, state)
	return sdk.SetSessionConfigOptionResponse{ConfigOptions: configOptionsFromState(state)}, nil
}

func (a *agentAdapter) applySessionStateToPrompt(req *agent.PromptRequest) {
	if req == nil {
		return
	}
	state, ok := a.getSessionState(req.SessionID)
	if !ok {
		return
	}
	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}
	req.Metadata["session_mode"] = string(state.ModeID)
	req.Metadata["approval_mode"] = string(state.ApprovalMode)
	req.Metadata["emit_plan_updates"] = state.EmitPlanUpdates
	if len(state.AdditionalDirectories) > 0 {
		req.Metadata["additional_directories"] = append([]string(nil), state.AdditionalDirectories...)
	}
	if strings.TrimSpace(req.CWD) == "" && strings.TrimSpace(state.CWD) != "" {
		req.CWD = state.CWD
	}
}

func (a *agentAdapter) setSessionState(sessionID string, state adapterSessionState) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessionStates[sessionID] = normalizeAdapterSessionState(state)
}

func (a *agentAdapter) getSessionState(sessionID string) (adapterSessionState, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	state, ok := a.sessionStates[sessionID]
	return state, ok
}

func (a *agentAdapter) sessionStateOrDefault(sessionID string, fallbackCWD string) adapterSessionState {
	if state, ok := a.getSessionState(sessionID); ok {
		if strings.TrimSpace(state.CWD) == "" {
			state.CWD = strings.TrimSpace(fallbackCWD)
		}
		return normalizeAdapterSessionState(state)
	}
	state := defaultAdapterSessionState(fallbackCWD)
	a.setSessionState(sessionID, state)
	return state
}

func (a *agentAdapter) deleteSessionState(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessionStates, sessionID)
}

func normalizeAdapterSessionState(in adapterSessionState) adapterSessionState {
	out := adapterSessionState{
		ModeID:                in.ModeID,
		ApprovalMode:          in.ApprovalMode,
		EmitPlanUpdates:       in.EmitPlanUpdates,
		AdditionalDirectories: append([]string(nil), in.AdditionalDirectories...),
		CWD:                   strings.TrimSpace(in.CWD),
	}
	if strings.TrimSpace(string(out.ModeID)) == "" {
		out.ModeID = defaultSessionModeID
	}
	if strings.TrimSpace(string(out.ApprovalMode)) == "" {
		out.ApprovalMode = defaultApprovalMode
	}
	return out
}

func defaultAdapterSessionState(cwd string) adapterSessionState {
	return adapterSessionState{
		ModeID:          defaultSessionModeID,
		ApprovalMode:    defaultApprovalMode,
		EmitPlanUpdates: true,
		CWD:             strings.TrimSpace(cwd),
	}
}

func sameOrderedStrings(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func modeStateFromSession(state adapterSessionState) *sdk.SessionModeState {
	return &sdk.SessionModeState{
		CurrentModeId:  state.ModeID,
		AvailableModes: availableModes(),
	}
}

func availableModes() []sdk.SessionMode {
	return []sdk.SessionMode{
		{
			Id:   defaultSessionModeID,
			Name: "Agent",
		},
	}
}

func isSupportedModeID(modeID sdk.SessionModeId) bool {
	for _, mode := range availableModes() {
		if mode.Id == modeID {
			return true
		}
	}
	return false
}

func configOptionsFromState(state adapterSessionState) []sdk.SessionConfigOption {
	options := make([]sdk.SessionConfigOption, 0, 2)

	approvalChoices := sdk.SessionConfigSelectOptionsUngrouped{
		{Name: "readonly", Value: sdk.SessionConfigValueId("readonly")},
		{Name: "suggest", Value: sdk.SessionConfigValueId("suggest")},
		{Name: "workspace-write", Value: sdk.SessionConfigValueId("workspace-write")},
		{Name: "full-auto", Value: sdk.SessionConfigValueId("full-auto")},
	}
	approvalOption := sdk.NewSessionConfigOptionSelect(state.ApprovalMode, sdk.SessionConfigSelectOptions{
		Ungrouped: &approvalChoices,
	})
	if approvalOption.Select != nil {
		approvalOption.Select.Id = configApprovalModeID
		approvalOption.Select.Name = "Approval Mode"
		description := "Controls approval policy for tool execution."
		approvalOption.Select.Description = &description
	}
	options = append(options, approvalOption)

	planOption := sdk.NewSessionConfigOptionBoolean(state.EmitPlanUpdates)
	if planOption.Boolean != nil {
		planOption.Boolean.Id = configEmitPlanUpdatesID
		planOption.Boolean.Name = "Emit Plan Updates"
		description := "Whether plan updates are emitted during runs."
		planOption.Boolean.Description = &description
	}
	options = append(options, planOption)

	return options
}
