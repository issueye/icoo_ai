package connector

import "context"

type AgentConnector interface {
	Initialize(ctx context.Context, req InitializeRequest) (InitializeResponse, error)
	NewSession(ctx context.Context, req NewSessionRequest) (NewSessionResponse, error)
	Prompt(ctx context.Context, req PromptRequest) (PromptResponse, error)
	Cancel(ctx context.Context, req CancelRequest) (CancelResponse, error)
	Close() error
}
