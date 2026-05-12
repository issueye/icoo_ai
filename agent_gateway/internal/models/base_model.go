package models

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID string `json:"id,omitempty" gorm:"primaryKey;size:36"`
}

func (m *BaseModel) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return nil
}

type ArrayString []string

func (a ArrayString) String() string {
	return strings.Join(a, ",")
}
