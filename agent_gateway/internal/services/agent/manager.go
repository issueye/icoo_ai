package agent

import (
	"context"
	"encoding/json"
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

func (m *Manager) BootstrapAgents() []models.Agent {
	if m == nil {
		return ToAgents(DefaultProfiles())
	}
	return ToAgents(m.bootstrap)
}

func (m *Manager) ReplaceAgents(agents []models.Agent) {
	if m == nil {
		return
	}
	m.profiles = AgentsToProfiles(agents)
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

func ToAgents(profiles []models.AgentProfile) []models.Agent {
	out := make([]models.Agent, 0, len(profiles))
	for _, item := range profiles {
		modelsJSON, _ := json.Marshal(item.Models)
		out = append(out, models.Agent{
			BaseModel:   models.BaseModel{ID: strings.TrimSpace(item.ID)},
			Name:        strings.TrimSpace(item.Name),
			Protocol:    models.AgentProtocol(strings.TrimSpace(item.Protocol)),
			Description: strings.TrimSpace(item.Description),
			ModelsJSON:  string(modelsJSON),
			Command:     strings.TrimSpace(item.Command),
			Enabled:     true,
		})
	}
	return out
}

func AgentsToProfiles(agents []models.Agent) []models.AgentProfile {
	out := make([]models.AgentProfile, 0, len(agents))
	for _, item := range agents {
		if !item.Enabled {
			continue
		}
		var agentModels []string
		_ = json.Unmarshal([]byte(item.ModelsJSON), &agentModels)
		out = append(out, models.AgentProfile{
			BaseModel:   models.BaseModel{ID: strings.TrimSpace(item.ID)},
			Name:        strings.TrimSpace(item.Name),
			Protocol:    strings.TrimSpace(string(item.Protocol)),
			Description: strings.TrimSpace(item.Description),
			Command:     strings.TrimSpace(item.Command),
			Models:      agentModels,
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
