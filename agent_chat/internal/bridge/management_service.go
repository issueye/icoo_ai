package bridge

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/gorm"
)

type managementService struct {
	store *managementStore
}

func newManagementService() (*managementService, error) {
	store, err := getManagementStore()
	if err != nil {
		return nil, err
	}
	return &managementService{store: store}, nil
}

func applyManagementSettingsFromStore(settings AppSettings) (AppSettings, error) {
	svc, err := newManagementService()
	if err != nil {
		return settings, err
	}
	ctx := context.Background()

	channelRecords, err := svc.store.channels.List(ctx)
	if err != nil {
		return settings, err
	}
	agentRecords, err := svc.store.agents.List(ctx)
	if err != nil {
		return settings, err
	}
	mcpRecords, err := svc.store.mcps.List(ctx)
	if err != nil {
		return settings, err
	}
	taskRecords, err := svc.store.tasks.List(ctx)
	if err != nil {
		return settings, err
	}

	settings.Channels = make([]ChannelConfig, 0, len(channelRecords))
	for _, rec := range channelRecords {
		settings.Channels = append(settings.Channels, ChannelConfig{
			ID:         strings.TrimSpace(rec.ID),
			Name:       strings.TrimSpace(rec.Name),
			Type:       strings.TrimSpace(rec.Type),
			Enabled:    rec.Enabled,
			AppID:      strings.TrimSpace(rec.AppID),
			AppSecret:  strings.TrimSpace(rec.AppSecret),
			BotToken:   strings.TrimSpace(rec.BotToken),
			WebhookURL: strings.TrimSpace(rec.WebhookURL),
		})
	}

	settings.MCPServers = make([]MCPServerConfig, 0, len(mcpRecords))
	for _, rec := range mcpRecords {
		settings.MCPServers = append(settings.MCPServers, MCPServerConfig{
			ID:      strings.TrimSpace(rec.ID),
			Name:    strings.TrimSpace(rec.Name),
			Command: strings.TrimSpace(rec.Command),
			Args:    decodeArgsField(rec.Args),
			Enabled: rec.Enabled,
		})
	}

	settings.Agents = make([]AgentConfig, 0, len(agentRecords))
	for _, rec := range agentRecords {
		settings.Agents = append(settings.Agents, AgentConfig{
			ID:          strings.TrimSpace(rec.ID),
			Name:        strings.TrimSpace(rec.Name),
			Protocol:    strings.TrimSpace(rec.Protocol),
			Description: strings.TrimSpace(rec.Description),
			Models:      decodeArgsField(rec.Models),
			Enabled:     rec.Enabled,
		})
	}

	settings.ScheduleTasks = make([]ScheduleTaskConfig, 0, len(taskRecords))
	for _, rec := range taskRecords {
		settings.ScheduleTasks = append(settings.ScheduleTasks, ScheduleTaskConfig{
			ID:      strings.TrimSpace(rec.ID),
			Name:    strings.TrimSpace(rec.Name),
			Spec:    strings.TrimSpace(rec.Spec),
			Command: strings.TrimSpace(rec.Command),
			Args:    decodeArgsField(rec.Args),
			Enabled: rec.Enabled,
		})
	}

	settings = normalizeAppSettings(settings)
	return settings, nil
}

func persistManagementSettings(settings AppSettings) error {
	svc, err := newManagementService()
	if err != nil {
		return err
	}
	ctx := context.Background()

	channelRecords := make([]ChannelRecord, 0, len(settings.Channels))
	for index, item := range normalizeChannels(settings.Channels) {
		channelRecords = append(channelRecords, ChannelRecord{
			ID:         strings.TrimSpace(item.ID),
			Name:       strings.TrimSpace(item.Name),
			Type:       strings.TrimSpace(item.Type),
			Enabled:    item.Enabled,
			AppID:      strings.TrimSpace(item.AppID),
			AppSecret:  strings.TrimSpace(item.AppSecret),
			BotToken:   strings.TrimSpace(item.BotToken),
			WebhookURL: strings.TrimSpace(item.WebhookURL),
			SortOrder:  index,
		})
	}

	mcpRecords := make([]MCPServerRecord, 0, len(settings.MCPServers))
	for index, item := range normalizeMCPServers(settings.MCPServers) {
		mcpRecords = append(mcpRecords, MCPServerRecord{
			ID:        strings.TrimSpace(item.ID),
			Name:      strings.TrimSpace(item.Name),
			Command:   strings.TrimSpace(item.Command),
			Args:      encodeArgsField(item.Args),
			Enabled:   item.Enabled,
			SortOrder: index,
		})
	}

	agentRecords := make([]AgentRecord, 0, len(settings.Agents))
	for index, item := range normalizeAgents(settings.Agents) {
		agentRecords = append(agentRecords, AgentRecord{
			ID:          strings.TrimSpace(item.ID),
			Name:        strings.TrimSpace(item.Name),
			Protocol:    strings.TrimSpace(item.Protocol),
			Description: strings.TrimSpace(item.Description),
			Models:      encodeArgsField(item.Models),
			Enabled:     item.Enabled,
			SortOrder:   index,
		})
	}

	taskRecords := make([]ScheduleTaskRecord, 0, len(settings.ScheduleTasks))
	for index, item := range normalizeScheduleTasks(settings.ScheduleTasks) {
		taskRecords = append(taskRecords, ScheduleTaskRecord{
			ID:        strings.TrimSpace(item.ID),
			Name:      strings.TrimSpace(item.Name),
			Spec:      strings.TrimSpace(item.Spec),
			Command:   strings.TrimSpace(item.Command),
			Args:      encodeArgsField(item.Args),
			Enabled:   item.Enabled,
			SortOrder: index,
		})
	}

	return svc.store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		channelCRUD := NewGormCRUD[ChannelRecord](tx)
		agentCRUD := NewGormCRUD[AgentRecord](tx)
		mcpCRUD := NewGormCRUD[MCPServerRecord](tx)
		taskCRUD := NewGormCRUD[ScheduleTaskRecord](tx)
		if err := channelCRUD.ReplaceAll(ctx, channelRecords); err != nil {
			return err
		}
		if err := agentCRUD.ReplaceAll(ctx, agentRecords); err != nil {
			return err
		}
		if err := mcpCRUD.ReplaceAll(ctx, mcpRecords); err != nil {
			return err
		}
		if err := taskCRUD.ReplaceAll(ctx, taskRecords); err != nil {
			return err
		}
		return nil
	})
}

func encodeArgsField(args []string) string {
	cleaned := make([]string, 0, len(args))
	for _, arg := range args {
		text := strings.TrimSpace(arg)
		if text == "" {
			continue
		}
		cleaned = append(cleaned, text)
	}
	data, err := json.Marshal(cleaned)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func decodeArgsField(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	cleaned := make([]string, 0, len(out))
	for _, item := range out {
		text := strings.TrimSpace(item)
		if text == "" {
			continue
		}
		cleaned = append(cleaned, text)
	}
	return cleaned
}
