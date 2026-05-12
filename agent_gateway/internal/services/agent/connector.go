package agent

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type Connector interface {
	Initialize(ctx context.Context, req models.ConnectorInitializeRequest) (models.ConnectorInitializeResponse, error)
	NewSession(ctx context.Context, req models.ConnectorNewSessionRequest) (models.ConnectorNewSessionResponse, error)
	ListSessions(ctx context.Context, req models.ConnectorListSessionsRequest) (models.ConnectorListSessionsResponse, error)
	ResumeSession(ctx context.Context, req models.ConnectorResumeSessionRequest) (models.ConnectorResumeSessionResponse, error)
	CloseSession(ctx context.Context, req models.ConnectorCloseSessionRequest) (models.ConnectorCloseSessionResponse, error)
	SetSessionMode(ctx context.Context, req models.ConnectorSetSessionModeRequest) (models.ConnectorSetSessionModeResponse, error)
	SetSessionConfigOption(ctx context.Context, req models.ConnectorSetSessionConfigOptionRequest) (models.ConnectorSetSessionConfigOptionResponse, error)
	Prompt(ctx context.Context, req models.ConnectorPromptRequest) (models.ConnectorPromptResponse, error)
	Cancel(ctx context.Context, req models.ConnectorCancelRequest) (models.ConnectorCancelResponse, error)
	Close() error
}

type AgentConnector = Connector

const (
	ErrCodeInvalidConnectorConfig = "invalid_connector_config"
	ErrCodeConnectorStartFailed   = "connector_start_failed"
	ErrCodeProcessExited          = "connector_process_exited"
	ErrCodeProtocolError          = "connector_protocol_error"
	ErrCodeIOError                = "connector_io_error"
	ErrCodeRequestCancelled       = "connector_request_cancelled"
	ErrCodeClosed                 = "connector_closed"
)

type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func NewError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func WrapError(code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}
