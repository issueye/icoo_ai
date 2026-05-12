package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	channelstore "github.com/icoo-ai/icoo-ai/agent_gateway/internal/channels/store"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type Manager struct {
	mu       sync.Mutex
	registry *FactoryRegistry
	store    channelstore.StatusStore
	channels map[string]Channel
	configs  map[string]models.ChannelRuntimeConfig
}

func NewManager(registry *FactoryRegistry, statusStore channelstore.StatusStore) *Manager {
	if registry == nil {
		registry = NewDefaultFactoryRegistry()
	}
	if statusStore == nil {
		statusStore = channelstore.NewMemoryStatusStore()
	}
	return &Manager{
		registry: registry,
		store:    statusStore,
		channels: map[string]Channel{},
		configs:  map[string]models.ChannelRuntimeConfig{},
	}
}

func (m *Manager) Initialize(ctx context.Context, configs []models.ChannelRuntimeConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	normalized, err := NormalizeConfigs(configs)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.stopAllLocked(ctx); err != nil {
		return err
	}
	m.channels = map[string]Channel{}
	m.configs = map[string]models.ChannelRuntimeConfig{}
	m.store.Reset()

	now := time.Now().UTC()
	for _, cfg := range normalized {
		channel, createErr := m.registry.Create(cfg)
		if createErr != nil {
			m.store.Upsert(models.ChannelRuntimeStatus{
				BaseModel:   models.BaseModel{ID: cfg.ID},
				Name:        cfg.Name,
				Type:        cfg.Type,
				Enabled:     cfg.Enabled,
				State:       models.StateError,
				LastError:   createErr.Error(),
				UpdatedAt:   now,
				Initialized: false,
			})
			return fmt.Errorf("create channel %s: %w", cfg.ID, createErr)
		}
		m.channels[cfg.ID] = channel
		m.configs[cfg.ID] = cfg
		state := models.StateInitialized
		if !cfg.Enabled {
			state = models.StateDisabled
		}
		m.store.Upsert(models.ChannelRuntimeStatus{
			BaseModel:   models.BaseModel{ID: cfg.ID},
			Name:        cfg.Name,
			Type:        cfg.Type,
			Enabled:     cfg.Enabled,
			State:       state,
			UpdatedAt:   now,
			Initialized: true,
		})
	}
	return nil
}

func (m *Manager) StartEnabled(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, channel := range m.channels {
		cfg := m.configs[id]
		if !cfg.Enabled {
			continue
		}
		now := time.Now().UTC()
		if err := channel.Start(ctx); err != nil {
			m.store.Upsert(models.ChannelRuntimeStatus{
				BaseModel:   models.BaseModel{ID: cfg.ID},
				Name:        cfg.Name,
				Type:        cfg.Type,
				Enabled:     cfg.Enabled,
				State:       models.StateError,
				LastError:   err.Error(),
				UpdatedAt:   now,
				Initialized: true,
			})
			return fmt.Errorf("start channel %s: %w", cfg.ID, err)
		}
		startedAt := now
		m.store.Upsert(models.ChannelRuntimeStatus{
			BaseModel:   models.BaseModel{ID: cfg.ID},
			Name:        cfg.Name,
			Type:        cfg.Type,
			Enabled:     cfg.Enabled,
			State:       models.StateRunning,
			UpdatedAt:   now,
			StartedAt:   &startedAt,
			Initialized: true,
		})
	}
	return nil
}

func (m *Manager) StopAll(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopAllLocked(ctx)
}

func (m *Manager) Status() []models.ChannelRuntimeStatus {
	return m.store.List()
}

func (m *Manager) stopAllLocked(ctx context.Context) error {
	for id, channel := range m.channels {
		cfg := m.configs[id]
		if err := channel.Stop(ctx); err != nil {
			now := time.Now().UTC()
			m.store.Upsert(models.ChannelRuntimeStatus{
				BaseModel:   models.BaseModel{ID: cfg.ID},
				Name:        cfg.Name,
				Type:        cfg.Type,
				Enabled:     cfg.Enabled,
				State:       models.StateError,
				LastError:   err.Error(),
				UpdatedAt:   now,
				Initialized: true,
			})
			return fmt.Errorf("stop channel %s: %w", cfg.ID, err)
		}
		now := time.Now().UTC()
		stoppedAt := now
		state := models.StateStopped
		if !cfg.Enabled {
			state = models.StateDisabled
		}
		m.store.Upsert(models.ChannelRuntimeStatus{
			BaseModel:   models.BaseModel{ID: cfg.ID},
			Name:        cfg.Name,
			Type:        cfg.Type,
			Enabled:     cfg.Enabled,
			State:       state,
			UpdatedAt:   now,
			StoppedAt:   &stoppedAt,
			Initialized: true,
		})
	}
	return nil
}
