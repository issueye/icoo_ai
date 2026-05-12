package models

type Metadata struct {
	Key   string `gorm:"primaryKey;size:128;comment:元数据键"`
	Value string `gorm:"size:-1;comment:元数据值"`
}

func (Metadata) TableName() string { return "metadata" }
