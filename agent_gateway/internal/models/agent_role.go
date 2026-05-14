package models

type AgentRole struct {
	BaseModel
	Name            string `json:"name" gorm:"size:256;not null;uniqueIndex;comment:角色名称"`
	Description     string `json:"description,omitempty" gorm:"type:text;comment:角色描述"`
	SystemPrompt    string `json:"systemPrompt,omitempty" gorm:"type:text;comment:系统提示词"`
	PermissionsJSON string `json:"permissionsJson,omitempty" gorm:"type:text;comment:权限配置JSON"`
	MetadataJSON    string `json:"metadataJson,omitempty" gorm:"type:text;comment:扩展元数据JSON"`
	Enabled         bool   `json:"enabled" gorm:"not null;default:true;index;comment:是否启用角色"`
	Position        int    `json:"position" gorm:"not null;default:0;index;comment:角色位置"`
}

func (AgentRole) TableName() string { return "agent_roles" }
