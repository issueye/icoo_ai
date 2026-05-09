package gatewayclient

import "time"

type Endpoint struct {
	PID       int       `json:"pid"`
	BaseURL   string    `json:"baseUrl"`
	TokenFile string    `json:"tokenFile"`
	StartedAt time.Time `json:"startedAt"`
}

type HealthResponse struct {
	Status       string    `json:"status"`
	Version      string    `json:"version"`
	Capabilities []string  `json:"capabilities"`
	StartedAt    time.Time `json:"startedAt"`
}
