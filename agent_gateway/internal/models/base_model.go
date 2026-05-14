package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        string         `json:"id,omitempty" gorm:"primaryKey;size:36"`
	CreatedAt time.Time      `json:"createdAt" gorm:"not null;index"`
	UpdatedAt time.Time      `json:"updatedAt" gorm:"not null;index"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (m *BaseModel) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return nil
}

func (m BaseModel) GetID() string { return m.ID }

func (m *BaseModel) SetID(id string) { m.ID = id }

type ArrayString []string

func (a ArrayString) String() string {
	return strings.Join(a, ",")
}
