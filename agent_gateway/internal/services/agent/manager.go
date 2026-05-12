package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type Manager struct {
	bootstrap []models.AgentProfile
	profiles  []models.AgentProfile
}

func NewManager(profiles []models.AgentProfile) *Manager {
	if len(profiles) == 0 {
		profiles = DefaultProfiles()
	}
	bootstrap := CloneProfiles(profiles)
	return &Manager{
		bootstrap: bootstrap,
		profiles:  CloneProfiles(profiles),
	}
}

func DefaultProfiles() []models.AgentProfile {
	return []models.AgentProfile{
		{
			BaseModel:   models.BaseModel{ID: "icoo-ai-acp"},
			Name:        "Icoo AI",
			Protocol:    "acp",
			Models:      []string{"gpt-5.4"},
			Description: "Default ACP connector profile.",
		},
	}
}

func (m *Manager) BootstrapConfigs() []models.AgentConfig {
	if m == nil {
		return ToConfigs(DefaultProfiles())
	}
	return ToConfigs(m.bootstrap)
}

func (m *Manager) ReplaceConfigs(configs []models.AgentConfig) {
	if m == nil {
		return
	}
	m.profiles = ToProfiles(configs)
}

func (m *Manager) List(ctx context.Context) ([]models.AgentProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m == nil {
		return CloneProfiles(DefaultProfiles()), nil
	}
	return CloneProfiles(m.profiles), nil
}

func (m *Manager) Has(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || m == nil {
		return false
	}
	for _, profile := range m.profiles {
		if profile.ID == id {
			return true
		}
	}
	return false
}

func (m *Manager) DefaultID() (string, error) {
	if m != nil && len(m.profiles) > 0 {
		return m.profiles[0].ID, nil
	}
	return "", fmt.Errorf("no enabled agents configured")
}

func ToConfigs(profiles []models.AgentProfile) []models.AgentConfig {
	out := make([]models.AgentConfig, 0, len(profiles))
	for _, item := range profiles {
		out = append(out, models.AgentConfig{
			BaseModel:   models.BaseModel{ID: strings.TrimSpace(item.ID)},
			Name:        strings.TrimSpace(item.Name),
			Protocol:    strings.TrimSpace(item.Protocol),
			Description: strings.TrimSpace(item.Description),
			Models:      append([]string(nil), item.Models...),
			Enabled:     true,
		})
	}
	return out
}

func ToProfiles(configs []models.AgentConfig) []models.AgentProfile {
	out := make([]models.AgentProfile, 0, len(configs))
	for _, item := range configs {
		if !item.Enabled {
			continue
		}
		out = append(out, models.AgentProfile{
			BaseModel:   models.BaseModel{ID: strings.TrimSpace(item.ID)},
			Name:        strings.TrimSpace(item.Name),
			Protocol:    strings.TrimSpace(item.Protocol),
			Description: strings.TrimSpace(item.Description),
			Models:      append([]string(nil), item.Models...),
		})
	}
	return out
}

func CloneProfiles(in []models.AgentProfile) []models.AgentProfile {
	out := make([]models.AgentProfile, 0, len(in))
	for _, item := range in {
		cp := item
		cp.Models = append([]string(nil), item.Models...)
		cp.Args = append([]string(nil), item.Args...)
		out = append(out, cp)
	}
	return out
}
