package connector

import "time"

const (
	ErrCodeInvalidConnectorConfig = "invalid_connector_config"
	ErrCodeConnectorStartFailed   = "connector_start_failed"
	ErrCodeProcessExited          = "connector_process_exited"
	ErrCodeProtocolError          = "connector_protocol_error"
	ErrCodeIOError                = "connector_io_error"
	ErrCodeRequestCancelled       = "connector_request_cancelled"
	ErrCodeClosed                 = "connector_closed"
)

type InitializeRequest struct {
	ClientName    string
	ClientVersion string
}

type InitializeResponse struct {
	ServerName    string
	ServerVersion string
}

type NewSessionRequest struct {
	AgentID  string
	Model    string
	CWD      string
	Metadata map[string]any
}

type NewSessionResponse struct {
	SessionID string
}

type PromptRequest struct {
	SessionID string
	Content   string
	RequestID string
}

type PromptResponse struct {
	RunID     string
	Output    string
	EndedAt   *time.Time
	Approvals []ApprovalRequest
}

type ApprovalRequest struct {
	RequestID string
	Action    string
	Message   string
}

type CancelRequest struct {
	SessionID string
	RunID     string
	Reason    string
}

type CancelResponse struct {
	RunID  string
	Status string
}

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
