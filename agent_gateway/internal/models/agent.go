package models

// AgentProtocol 定义了支持的智能体协议。
type AgentProtocol string

const AgentProtocolACP AgentProtocol = "acp_agent"
const AgentProtocolICOO AgentProtocol = "icoo_agent"

func (p AgentProtocol) String() string { return string(p) }

type Agent struct {
	BaseModel
	Name        string        `gorm:"size:256;not null;comment:智能体名称"`
	Protocol    AgentProtocol `gorm:"size:64;comment:智能体协议"`
	Description string        `gorm:"size:-1;comment:智能体描述"`
	ModelsJSON  string        `gorm:"type:text;comment:智能体模型列表"`
	Command     string        `gorm:"size:-1;comment:智能体命令行"`    // 仅对ACP协议生效
	Args        ArrayString   `gorm:"type:text;comment:智能体命令参数"` // 仅对ACP协议生效
	Enabled     bool          `gorm:"not null;comment:是否启用智能体"`
	Position    int           `gorm:"not null;index;comment:智能体位置"`
}

func (Agent) TableName() string { return "agents" }
