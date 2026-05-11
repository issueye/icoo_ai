package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type crudEntity interface {
	GetID() string
}

// GormCRUD is the standard CRUD pattern for management modules.
// It provides a unified Create/List/Update/Delete/ReplaceAll workflow.
type GormCRUD[T crudEntity] struct {
	db *gorm.DB
}

func NewGormCRUD[T crudEntity](db *gorm.DB) *GormCRUD[T] {
	return &GormCRUD[T]{db: db}
}

func (r *GormCRUD[T]) Create(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *GormCRUD[T]) List(ctx context.Context) ([]T, error) {
	var items []T
	if err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormCRUD[T]) Update(ctx context.Context, id string, updates map[string]any) error {
	var zero T
	return r.db.WithContext(ctx).Model(&zero).Where("id = ?", id).Updates(updates).Error
}

func (r *GormCRUD[T]) Delete(ctx context.Context, id string) error {
	var zero T
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&zero).Error
}

func (r *GormCRUD[T]) ReplaceAll(ctx context.Context, items []T) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var zero T
		if err := tx.Where("1 = 1").Delete(&zero).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}

type ChannelRecord struct {
	ID         string `gorm:"primaryKey;size:128"`
	Name       string `gorm:"size:255"`
	Type       string `gorm:"size:64;index"`
	Enabled    bool
	AppID      string `gorm:"size:255"`
	AppSecret  string `gorm:"size:255"`
	BotToken   string `gorm:"size:255"`
	WebhookURL string `gorm:"size:1024"`
	SortOrder  int    `gorm:"index"`
}

func (ChannelRecord) TableName() string { return "management_channels" }
func (r ChannelRecord) GetID() string   { return r.ID }

type MCPServerRecord struct {
	ID        string `gorm:"primaryKey;size:128"`
	Name      string `gorm:"size:255"`
	Command   string `gorm:"size:1024"`
	Args      string `gorm:"type:text"`
	Enabled   bool
	SortOrder int `gorm:"index"`
}

func (MCPServerRecord) TableName() string { return "management_mcp_servers" }
func (r MCPServerRecord) GetID() string   { return r.ID }

type AgentRecord struct {
	ID          string `gorm:"primaryKey;size:128"`
	Name        string `gorm:"size:255"`
	Protocol    string `gorm:"size:64;index"`
	Description string `gorm:"type:text"`
	Models      string `gorm:"type:text"`
	Enabled     bool
	SortOrder   int `gorm:"index"`
}

func (AgentRecord) TableName() string { return "management_agents" }
func (r AgentRecord) GetID() string   { return r.ID }

type ScheduleTaskRecord struct {
	ID        string `gorm:"primaryKey;size:128"`
	Name      string `gorm:"size:255"`
	Spec      string `gorm:"size:255"`
	Command   string `gorm:"size:1024"`
	Args      string `gorm:"type:text"`
	Enabled   bool
	SortOrder int `gorm:"index"`
}

func (ScheduleTaskRecord) TableName() string { return "management_schedule_tasks" }
func (r ScheduleTaskRecord) GetID() string   { return r.ID }

type managementStore struct {
	db       *gorm.DB
	channels *GormCRUD[ChannelRecord]
	agents   *GormCRUD[AgentRecord]
	mcps     *GormCRUD[MCPServerRecord]
	tasks    *GormCRUD[ScheduleTaskRecord]
}

var (
	managementStoreOnce sync.Once
	managementStoreInst *managementStore
	managementStoreErr  error
)

func getManagementStore() (*managementStore, error) {
	managementStoreOnce.Do(func() {
		dbPath, err := managementDBPath()
		if err != nil {
			managementStoreErr = err
			return
		}
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
			managementStoreErr = err
			return
		}
		db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
		if err != nil {
			managementStoreErr = err
			return
		}
		if err := db.AutoMigrate(&ChannelRecord{}, &AgentRecord{}, &MCPServerRecord{}, &ScheduleTaskRecord{}); err != nil {
			managementStoreErr = err
			return
		}
		managementStoreInst = &managementStore{
			db:       db,
			channels: NewGormCRUD[ChannelRecord](db),
			agents:   NewGormCRUD[AgentRecord](db),
			mcps:     NewGormCRUD[MCPServerRecord](db),
			tasks:    NewGormCRUD[ScheduleTaskRecord](db),
		}
	})
	if managementStoreErr != nil {
		return nil, managementStoreErr
	}
	return managementStoreInst, nil
}

func managementDBPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve workspace directory: %w", err)
	}
	return filepath.Join(wd, "data", "agent_chat.db"), nil
}
