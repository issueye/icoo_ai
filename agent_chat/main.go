package main

import "github.com/icoo-ai/icoo-ai/agent_chat/internal/bridge"

func main() {
	// Wails 3 ????????? App ? bridge.AgentService?
	_ = NewApp(bridge.NewAgentService())
}
