package models

type MCPServer struct {
	BaseModel
	Name        string `json:"name" gorm:"size:256;not null;index;comment:服务器名称"`
	Description string `json:"description,omitempty" gorm:"type:text;comment:服务器描述"`
	Transport   string `json:"transport" gorm:"size:32;not null;default:stdio;index;comment:传输类型"`
	Command     string `json:"command,omitempty" gorm:"type:text;comment:服务器命令行"`
	ArgsJSON    string `json:"argsJson,omitempty" gorm:"type:text;comment:服务器参数列表"`
	EnvJSON     string `json:"envJson,omitempty" gorm:"type:text;comment:环境变量JSON"`
	EnvFile     string `json:"envFile,omitempty" gorm:"type:text;comment:环境变量文件"`
	HeadersJSON string `json:"headersJson,omitempty" gorm:"type:text;comment:HTTP请求头JSON"`
	URL         string `json:"url,omitempty" gorm:"type:text;comment:HTTP/SSE地址"`
	Cwd         string `json:"cwd,omitempty" gorm:"type:text;comment:工作目录"`
	ToolsJSON   string `json:"toolsJson,omitempty" gorm:"type:text;comment:工具列表JSON"`
	Status      string `json:"status,omitempty" gorm:"size:64;index;comment:运行状态"`
	LastError   string `json:"lastError,omitempty" gorm:"type:text;comment:最近错误"`
	Enabled     bool   `json:"enabled" gorm:"not null;default:false;index;comment:是否启用服务器"`
}

func (MCPServer) TableName() string { return "mcp_servers" }
