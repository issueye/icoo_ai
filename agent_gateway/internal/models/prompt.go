package models

type PromptRequest struct {
	Content     string `json:"content"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	CWD         string `json:"cwd,omitempty"`
	Mode        string `json:"mode,omitempty"`
	AgentID     string `json:"agentId,omitempty"`
	Model       string `json:"model,omitempty"`
}

type PromptResponse struct {
	Run      Run       `json:"run"`
	Messages []Message `json:"messages"`
	Approval *Approval `json:"approval,omitempty"`
}
