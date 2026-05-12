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
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrManagementSettingsNotFound = errors.New("management settings not found")

type ManagementSettingsStore interface {
	Load(ctx context.Context) (models.ManagementSettings, error)
	Save(ctx context.Context, settings models.ManagementSettings) error
	Close() error
}

type MemoryManagementSettingsStore struct {
	mu       sync.RWMutex
	settings *models.ManagementSettings
}

func NewMemoryManagementSettingsStore() *MemoryManagementSettingsStore {
	return &MemoryManagementSettingsStore{}
}

func (s *MemoryManagementSettingsStore) Load(ctx context.Context) (models.ManagementSettings, error) {
	if err := ctx.Err(); err != nil {
		return models.ManagementSettings{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil {
		return models.ManagementSettings{}, ErrManagementSettingsNotFound
	}
	return cloneManagementSettings(*s.settings), nil
}

func (s *MemoryManagementSettingsStore) Save(ctx context.Context, settings models.ManagementSettings) error {
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
	if err := db.AutoMigrate(&models.ManagementChannel{}, &models.ManagementMCPServer{}, &models.ManagementScheduleTask{}, &models.ManagementAgent{}, &models.ManagementMeta{}); err != nil {
		return nil, fmt.Errorf("migrate sqlite schema: %w", err)
	}
	return &SQLiteManagementSettingsStore{db: db}, nil
}

func (s *SQLiteManagementSettingsStore) Load(ctx context.Context) (models.ManagementSettings, error) {
	empty := models.ManagementSettings{}

	if err := ctx.Err(); err != nil {
		return empty, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var channels []models.ManagementChannel
	var mcpRows []models.ManagementMCPServer
	var taskRows []models.ManagementScheduleTask
	var agentRows []models.ManagementAgent
	if err := s.db.WithContext(ctx).Order("position asc").Find(&channels).Error; err != nil {
		return empty, fmt.Errorf("load channels: %w", err)
	}
	if err := s.db.WithContext(ctx).Order("position asc").Find(&mcpRows).Error; err != nil {
		return empty, fmt.Errorf("load mcp servers: %w", err)
	}
	if err := s.db.WithContext(ctx).Order("position asc").Find(&taskRows).Error; err != nil {
		return empty, fmt.Errorf("load schedule tasks: %w", err)
	}
	if err := s.db.WithContext(ctx).Order("position asc").Find(&agentRows).Error; err != nil {
		return empty, fmt.Errorf("load agents: %w", err)
	}
	var meta models.ManagementMeta
	metaErr := s.db.WithContext(ctx).First(&meta, "key = ?", "initialized").Error
	if errors.Is(metaErr, gorm.ErrRecordNotFound) {
		return empty, ErrManagementSettingsNotFound
	}
	if metaErr != nil {
		return empty, fmt.Errorf("load management meta: %w", metaErr)
	}

	out := models.ManagementSettings{
		Channels:      make([]models.ChannelConfig, 0, len(channels)),
		MCPServers:    make([]models.MCPServerConfig, 0, len(mcpRows)),
		ScheduleTasks: make([]models.ScheduleTaskConfig, 0, len(taskRows)),
		Agents:        make([]models.AgentConfig, 0, len(agentRows)),
	}
	for _, item := range channels {
		out.Channels = append(out.Channels, models.ChannelConfig{
			BaseModel:  models.BaseModel{ID: item.ID},
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
			return empty, fmt.Errorf("decode mcp args for %s: %w", item.ID, err)
		}
		out.MCPServers = append(out.MCPServers, models.MCPServerConfig{
			BaseModel: models.BaseModel{ID: item.ID}, Name: item.Name, Command: item.Command, Args: args, Enabled: item.Enabled,
		})
	}
	for _, item := range taskRows {
		out.ScheduleTasks = append(out.ScheduleTasks, models.ScheduleTaskConfig{
			BaseModel: models.BaseModel{ID: item.ID}, Name: item.Name, Spec: item.Spec, Content: item.Content, Enabled: item.Enabled,
		})
	}
	for _, item := range agentRows {
		list, err := decodeStringList(item.ModelsJSON)
		if err != nil {
			return empty, fmt.Errorf("decode models for %s: %w", item.ID, err)
		}
		out.Agents = append(out.Agents, models.AgentConfig{
			BaseModel: models.BaseModel{ID: item.ID}, Name: item.Name, Protocol: item.Protocol, Description: item.Description, Models: list, Enabled: item.Enabled,
		})
	}
	return out, nil
}

func (s *SQLiteManagementSettingsStore) Save(ctx context.Context, settings models.ManagementSettings) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&models.ManagementChannel{}).Error; err != nil {
			return fmt.Errorf("clear channels: %w", err)
		}
		if err := tx.Where("1 = 1").Delete(&models.ManagementMCPServer{}).Error; err != nil {
			return fmt.Errorf("clear mcp servers: %w", err)
		}
		if err := tx.Where("1 = 1").Delete(&models.ManagementScheduleTask{}).Error; err != nil {
			return fmt.Errorf("clear schedule tasks: %w", err)
		}
		if err := tx.Where("1 = 1").Delete(&models.ManagementAgent{}).Error; err != nil {
			return fmt.Errorf("clear agents: %w", err)
		}

		for i, item := range settings.Channels {
			row := models.ManagementChannel{
				BaseModel:  models.BaseModel{ID: item.ID},
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
			row := models.ManagementMCPServer{
				BaseModel: models.BaseModel{ID: item.ID},
				Name:      item.Name,
				Command:   item.Command,
				ArgsJSON:  args,
				Enabled:   item.Enabled,
				Position:  i + 1,
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
				return fmt.Errorf("save mcp server %s: %w", item.ID, err)
			}
		}
		for i, item := range settings.ScheduleTasks {
			row := models.ManagementScheduleTask{
				BaseModel: models.BaseModel{ID: item.ID},
				Name:      item.Name,
				Spec:      item.Spec,
				Content:   item.Content,
				Enabled:   item.Enabled,
				Position:  i + 1,
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
				return fmt.Errorf("save schedule task %s: %w", item.ID, err)
			}
		}
		for i, item := range settings.Agents {
			modelList, err := encodeStringList(item.Models)
			if err != nil {
				return fmt.Errorf("encode models for %s: %w", item.ID, err)
			}
			row := models.ManagementAgent{
				BaseModel:   models.BaseModel{ID: item.ID},
				Name:        item.Name,
				Protocol:    item.Protocol,
				Description: item.Description,
				ModelsJSON:  modelList,
				Enabled:     item.Enabled,
				Position:    i + 1,
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
				return fmt.Errorf("save agent %s: %w", item.ID, err)
			}
		}
		meta := models.ManagementMeta{Key: "initialized", Value: "true"}
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
