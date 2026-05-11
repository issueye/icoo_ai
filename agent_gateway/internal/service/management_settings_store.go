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
)

var ErrManagementSettingsNotFound = errors.New("management settings not found")

type ManagementSettingsStore interface {
	Load(ctx context.Context) (ManagementSettings, error)
	Save(ctx context.Context, settings ManagementSettings) error
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

type FileManagementSettingsStore struct {
	mu   sync.Mutex
	path string
}

func NewFileManagementSettingsStore(path string) *FileManagementSettingsStore {
	return &FileManagementSettingsStore{path: strings.TrimSpace(path)}
}

func (s *FileManagementSettingsStore) Load(ctx context.Context) (ManagementSettings, error) {
	if err := ctx.Err(); err != nil {
		return ManagementSettings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(s.path) == "" {
		return ManagementSettings{}, fmt.Errorf("management settings path is required")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return ManagementSettings{}, ErrManagementSettingsNotFound
		}
		return ManagementSettings{}, fmt.Errorf("read management settings: %w", err)
	}
	var settings ManagementSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return ManagementSettings{}, fmt.Errorf("decode management settings: %w", err)
	}
	return cloneManagementSettings(settings), nil
}

func (s *FileManagementSettingsStore) Save(ctx context.Context, settings ManagementSettings) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(s.path) == "" {
		return fmt.Errorf("management settings path is required")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create management settings directory: %w", err)
	}
	payload, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode management settings: %w", err)
	}
	tempFile := s.path + ".tmp"
	if err := os.WriteFile(tempFile, payload, 0o644); err != nil {
		return fmt.Errorf("write management settings temp file: %w", err)
	}
	if err := os.Rename(tempFile, s.path); err != nil {
		_ = os.Remove(tempFile)
		return fmt.Errorf("persist management settings: %w", err)
	}
	return nil
}
