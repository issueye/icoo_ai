package acp

import (
	"context"
	"errors"
	"io"
	"log/slog"

	sdk "github.com/coder/acp-go-sdk"
	"github.com/icoo-ai/icoo-ai/internal/agent"
)

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

type agentAdapter struct {
	runtime      agent.Runtime
	capabilities CapabilitiesOptions
	conn         sessionUpdater
}

type sessionUpdater interface {
	SessionUpdate(ctx context.Context, params sdk.SessionNotification) error
}

var _ sdk.Agent = (*agentAdapter)(nil)

func newAgentAdapter(runtime agent.Runtime, capabilities CapabilitiesOptions) *agentAdapter {
	return &agentAdapter{runtime: runtime, capabilities: capabilities}
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
	return sdk.CloseSessionResponse{}, sdk.NewMethodNotFound(sdk.AgentMethodSessionClose)
}

func (a *agentAdapter) ListSessions(ctx context.Context, params sdk.ListSessionsRequest) (sdk.ListSessionsResponse, error) {
	return sdk.ListSessionsResponse{}, sdk.NewMethodNotFound(sdk.AgentMethodSessionList)
}

func (a *agentAdapter) NewSession(ctx context.Context, params sdk.NewSessionRequest) (sdk.NewSessionResponse, error) {
	session, err := a.runtime.NewSession(ctx, mapNewSessionRequest(params))
	if err != nil {
		return sdk.NewSessionResponse{}, err
	}
	return sdk.NewSessionResponse{SessionId: sdk.SessionId(session.ID)}, nil
}

func (a *agentAdapter) Prompt(ctx context.Context, params sdk.PromptRequest) (sdk.PromptResponse, error) {
	if a.conn == nil {
		return sdk.PromptResponse{}, errors.New("acp connection is not initialized")
	}
	events, err := a.runtime.Prompt(ctx, mapPromptRequest(params))
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
	return sdk.ResumeSessionResponse{}, sdk.NewMethodNotFound(sdk.AgentMethodSessionResume)
}

func (a *agentAdapter) SetSessionConfigOption(ctx context.Context, params sdk.SetSessionConfigOptionRequest) (sdk.SetSessionConfigOptionResponse, error) {
	return sdk.SetSessionConfigOptionResponse{}, sdk.NewMethodNotFound(sdk.AgentMethodSessionSetConfigOption)
}

func (a *agentAdapter) SetSessionMode(ctx context.Context, params sdk.SetSessionModeRequest) (sdk.SetSessionModeResponse, error) {
	return sdk.SetSessionModeResponse{}, sdk.NewMethodNotFound(sdk.AgentMethodSessionSetMode)
}
