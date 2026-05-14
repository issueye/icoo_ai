package database

import (
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return db.AutoMigrate(
		&models.Agent{},
		&models.AgentRole{},
		&models.MCPServer{},
		&models.ScheduleTask{},
		&models.Skill{},
	)
}
