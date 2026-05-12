package models

type ScheduleTaskConfig struct {
	BaseModel
	Name    string `json:"name" gorm:"size:256;not null"`
	Spec    string `json:"spec,omitempty" gorm:"size:256"`
	Content string `json:"content,omitempty" gorm:"type:text"`
	Enabled bool   `json:"enabled" gorm:"not null"`
}
