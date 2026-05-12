package models

type ScheduleTask struct {
	BaseModel
	Name    string `gorm:"size:256;not null;comment:任务名称"`
	Cron    string `gorm:"size:256;comment:任务Cron表达式"`
	Spec    string `gorm:"size:256;comment:任务规格"`
	Content string `gorm:"type:text;comment:任务内容"`
	Enabled bool   `gorm:"not null;comment:是否启用任务"`
}

func (ScheduleTask) TableName() string { return "schedule_tasks" }
