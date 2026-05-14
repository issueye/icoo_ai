package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
)

type ManagementSettings struct {
	Agents        []models.Agent        `json:"agents,omitempty"`
	MCPServers    []models.MCPServer    `json:"mcpServers,omitempty"`
	ScheduleTasks []models.ScheduleTask `json:"scheduleTasks,omitempty"`
	Skills        []models.Skill        `json:"skills,omitempty"`
}

type Result struct {
	BackupPath    string `json:"backupPath,omitempty"`
	Agents        int    `json:"agents"`
	MCPServers    int    `json:"mcpServers"`
	ScheduleTasks int    `json:"scheduleTasks"`
	Skills        int    `json:"skills"`
}

func MigrateManagementSettingsFile(ctx context.Context, db *gorm.DB, path string, backup bool) (Result, error) {
	if db == nil {
		return Result{}, fmt.Errorf("database is required")
	}
	if path == "" {
		return Result{}, fmt.Errorf("input path is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("read management settings %s: %w", path, err)
	}
	settings, err := DecodeManagementSettings(raw)
	if err != nil {
		return Result{}, fmt.Errorf("decode management settings %s: %w", path, err)
	}

	result := Result{}
	if backup {
		backupPath, err := backupFile(path)
		if err != nil {
			return Result{}, err
		}
		result.BackupPath = backupPath
	}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range settings.Agents {
			if err := tx.Save(&item).Error; err != nil {
				return fmt.Errorf("migrate agent %q: %w", item.Name, err)
			}
			result.Agents++
		}
		for _, item := range settings.MCPServers {
			if err := tx.Save(&item).Error; err != nil {
				return fmt.Errorf("migrate mcp server %q: %w", item.Name, err)
			}
			result.MCPServers++
		}
		for _, item := range settings.ScheduleTasks {
			if err := tx.Save(&item).Error; err != nil {
				return fmt.Errorf("migrate schedule task %q: %w", item.Name, err)
			}
			result.ScheduleTasks++
		}
		for _, item := range settings.Skills {
			if err := tx.Save(&item).Error; err != nil {
				return fmt.Errorf("migrate skill %q: %w", item.Name, err)
			}
			result.Skills++
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

func DecodeManagementSettings(raw []byte) (ManagementSettings, error) {
	var settings ManagementSettings
	if err := json.Unmarshal(raw, &settings); err == nil && !settings.empty() {
		return settings, nil
	}

	var wrapped struct {
		Data ManagementSettings `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return ManagementSettings{}, err
	}
	return wrapped.Data, nil
}

func (s ManagementSettings) empty() bool {
	return len(s.Agents) == 0 && len(s.MCPServers) == 0 && len(s.ScheduleTasks) == 0 && len(s.Skills) == 0
}

func backupFile(path string) (string, error) {
	src, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open migration backup source %s: %w", path, err)
	}
	defer src.Close()

	backupPath := path + "." + time.Now().UTC().Format("20060102T150405Z") + ".bak"
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return "", fmt.Errorf("create migration backup directory: %w", err)
	}
	dst, err := os.OpenFile(backupPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("create migration backup %s: %w", backupPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("write migration backup %s: %w", backupPath, err)
	}
	return backupPath, nil
}
