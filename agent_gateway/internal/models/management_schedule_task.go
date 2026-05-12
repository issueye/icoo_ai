package models

type ManagementScheduleTask struct {
	BaseModel
	Name     string `gorm:"size:256;not null"`
	Spec     string `gorm:"size:256"`
	Content  string `gorm:"type:text"`
	Enabled  bool   `gorm:"not null"`
	Position int    `gorm:"not null;index"`
}

func (ManagementScheduleTask) TableName() string { return "management_schedule_tasks" }
