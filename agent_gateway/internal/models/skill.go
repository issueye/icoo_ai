package models

type Skill struct {
	BaseModel
	Name          string `json:"name" gorm:"size:256;not null;index"`
	Description   string `json:"description,omitempty" gorm:"type:text"`
	Source        string `json:"source,omitempty" gorm:"size:64;index;comment:技能来源"`
	Path          string `json:"path,omitempty" gorm:"type:text;comment:技能路径"`
	Entrypoint    string `json:"entrypoint,omitempty" gorm:"type:text;comment:入口文件"`
	Version       string `json:"version,omitempty" gorm:"size:128;comment:版本"`
	ContentHash   string `json:"contentHash,omitempty" gorm:"size:128;index;comment:内容哈希"`
	ManifestJSON  string `json:"manifestJson,omitempty" gorm:"type:text;comment:清单JSON"`
	MetadataJSON  string `json:"metadataJson,omitempty" gorm:"type:text;comment:扩展元数据JSON"`
	Documentation string `json:"documentation,omitempty" gorm:"type:text;comment:文档内容"`
	LastError     string `json:"lastError,omitempty" gorm:"type:text;comment:最近错误"`
	Enabled       bool   `json:"enabled" gorm:"not null;default:true;index;comment:是否启用技能"`
}

func (Skill) TableName() string { return "skills" }
