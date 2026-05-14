package gatewayclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
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
	return c.StreamEventsWithFilter(ctx, lastEventID, "", "", onEvent)
}

func (c *Client) StreamEventsWithFilter(ctx context.Context, lastEventID string, sessionID string, agentID string, onEvent func(StreamEnvelope) error) error {
	conn, err := c.dialEvents(ctx, lastEventID, sessionID, agentID)
	if err != nil {
		return err
	}
	defer conn.Close()

	for {
		var msg rpcMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return io.EOF
			}
			return fmt.Errorf("read gateway event websocket: %w", err)
		}
		if msg.Method != "event" || len(msg.Params) == 0 {
			continue
		}
		var event StreamEnvelope
		if err := json.Unmarshal(msg.Params, &event); err != nil {
			return fmt.Errorf("decode stream envelope: %w", err)
		}
		if onEvent != nil {
			if err := onEvent(event); err != nil {
				return err
			}
		}
	}
}

func (c *Client) ProbeEvents(ctx context.Context) error {
	conn, err := c.dialEvents(ctx, "", "", "")
	if err != nil {
		return err
	}
	return conn.Close()
}

func (c *Client) dialEvents(ctx context.Context, lastEventID string, sessionID string, agentID string) (*websocket.Conn, error) {
	if c == nil {
		return nil, fmt.Errorf("gateway client is nil")
	}
	if c.baseURL == "" {
		return nil, fmt.Errorf("gateway base URL is empty")
	}
	eventsURL, err := url.JoinPath(c.baseURL, "v1", "events")
	if err != nil {
		return nil, fmt.Errorf("build websocket URL: %w", err)
	}
	u, err := url.Parse(eventsURL)
	if err != nil {
		return nil, fmt.Errorf("parse websocket URL: %w", err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	query := u.Query()
	if strings.TrimSpace(lastEventID) != "" {
		query.Set("lastEventId", strings.TrimSpace(lastEventID))
	}
	if strings.TrimSpace(sessionID) != "" {
		query.Set("sessionId", strings.TrimSpace(sessionID))
	}
	if strings.TrimSpace(agentID) != "" {
		query.Set("agentId", strings.TrimSpace(agentID))
	}
	u.RawQuery = query.Encode()
	header := http.Header{}
	if c.token != "" {
		header.Set("Authorization", "Bearer "+c.token)
	}
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.DialContext(ctx, u.String(), header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			detail := strings.TrimSpace(string(body))
			if detail == "" {
				detail = fmt.Sprintf("gateway event websocket returned status %d", resp.StatusCode)
			}
			return nil, &httpStatusError{statusCode: resp.StatusCode, message: detail}
		}
		return nil, fmt.Errorf("gateway event websocket request: %w", err)
	}
	return conn, nil
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
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
