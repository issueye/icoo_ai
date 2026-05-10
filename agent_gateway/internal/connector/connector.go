package connector

import "context"

type AgentConnector interface {
	Initialize(ctx context.Context, req InitializeRequest) (InitializeResponse, error)
	NewSession(ctx context.Context, req NewSessionRequest) (NewSessionResponse, error)
	ListSessions(ctx context.Context, req ListSessionsRequest) (ListSessionsResponse, error)
	ResumeSession(ctx context.Context, req ResumeSessionRequest) (ResumeSessionResponse, error)
	CloseSession(ctx context.Context, req CloseSessionRequest) (CloseSessionResponse, error)
	SetSessionMode(ctx context.Context, req SetSessionModeRequest) (SetSessionModeResponse, error)
	SetSessionConfigOption(ctx context.Context, req SetSessionConfigOptionRequest) (SetSessionConfigOptionResponse, error)
	Prompt(ctx context.Context, req PromptRequest) (PromptResponse, error)
	Cancel(ctx context.Context, req CancelRequest) (CancelResponse, error)
	Close() error
}
