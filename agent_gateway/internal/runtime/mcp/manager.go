package mcp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrTransportUnavailable = errors.New("MCP transport connector is not configured")
var ErrUnsupportedTransport = errors.New("unsupported MCP transport")

// Client is the minimal runtime boundary needed from a concrete MCP SDK client.
type Client interface {
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, call ToolCall) (CallResult, error)
	Close() error
}

// Connector creates a client for one MCP server configuration.
type Connector interface {
	Connect(ctx context.Context, cfg ServerConfig) (Client, error)
}

type ConnectorFunc func(ctx context.Context, cfg ServerConfig) (Client, error)

func (f ConnectorFunc) Connect(ctx context.Context, cfg ServerConfig) (Client, error) {
	return f(ctx, cfg)
}

type ManagerOption func(*Manager)

// WithConnector injects the concrete MCP SDK transport boundary.
func WithConnector(connector Connector) ManagerOption {
	return func(m *Manager) {
		if connector != nil {
			m.connector = connector
		}
	}
}

func WithStatusListener(listener func(ServerStatus)) ManagerOption {
	return func(m *Manager) {
		m.statusListener = listener
	}
}

// Manager tracks long-lived MCP clients and their discovered tools.
type Manager struct {
	mu             sync.RWMutex
	connector      Connector
	servers        map[string]*serverConnection
	inflight       sync.WaitGroup
	statusListener func(ServerStatus)
	closed         bool
}

type serverConnection struct {
	cfg    ServerConfig
	client Client
	tools  []Tool
	status ServerStatus
}

// NewManager creates an MCP runtime manager.
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{
		connector: Mark3LabsConnector{},
		servers:   make(map[string]*serverConnection),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
}

func (m *Manager) CallTool(ctx context.Context, serverID string, call ToolCall) (CallResult, error) {
	key := ServerConfig{ID: serverID}.ServerKey()
	if key == "" {
		return CallResult{}, fmt.Errorf("MCP server id is required")
	}

	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return CallResult{}, fmt.Errorf("MCP manager is closed")
	}
	conn := m.servers[key]
	if conn == nil || conn.client == nil {
		m.mu.RUnlock()
		return CallResult{}, fmt.Errorf("MCP server %s is not connected", key)
	}
	client := conn.client
	m.inflight.Add(1)
	m.mu.RUnlock()
	defer m.inflight.Done()

	return client.CallTool(ctx, call)
}

// RefreshTools connects or reconnects a server and returns its current tool list.
// If tool discovery fails, the manager reconnects once and retries the list call.
func (m *Manager) RefreshTools(ctx context.Context, cfg ServerConfig) ([]Tool, error) {
	var normalizeErr error
	cfg, normalizeErr = cfg.Normalized()
	if normalizeErr != nil {
		key := cfg.ServerKey()
		m.recordFailure(key, cfg, normalizeErr)
		return nil, normalizeErr
	}

	key := cfg.ServerKey()
	if key == "" {
		return nil, fmt.Errorf("MCP server id or name is required")
	}

	transport, err := cfg.ResolveTransport()
	if err != nil {
		m.recordFailure(key, cfg, err)
		return nil, err
	}

	if !cfg.Enabled {
		if err := m.closeServerWithState(key, cfg, StateDisabled, ""); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if _, err := cfg.Environment(); err != nil {
		m.recordFailure(key, cfg, err)
		return nil, err
	}

	if err := m.setConnecting(key, cfg, transport); err != nil {
		return nil, err
	}

	client, tools, err := m.connectAndList(ctx, cfg)
	if err != nil {
		m.recordFailure(key, cfg, err)
		return nil, err
	}

	oldClient, ok := m.replaceConnected(key, cfg, transport, client, tools)
	if !ok {
		_ = client.Close()
		return nil, fmt.Errorf("MCP manager is closed")
	}
	if oldClient != nil {
		_ = oldClient.Close()
	}
	return cloneTools(tools), nil
}

// Status returns a snapshot for one server. Unknown servers are disconnected.
func (m *Manager) Status(id string) ServerStatus {
	key := ServerConfig{ID: id}.ServerKey()
	now := time.Now().UTC()

	m.mu.RLock()
	defer m.mu.RUnlock()

	if conn, ok := m.servers[key]; ok {
		return conn.status
	}
	return ServerStatus{ID: key, State: StateDisconnected, UpdatedAt: now}
}

// StatusAll returns all known server snapshots.
func (m *Manager) StatusAll() []ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]ServerStatus, 0, len(m.servers))
	for _, conn := range m.servers {
		out = append(out, conn.status)
	}
	return out
}

// CloseServer closes one server connection and marks it disconnected.
func (m *Manager) CloseServer(id string) error {
	key := ServerConfig{ID: id}.ServerKey()
	if key == "" {
		return fmt.Errorf("MCP server id is required")
	}
	return m.closeServerWithState(key, ServerConfig{ID: key}, StateDisconnected, "")
}

// Close closes every tracked MCP connection. The manager cannot be reused after
// Close returns.
func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true

	connections := make([]Client, 0, len(m.servers))
	for _, conn := range m.servers {
		if conn.client != nil {
			connections = append(connections, conn.client)
		}
		conn.client = nil
		conn.tools = nil
		conn.status.State = StateClosed
		conn.status.ToolCount = 0
		conn.status.UpdatedAt = time.Now().UTC()
		m.publishStatusLocked(conn.status)
	}
	m.mu.Unlock()

	m.inflight.Wait()
	return closeConnections(connections)
}

func (m *Manager) connectAndList(ctx context.Context, cfg ServerConfig) (Client, []Tool, error) {
	client, err := m.connector.Connect(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	tools, err := client.ListTools(ctx)
	if err == nil {
		return client, tools, nil
	}
	_ = client.Close()

	retryClient, retryErr := m.connector.Connect(ctx, cfg)
	if retryErr != nil {
		return nil, nil, fmt.Errorf("list tools failed: %w; reconnect failed: %w", err, retryErr)
	}
	tools, retryErr = retryClient.ListTools(ctx)
	if retryErr != nil {
		_ = retryClient.Close()
		return nil, nil, fmt.Errorf("list tools failed after reconnect: %w", retryErr)
	}
	return retryClient, tools, nil
}

func (m *Manager) setConnecting(key string, cfg ServerConfig, transport TransportType) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("MCP manager is closed")
	}
	now := time.Now().UTC()
	conn := m.ensureConnectionLocked(key, cfg)
	status := ServerStatus{
		ID:        key,
		Name:      cfg.Name,
		State:     StateConnecting,
		Transport: string(transport),
		UpdatedAt: now,
	}
	conn.status = status
	m.publishStatusLocked(status)
	return nil
}

func (m *Manager) replaceConnected(
	key string,
	cfg ServerConfig,
	transport TransportType,
	client Client,
	tools []Tool,
) (Client, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, false
	}

	old := m.ensureConnectionLocked(key, cfg).client
	now := time.Now().UTC()
	status := ServerStatus{
		ID:          key,
		Name:        cfg.Name,
		State:       StateConnected,
		Transport:   string(transport),
		ToolCount:   len(tools),
		UpdatedAt:   now,
		ConnectedAt: now,
	}
	m.servers[key] = &serverConnection{
		cfg:    cfg,
		client: client,
		tools:  cloneTools(tools),
		status: status,
	}
	m.publishStatusLocked(status)
	return old, true
}

func (m *Manager) recordFailure(key string, cfg ServerConfig, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	conn := m.ensureConnectionLocked(key, cfg)
	conn.status.ID = key
	conn.status.Name = cfg.Name
	conn.status.State = StateFailed
	conn.status.ToolCount = len(conn.tools)
	conn.status.LastError = err.Error()
	conn.status.UpdatedAt = time.Now().UTC()
	m.publishStatusLocked(conn.status)
}

func (m *Manager) closeServerWithState(key string, cfg ServerConfig, state State, lastError string) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return fmt.Errorf("MCP manager is closed")
	}
	conn := m.ensureConnectionLocked(key, cfg)
	client := conn.client
	conn.client = nil
	conn.tools = nil
	conn.status.State = state
	conn.status.ToolCount = 0
	conn.status.LastError = lastError
	conn.status.UpdatedAt = time.Now().UTC()
	status := conn.status
	m.publishStatusLocked(status)
	m.mu.Unlock()

	if client != nil {
		return client.Close()
	}
	return nil
}

func (m *Manager) publishStatusLocked(status ServerStatus) {
	if m.statusListener == nil {
		return
	}
	copied := status
	go m.statusListener(copied)
}

func (m *Manager) ensureConnectionLocked(key string, cfg ServerConfig) *serverConnection {
	if conn, ok := m.servers[key]; ok {
		if cfg.Name != "" {
			conn.cfg = cfg
			conn.status.Name = cfg.Name
		}
		return conn
	}
	conn := &serverConnection{
		cfg: cfg,
		status: ServerStatus{
			ID:        key,
			Name:      cfg.Name,
			State:     StateDisconnected,
			UpdatedAt: time.Now().UTC(),
		},
	}
	m.servers[key] = conn
	return conn
}

func closeConnections(connections []Client) error {
	var errs []error
	for _, client := range connections {
		if client == nil {
			continue
		}
		if err := client.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func cloneTools(tools []Tool) []Tool {
	out := make([]Tool, len(tools))
	copy(out, tools)
	return out
}

type unavailableConnector struct{}

func (unavailableConnector) Connect(context.Context, ServerConfig) (Client, error) {
	return nil, ErrTransportUnavailable
}
