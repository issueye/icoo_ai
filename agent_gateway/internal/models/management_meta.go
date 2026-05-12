package models

type ManagementMeta struct {
	Key   string `gorm:"primaryKey;size:128"`
	Value string `gorm:"size:2048"`
}

func (ManagementMeta) TableName() string { return "management_meta" }
