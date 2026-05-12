package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestBaseModelBeforeCreateGeneratesUUID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&AgentConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	agent := AgentConfig{Name: "Test Agent", Enabled: true}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := uuid.Parse(agent.ID); err != nil {
		t.Fatalf("ID = %q, want uuid: %v", agent.ID, err)
	}
}
