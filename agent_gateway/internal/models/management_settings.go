package models

type ManagementSettings struct {
	Channels      []ChannelConfig      `json:"channels,omitempty" gorm:"-"`
	MCPServers    []MCPServerConfig    `json:"mcpServers,omitempty" gorm:"-"`
	ScheduleTasks []ScheduleTaskConfig `json:"scheduleTasks,omitempty" gorm:"-"`
	Agents        []AgentConfig        `json:"agents,omitempty" gorm:"-"`
}
