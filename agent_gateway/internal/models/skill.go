package models

type Skill struct {
	BaseModel
	Name        string `json:"name" gorm:"size:256;not null"`
	Description string `json:"description,omitempty" gorm:"size:2048"`
}
