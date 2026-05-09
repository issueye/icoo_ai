package protocol

type Server interface {
	Serve() error
}

type RuntimeMetadata struct {
	Name         string         `json:"name"`
	Version      string         `json:"version,omitempty"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
}
