package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrManagementSettingsNotFound = errors.New("management settings not found")

type ManagementSettingsStore interface {
	Load(ctx context.Context) (ManagementSettings, error)
	Save(ctx context.Context, settings ManagementSettings) error
	Close() error
}

type MemoryManagementSettingsStore struct {
	mu       sync.RWMutex
	settings *ManagementSettings
}

func NewMemoryManagementSettingsStore() *MemoryManagementSettingsStore {
	return &MemoryManagementSettingsStore{}
}

func (s *MemoryManagementSettingsStore) Load(ctx context.Context) (ManagementSettings, error) {
	if err := ctx.Err(); err != nil {
		return ManagementSettings{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil {
		return ManagementSettings{}, ErrManagementSettingsNotFound
	}
	return cloneManagementSettings(*s.settings), nil
}

func (s *MemoryManagementSettingsStore) Save(ctx context.Context, settings ManagementSettings) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := cloneManagementSettings(settings)
	s.settings = &cloned
	return nil
}

func (s *MemoryManagementSettingsStore) Close() error {
	return nil
}

type SQLiteManagementSettingsStore struct {
	mu sync.Mutex
	db *gorm.DB
}

type channelRow struct {
	ID         string `gorm:"primaryKey;size:128"`
	Name       string `gorm:"size:256;not null"`
	Type       string `gorm:"size:64;not null"`
	Enabled    bool   `gorm:"not null"`
	AppID      string `gorm:"size:1024"`
	AppSecret  string `gorm:"size:2048"`
	BotToken   string `gorm:"size:2048"`
	WebhookURL string `gorm:"size:2048"`
	Position   int    `gorm:"not null;index"`
}

func (channelRow) TableName() string { return "management_channels" }

type mcpServerRow struct {
	ID       string `gorm:"primaryKey;size:128"`
	Name     string `gorm:"size:256;not null"`
	Command  string `gorm:"size:2048"`
	ArgsJSON string `gorm:"type:text"`
	Enabled  bool   `gorm:"not null"`
	Position int    `gorm:"not null;index"`
}

func (mcpServerRow) TableName() string { return "management_mcp_servers" }

type scheduleTaskRow struct {
	ID       string `gorm:"primaryKey;size:128"`
	Name     string `gorm:"size:256;not null"`
	Spec     string `gorm:"size:256"`
	Content  string `gorm:"type:text"`
	Enabled  bool   `gorm:"not null"`
	Position int    `gorm:"not null;index"`
}

func (scheduleTaskRow) TableName() string { return "management_schedule_tasks" }

type agentRow struct {
	ID          string `gorm:"primaryKey;size:128"`
	Name        string `gorm:"size:256;not null"`
	Protocol    string `gorm:"size:64"`
	Description string `gorm:"size:2048"`
	ModelsJSON  string `gorm:"type:text"`
	Enabled     bool   `gorm:"not null"`
	Position    int    `gorm:"not null;index"`
}

func (agentRow) TableName() string { return "management_agents" }

type managementMetaRow struct {
	Key   string `gorm:"primaryKey;size:128"`
	Value string `gorm:"size:2048"`
}

func (managementMetaRow) TableName() string { return "management_meta" }

func NewSQLiteManagementSettingsStore(path string) (*SQLiteManagementSettingsStore, error) {
	target := strings.TrimSpace(path)
	if target == "" {
		return nil, fmt.Errorf("management settings sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}
	db, err := gorm.Open(sqlite.Open(target), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.AutoMigrate(&channelRow{}, &mcpServerRow{}, &scheduleTaskRow{}, &agentRow{}, &managementMetaRow{}); err != nil {
		return nil, fmt.Errorf("migrate sqlite schema: %w", err)
	}
	return &SQLiteManagementSettingsStore{db: db}, nil
}

func (s *SQLiteManagementSettingsStore) Load(ctx context.Context) (ManagementSettings, error) {
	if err := ctx.Err(); err != nil {
		return ManagementSettings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var channels []channelRow
	var mcpRows []mcpServerRow
	var taskRows []scheduleTaskRow
	var agentRows []agentRow
	if err := s.db.WithContext(ctx).Order("position asc").Find(&channels).Error; err != nil {
		return ManagementSettings{}, fmt.Errorf("load channels: %w", err)
	}
	if err := s.db.WithContext(ctx).Order("position asc").Find(&mcpRows).Error; err != nil {
		return ManagementSettings{}, fmt.Errorf("load mcp servers: %w", err)
	}
	if err := s.db.WithContext(ctx).Order("position asc").Find(&taskRows).Error; err != nil {
		return ManagementSettings{}, fmt.Errorf("load schedule tasks: %w", err)
	}
	if err := s.db.WithContext(ctx).Order("position asc").Find(&agentRows).Error; err != nil {
		return ManagementSettings{}, fmt.Errorf("load agents: %w", err)
	}
	var meta managementMetaRow
	metaErr := s.db.WithContext(ctx).First(&meta, "key = ?", "initialized").Error
	if errors.Is(metaErr, gorm.ErrRecordNotFound) {
		return ManagementSettings{}, ErrManagementSettingsNotFound
	}
	if metaErr != nil {
		return ManagementSettings{}, fmt.Errorf("load management meta: %w", metaErr)
	}

	out := ManagementSettings{
		Channels:      make([]ChannelConfig, 0, len(channels)),
		MCPServers:    make([]MCPServerConfig, 0, len(mcpRows)),
		ScheduleTasks: make([]ScheduleTaskConfig, 0, len(taskRows)),
		Agents:        make([]AgentConfig, 0, len(agentRows)),
	}
	for _, item := range channels {
		out.Channels = append(out.Channels, ChannelConfig{
			ID:         item.ID,
			Name:       item.Name,
			Type:       item.Type,
			Enabled:    item.Enabled,
			AppID:      item.AppID,
			AppSecret:  item.AppSecret,
			BotToken:   item.BotToken,
			WebhookURL: item.WebhookURL,
		})
	}
	for _, item := range mcpRows {
		args, err := decodeStringList(item.ArgsJSON)
		if err != nil {
			return ManagementSettings{}, fmt.Errorf("decode mcp args for %s: %w", item.ID, err)
		}
		out.MCPServers = append(out.MCPServers, MCPServerConfig{
			ID: item.ID, Name: item.Name, Command: item.Command, Args: args, Enabled: item.Enabled,
		})
	}
	for _, item := range taskRows {
		out.ScheduleTasks = append(out.ScheduleTasks, ScheduleTaskConfig{
			ID: item.ID, Name: item.Name, Spec: item.Spec, Content: item.Content, Enabled: item.Enabled,
		})
	}
	for _, item := range agentRows {
		models, err := decodeStringList(item.ModelsJSON)
		if err != nil {
			return ManagementSettings{}, fmt.Errorf("decode models for %s: %w", item.ID, err)
		}
		out.Agents = append(out.Agents, AgentConfig{
			ID: item.ID, Name: item.Name, Protocol: item.Protocol, Description: item.Description, Models: models, Enabled: item.Enabled,
		})
	}
	return out, nil
}

func (s *SQLiteManagementSettingsStore) Save(ctx context.Context, settings ManagementSettings) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&channelRow{}).Error; err != nil {
			return fmt.Errorf("clear channels: %w", err)
		}
		if err := tx.Where("1 = 1").Delete(&mcpServerRow{}).Error; err != nil {
			return fmt.Errorf("clear mcp servers: %w", err)
		}
		if err := tx.Where("1 = 1").Delete(&scheduleTaskRow{}).Error; err != nil {
			return fmt.Errorf("clear schedule tasks: %w", err)
		}
		if err := tx.Where("1 = 1").Delete(&agentRow{}).Error; err != nil {
			return fmt.Errorf("clear agents: %w", err)
		}

		for i, item := range settings.Channels {
			row := channelRow{
				ID:         item.ID,
				Name:       item.Name,
				Type:       item.Type,
				Enabled:    item.Enabled,
				AppID:      item.AppID,
				AppSecret:  item.AppSecret,
				BotToken:   item.BotToken,
				WebhookURL: item.WebhookURL,
				Position:   i + 1,
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
				return fmt.Errorf("save channel %s: %w", item.ID, err)
			}
		}
		for i, item := range settings.MCPServers {
			args, err := encodeStringList(item.Args)
			if err != nil {
				return fmt.Errorf("encode mcp args for %s: %w", item.ID, err)
			}
			row := mcpServerRow{
				ID:       item.ID,
				Name:     item.Name,
				Command:  item.Command,
				ArgsJSON: args,
				Enabled:  item.Enabled,
				Position: i + 1,
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
				return fmt.Errorf("save mcp server %s: %w", item.ID, err)
			}
		}
		for i, item := range settings.ScheduleTasks {
			row := scheduleTaskRow{
				ID:       item.ID,
				Name:     item.Name,
				Spec:     item.Spec,
				Content:  item.Content,
				Enabled:  item.Enabled,
				Position: i + 1,
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
				return fmt.Errorf("save schedule task %s: %w", item.ID, err)
			}
		}
		for i, item := range settings.Agents {
			models, err := encodeStringList(item.Models)
			if err != nil {
				return fmt.Errorf("encode models for %s: %w", item.ID, err)
			}
			row := agentRow{
				ID:          item.ID,
				Name:        item.Name,
				Protocol:    item.Protocol,
				Description: item.Description,
				ModelsJSON:  models,
				Enabled:     item.Enabled,
				Position:    i + 1,
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
				return fmt.Errorf("save agent %s: %w", item.ID, err)
			}
		}
		meta := managementMetaRow{Key: "initialized", Value: "true"}
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&meta).Error; err != nil {
			return fmt.Errorf("save management meta: %w", err)
		}
		return nil
	})
}

func (s *SQLiteManagementSettingsStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	s.db = nil
	return sqlDB.Close()
}

func encodeStringList(values []string) (string, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func decodeStringList(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if out == nil {
		return []string{}, nil
	}
	return out, nil
}
