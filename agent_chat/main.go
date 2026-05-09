package main

import (
	"embed"
	"log"

	"github.com/icoo-ai/icoo-ai/agent_chat/internal/bridge"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[bridge.MessageEvent]("agent:event")
}

func main() {
	agentService := bridge.NewAgentService()
	app := application.New(application.Options{
		Name:        "agent_chat",
		Description: "icoo-ai desktop agent chat client",
		Services: []application.Service{
			application.NewService(agentService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "agent_chat",
		Width:            1280,
		Height:           820,
		MinWidth:         960,
		MinHeight:        640,
		Frameless:        true,
		BackgroundColour: application.NewRGB(234, 243, 251),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
