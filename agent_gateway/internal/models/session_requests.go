package models

type CreateSessionRequest struct {
	Title                 string   `json:"title"`
	WorkspaceID           string   `json:"workspaceId,omitempty"`
	CWD                   string   `json:"cwd,omitempty"`
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
	StartupCommand        string   `json:"startupCommand,omitempty"`
	Mode                  string   `json:"mode,omitempty"`
	AgentID               string   `json:"agentId,omitempty"`
	Model                 string   `json:"model,omitempty"`
}

type ResumeSessionRequest struct {
	CWD                   string   `json:"cwd,omitempty"`
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
}

type SetSessionModeRequest struct {
	Mode string `json:"mode"`
}

type SetSessionConfigOptionRequest struct {
	ConfigID     string `json:"configId"`
	BooleanValue *bool  `json:"booleanValue,omitempty"`
	ValueID      string `json:"valueId,omitempty"`
}
