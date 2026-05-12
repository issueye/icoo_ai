package wshub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	jsonrpcVersion = "2.0"
	writeTimeout   = 5 * time.Second
	pingInterval   = 15 * time.Second
)

const (
	errParseError     = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternal       = -32603
)

// RawHandler 是底层 JSON-RPC 路由回调。
// params 是请求中的原始 JSON 值，适合在泛型 Handle 助手无法满足时自行解码。
type RawHandler func(ctx context.Context, params json.RawMessage) error

// EventSource 是 wshub 依赖的事件源抽象。
// 业务层可以把自己的事件总线、消息队列或其它发布订阅实现适配成该接口。
type EventSource interface {
	Subscribe(ctx context.Context, lastEventID string) (Subscription, []any)
}

// Subscription 表示一次事件订阅。
type Subscription interface {
	Events() <-chan any
	Close()
}

// EventFilter 用于按连接请求过滤服务端事件。
type EventFilter func(event any, r *http.Request) bool

// Hub 负责 WebSocket 升级、JSON-RPC 路由分发和事件广播。
// 它会把事件源中的事件转成 JSON-RPC notification 推给客户端，
// 同时接收客户端发来的 JSON-RPC request/notification。
type Hub struct {
	source   EventSource
	filter   EventFilter
	upgrader websocket.Upgrader

	mu       sync.RWMutex
	handlers map[string]RawHandler
}

type request struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type client struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

// New 创建绑定到事件源的 WebSocket Hub。
func New(source EventSource, options ...Option) *Hub {
	hub := &Hub{
		source:   source,
		filter:   AcceptAll,
		handlers: make(map[string]RawHandler),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
	return hub.apply(options...)
}

// Option 用于调整 Hub 行为。
type Option func(*Hub)

// WithFilter 设置服务端事件过滤逻辑。
func WithFilter(filter EventFilter) Option {
	return func(h *Hub) {
		if filter != nil {
			h.filter = filter
		}
	}
}

func (h *Hub) apply(options ...Option) *Hub {
	for _, option := range options {
		if option != nil {
			option(h)
		}
	}
	return h
}

// Handle 注册原始 JSON-RPC method 路由。
// 重复注册同名 method 会覆盖之前的 handler。
func (h *Hub) Handle(method string, handler RawHandler) {
	method = strings.TrimSpace(method)
	if method == "" || handler == nil {
		return
	}
	h.mu.Lock()
	h.handlers[method] = handler
	h.mu.Unlock()
}

// Handle 注册带类型解码的 JSON-RPC method 路由。
// Go 不支持方法级类型参数，所以这里使用包级泛型函数，而不是 (*Hub).Handle[T]。
func Handle[T any](h *Hub, method string, handler func(context.Context, T) error) {
	if h == nil || handler == nil {
		return
	}
	h.Handle(method, func(ctx context.Context, params json.RawMessage) error {
		var data T
		if len(params) > 0 && string(params) != "null" {
			if err := json.Unmarshal(params, &data); err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidParams, err)
			}
		}
		return handler(ctx, data)
	})
}

// Serve 将 HTTP 请求升级为 WebSocket，并运行 JSON-RPC 会话。
// 服务端事件会以 method 为 "event" 的 notification 下发；
// 客户端消息会按 JSON-RPC method 分发到对应路由。
func (h *Hub) Serve(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{conn: conn}
	defer conn.Close()

	conn.SetReadLimit(64 * 1024)
	readDone := make(chan struct{})
	go h.readLoop(ctx, c, readDone)

	if h.source == nil {
		_ = c.writeClose(websocket.CloseGoingAway, "event source is not configured")
		return
	}

	sub, buffered := h.source.Subscribe(ctx, lastEventID(r))
	defer sub.Close()

	for _, event := range buffered {
		if !h.filter(event, r) {
			continue
		}
		if err := c.writeNotification("event", event); err != nil {
			return
		}
	}

	ping := time.NewTicker(pingInterval)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = c.writeClose(websocket.CloseGoingAway, "server shutdown")
			return
		case <-readDone:
			return
		case <-ping.C:
			if err := c.writePing(); err != nil {
				return
			}
		case event, ok := <-sub.Events():
			if !ok {
				_ = c.writeClose(websocket.CloseGoingAway, "event subscription closed")
				return
			}
			if !h.filter(event, r) {
				continue
			}
			if err := c.writeNotification("event", event); err != nil {
				return
			}
		}
	}
}

// readLoop 处理客户端发来的 JSON-RPC 消息。
// 带 id 的 request 会收到 response；不带 id 的 notification 只分发不响应。
func (h *Hub) readLoop(ctx context.Context, c *client, done chan<- struct{}) {
	defer close(done)
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		req, rpcErr := decodeRequest(raw)
		if rpcErr != nil {
			_ = c.writeResponse(response{JSONRPC: jsonrpcVersion, Error: rpcErr})
			continue
		}
		if req.ID == nil {
			_ = h.dispatch(ctx, req.Method, req.Params)
			continue
		}
		if err := h.dispatch(ctx, req.Method, req.Params); err != nil {
			_ = c.writeResponse(response{JSONRPC: jsonrpcVersion, ID: req.ID, Error: errorResponse(err)})
			continue
		}
		_ = c.writeResponse(response{JSONRPC: jsonrpcVersion, ID: req.ID, Result: map[string]bool{"ok": true}})
	}
}

// dispatch 根据 JSON-RPC method 查找并执行已注册的路由回调。
func (h *Hub) dispatch(ctx context.Context, method string, params json.RawMessage) error {
	method = strings.TrimSpace(method)
	h.mu.RLock()
	handler := h.handlers[method]
	h.mu.RUnlock()
	if handler == nil {
		return ErrMethodNotFound
	}
	return handler(ctx, params)
}

// decodeRequest 校验 wshub 需要的最小 JSON-RPC 2.0 请求信封。
func decodeRequest(raw []byte) (request, *rpcError) {
	var req request
	if err := json.Unmarshal(raw, &req); err != nil {
		return request{}, &rpcError{Code: errParseError, Message: "parse error"}
	}
	if req.JSONRPC != jsonrpcVersion || strings.TrimSpace(req.Method) == "" {
		return request{}, &rpcError{Code: errInvalidRequest, Message: "invalid request"}
	}
	return req, nil
}

// ErrInvalidParams 会映射为 JSON-RPC -32602。
var ErrInvalidParams = errors.New("invalid params")

// ErrMethodNotFound 会映射为 JSON-RPC -32601。
var ErrMethodNotFound = errors.New("method not found")

func errorResponse(err error) *rpcError {
	switch {
	case errors.Is(err, ErrInvalidParams):
		return &rpcError{Code: errInvalidParams, Message: err.Error()}
	case errors.Is(err, ErrMethodNotFound):
		return &rpcError{Code: errMethodNotFound, Message: "method not found"}
	default:
		return &rpcError{Code: errInternal, Message: err.Error()}
	}
}

// AcceptAll 是默认事件过滤器。
func AcceptAll(any, *http.Request) bool {
	return true
}

func lastEventID(r *http.Request) string {
	id := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if id == "" {
		id = strings.TrimSpace(r.URL.Query().Get("lastEventId"))
	}
	return id
}

func (c *client) writeNotification(method string, params any) error {
	return c.writeJSON(notification{JSONRPC: jsonrpcVersion, Method: method, Params: params})
}

func (c *client) writeResponse(resp response) error {
	return c.writeJSON(resp)
}

func (c *client) writeJSON(value any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	return c.conn.WriteJSON(value)
}

func (c *client) writePing() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.PingMessage, nil)
}

func (c *client) writeClose(code int, text string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	return c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, text), time.Now().Add(writeTimeout))
}
