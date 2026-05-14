package migration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/database"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
)

func TestDecodeManagementSettingsSupportsWrappedResponse(t *testing.T) {
	settings, err := DecodeManagementSettings([]byte(`{"code":"ok","data":{"agents":[{"name":"agent"}]}}`))
	if err != nil {
		t.Fatalf("DecodeManagementSettings() error = %v", err)
	}
	if len(settings.Agents) != 1 || settings.Agents[0].Name != "agent" {
		t.Fatalf("settings = %#v", settings)
	}
}

func TestMigrateManagementSettingsFileImportsAndBacksUp(t *testing.T) {
	db := openMigrationTestDB(t)
	path := filepath.Join(t.TempDir(), "settings-export.data")
	if err := os.WriteFile(path, []byte(`{
		"agents":[{"name":"agent","enabled":true}],
		"mcpServers":[{"name":"mcp","transport":"stdio","enabled":true}],
		"scheduleTasks":[{"name":"task","type":"every","spec":"1m","enabled":true}],
		"skills":[{"name":"skill","enabled":true}]
	}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := MigrateManagementSettingsFile(context.Background(), db, path, true)
	if err != nil {
		t.Fatalf("MigrateManagementSettingsFile() error = %v", err)
	}
	if result.BackupPath == "" {
		t.Fatal("BackupPath is empty")
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("backup stat error = %v", err)
	}
	if result.Agents != 1 || result.MCPServers != 1 || result.ScheduleTasks != 1 || result.Skills != 1 {
		t.Fatalf("result = %#v", result)
	}

	assertCount[models.Agent](t, db, 1)
	assertCount[models.MCPServer](t, db, 1)
	assertCount[models.ScheduleTask](t, db, 1)
	assertCount[models.Skill](t, db, 1)
}

func openMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

func assertCount[T any](t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var got int64
	if err := db.Model(new(T)).Count(&got).Error; err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if got != want {
		t.Fatalf("count = %d, want %d", got, want)
	}
}
