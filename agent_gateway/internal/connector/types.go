package connector

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
