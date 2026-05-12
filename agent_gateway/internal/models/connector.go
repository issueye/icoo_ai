package models

import "time"

type ConnectorInitializeRequest struct {
	ClientName    string
	ClientVersion string
}

type ConnectorInitializeResponse struct {
	ServerName    string
	ServerVersion string
}

type ConnectorNewSessionRequest struct {
	AgentID  string
	Model    string
	CWD      string
	Metadata map[string]any
}

type ConnectorNewSessionResponse struct {
	SessionID string
}

type ConnectorSessionInfo struct {
	SessionID             string
	CWD                   string
	Title                 string
	AdditionalDirectories []string
}

type ConnectorListSessionsRequest struct {
	CWD                   string
	AdditionalDirectories []string
}

type ConnectorListSessionsResponse struct {
	Sessions []ConnectorSessionInfo
}

type ConnectorPromptRequest struct {
	SessionID string
	Content   string
	RequestID string
}

type ConnectorPromptResponse struct {
	RunID     string
	Output    string
	EndedAt   *time.Time
	Approvals []ConnectorApprovalRequest
}

type ConnectorApprovalRequest struct {
	RequestID string
	Action    string
	Message   string
}

type ConnectorCancelRequest struct {
	SessionID string
	RunID     string
	Reason    string
}

type ConnectorCancelResponse struct {
	RunID  string
	Status string
}

type ConnectorResumeSessionRequest struct {
	SessionID             string
	CWD                   string
	AdditionalDirectories []string
}

type ConnectorResumeSessionResponse struct{}

type ConnectorCloseSessionRequest struct {
	SessionID string
}

type ConnectorCloseSessionResponse struct{}

type ConnectorSetSessionModeRequest struct {
	SessionID string
	ModeID    string
}

type ConnectorSetSessionModeResponse struct{}

type ConnectorSetSessionConfigOptionRequest struct {
	SessionID    string
	ConfigID     string
	BooleanValue *bool
	ValueID      string
}

type ConnectorSetSessionConfigOptionResponse struct{}
