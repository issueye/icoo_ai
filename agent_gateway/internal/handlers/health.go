package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

type HealthResponse struct {
	Status       string    `json:"status"`
	Version      string    `json:"version"`
	Capabilities []string  `json:"capabilities"`
	StartedAt    time.Time `json:"startedAt"`
}

func HealthHandler(version string, startedAt time.Time) http.Handler {
	response := HealthResponse{
		Status:       "ok",
		Version:      version,
		Capabilities: []string{"health", "local-auth", "endpoint-file", "crud-handlers"},
		StartedAt:    startedAt,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
}
