package models

type ScheduleTask struct {
	BaseModel
	Name        string `json:"name" gorm:"size:256;not null;index;comment:任务名称"`
	Description string `json:"description,omitempty" gorm:"type:text;comment:任务描述"`
	Type        string `json:"type" gorm:"size:32;not null;default:cron;index;comment:任务类型"`
	Cron        string `json:"cron,omitempty" gorm:"size:256;comment:任务Cron表达式"`
	Spec        string `json:"spec,omitempty" gorm:"size:256;comment:任务规格"`
	Timezone    string `json:"timezone,omitempty" gorm:"size:128;comment:时区"`
	AgentID     string `json:"agentId,omitempty" gorm:"size:36;index;comment:目标智能体ID"`
	Content     string `json:"content,omitempty" gorm:"type:text;comment:任务内容"`
	PayloadJSON string `json:"payloadJson,omitempty" gorm:"type:text;comment:任务负载JSON"`
	NextRunAt   *int64 `json:"nextRunAt,omitempty" gorm:"index;comment:下次执行Unix时间"`
	LastRunAt   *int64 `json:"lastRunAt,omitempty" gorm:"index;comment:最近执行Unix时间"`
	LastStatus  string `json:"lastStatus,omitempty" gorm:"size:64;index;comment:最近执行状态"`
	LastError   string `json:"lastError,omitempty" gorm:"type:text;comment:最近执行错误"`
	Enabled     bool   `json:"enabled" gorm:"not null;default:false;index;comment:是否启用任务"`
}

func (ScheduleTask) TableName() string { return "schedule_tasks" }
