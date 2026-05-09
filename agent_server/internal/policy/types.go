package policy

type PermissionMode string

const (
	PermissionModeReadonly       PermissionMode = "readonly"
	PermissionModeSuggest        PermissionMode = "suggest"
	PermissionModeWorkspaceWrite PermissionMode = "workspace-write"
	PermissionModeFullAuto       PermissionMode = "full-auto"
)

const DefaultPermissionMode = PermissionModeWorkspaceWrite

type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

type DecisionAction string

const (
	DecisionAllow           DecisionAction = "allow"
	DecisionBlock           DecisionAction = "block"
	DecisionRequestApproval DecisionAction = "request_approval"
)

type Decision struct {
	Action  DecisionAction `json:"action"`
	Reason  string         `json:"reason,omitempty"`
	Risk    RiskLevel      `json:"risk,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

type Policy interface {
	EvaluateCommand(CommandRequest) Decision
	EvaluatePath(PathRequest) Decision
	EvaluateNetwork(NetworkRequest) Decision
	EvaluateMCP(MCPRequest) Decision
}

type CommandRequest struct {
	Command    string `json:"command"`
	WorkingDir string `json:"working_dir,omitempty"`
}

type PathOperation string

const (
	PathOperationRead   PathOperation = "read"
	PathOperationWrite  PathOperation = "write"
	PathOperationDelete PathOperation = "delete"
)

type PathRequest struct {
	Path          string        `json:"path"`
	WorkspaceRoot string        `json:"workspace_root,omitempty"`
	Operation     PathOperation `json:"operation"`
}

type NetworkRequest struct {
	URL         string   `json:"url"`
	Method      string   `json:"method,omitempty"`
	ResolvedIPs []string `json:"resolved_ips,omitempty"`
}

type MCPRequest struct {
	Server    string         `json:"server,omitempty"`
	Name      string         `json:"name"`
	Kind      string         `json:"kind,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}
