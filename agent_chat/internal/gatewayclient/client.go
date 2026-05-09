package gatewayclient

import (
	"bufio"
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

type StreamEnvelope struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	AgentID   string          `json:"agentId,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	RunID     string          `json:"runId,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt string          `json:"createdAt"`
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

func (c *Client) StreamEvents(ctx context.Context, lastEventID string, onEvent func(StreamEnvelope) error) error {
	if c == nil {
		return fmt.Errorf("gateway client is nil")
	}
	if c.baseURL == "" {
		return fmt.Errorf("gateway base URL is empty")
	}
	streamURL, err := url.JoinPath(c.baseURL, "v1", "events", "stream")
	if err != nil {
		return fmt.Errorf("build stream URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return fmt.Errorf("create stream request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
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
		return fmt.Errorf("gateway event stream request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = fmt.Sprintf("gateway event stream returned status %d", resp.StatusCode)
		}
		return &httpStatusError{statusCode: resp.StatusCode, message: detail}
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		raw := strings.TrimPrefix(line, "data: ")
		var event StreamEnvelope
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			return fmt.Errorf("decode stream envelope: %w", err)
		}
		if onEvent != nil {
			if err := onEvent(event); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stream: %w", err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return io.EOF
}

type httpStatusError struct {
	statusCode int
	message    string
}

func (e *httpStatusError) Error() string {
	return e.message
}

func (e *httpStatusError) StatusCode() int {
	return e.statusCode
}
