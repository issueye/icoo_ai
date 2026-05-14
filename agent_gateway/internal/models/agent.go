package models

// AgentProtocol 定义了支持的智能体协议。
type AgentProtocol string

const AgentProtocolACP AgentProtocol = "acp"

func (p AgentProtocol) String() string { return string(p) }

type Agent struct {
	BaseModel
	Name        string        `json:"name" gorm:"size:256;not null;index;comment:智能体名称"`
	Protocol    AgentProtocol `json:"protocol" gorm:"size:64;not null;default:acp;index;comment:智能体协议"`
	RoleID      string        `json:"roleId,omitempty" gorm:"size:36;index;comment:智能体角色ID"`
	Description string        `json:"description,omitempty" gorm:"type:text;comment:智能体描述"`
	ModelsJSON  string        `json:"modelsJson,omitempty" gorm:"type:text;comment:智能体模型列表"`
	Command     string        `json:"command,omitempty" gorm:"type:text;comment:智能体命令行"`
	ArgsJSON    string        `json:"argsJson,omitempty" gorm:"type:text;comment:智能体命令参数"`
	Args        ArrayString   `json:"args,omitempty" gorm:"-"`
	EnvJSON     string        `json:"envJson,omitempty" gorm:"type:text;comment:环境变量JSON"`
	Cwd         string        `json:"cwd,omitempty" gorm:"type:text;comment:工作目录"`
	Enabled     bool          `json:"enabled" gorm:"not null;default:false;index;comment:是否启用智能体"`
	Position    int           `json:"position" gorm:"not null;default:0;index;comment:智能体位置"`
}

func (Agent) TableName() string { return "agents" }
