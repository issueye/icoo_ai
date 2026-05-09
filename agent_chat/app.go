package main

import "github.com/icoo-ai/icoo-ai/agent_chat/internal/bridge"

type App struct {
	AgentService *bridge.AgentService
}

func NewApp(agentService *bridge.AgentService) *App {
	return &App{AgentService: agentService}
}
