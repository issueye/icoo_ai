package acp

import sdk "github.com/coder/acp-go-sdk"

const (
	defaultAgentName    = "icoo-ai"
	defaultAgentVersion = "dev"
)

type CapabilitiesOptions struct {
	Name    string
	Version string
}

func InitializeResponse(opts CapabilitiesOptions) sdk.InitializeResponse {
	name := opts.Name
	if name == "" {
		name = defaultAgentName
	}
	version := opts.Version
	if version == "" {
		version = defaultAgentVersion
	}
	return sdk.InitializeResponse{
		ProtocolVersion: sdk.ProtocolVersionNumber,
		AgentInfo: &sdk.Implementation{
			Name:    name,
			Version: version,
		},
		AuthMethods: []sdk.AuthMethod{},
		AgentCapabilities: sdk.AgentCapabilities{
			LoadSession: false,
			McpCapabilities: sdk.McpCapabilities{
				Http: false,
				Sse:  false,
			},
			PromptCapabilities: sdk.PromptCapabilities{
				EmbeddedContext: false,
				Image:           false,
				Audio:           false,
			},
			SessionCapabilities: sdk.SessionCapabilities{},
		},
	}
}
