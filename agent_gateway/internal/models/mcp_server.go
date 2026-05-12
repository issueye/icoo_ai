package models

type MCPServer struct {
	BaseModel
	Name     string      `gorm:"size:256;not null;comment:服务器名称"`
	Command  string      `gorm:"size:-1;comment:服务器命令行"`
	ArgsJSON AgentConfig `gorm:"type:text;comment:服务器参数列表"`
	Enabled  bool        `gorm:"not null;comment:是否启用服务器"`
}

func (MCPServer) TableName() string { return "mcp_servers" }
