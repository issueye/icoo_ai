package models

type ManagementSettings struct {
	Agents        []Agent        `json:"agents,omitempty" gorm:"-"`
	Channels      []Channel      `json:"channels,omitempty" gorm:"-"`
	MCPServers    []MCPServer    `json:"mcpServers,omitempty" gorm:"-"`
	ScheduleTasks []ScheduleTask `json:"scheduleTasks,omitempty" gorm:"-"`
}
