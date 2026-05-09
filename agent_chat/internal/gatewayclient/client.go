package gatewayclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      strings.TrimSpace(token),
		httpClient: http.DefaultClient,
	}
}

func (c *Client) Health(ctx context.Context) (HealthResponse, error) {
	if c == nil {
		return HealthResponse{}, fmt.Errorf("gateway client is nil")
	}
	if c.baseURL == "" {
		return HealthResponse{}, fmt.Errorf("gateway base URL is empty")
	}
	healthURL, err := url.JoinPath(c.baseURL, "health")
	if err != nil {
		return HealthResponse{}, fmt.Errorf("build health URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return HealthResponse{}, fmt.Errorf("create health request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return HealthResponse{}, fmt.Errorf("gateway health request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			return HealthResponse{}, fmt.Errorf("gateway health returned status %d", resp.StatusCode)
		}
		return HealthResponse{}, fmt.Errorf("gateway health returned status %d: %s", resp.StatusCode, detail)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return HealthResponse{}, fmt.Errorf("decode gateway health response: %w", err)
	}
	return health, nil
}
