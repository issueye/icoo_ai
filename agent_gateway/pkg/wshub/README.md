# wshub

`wshub` 提供通用的 WebSocket 事件通道和客户端消息路由。
所有应用层消息都使用 JSON-RPC 2.0 信封。

该模块位于 `pkg` 下，不直接依赖业务层代码。业务层需要通过
`EventSource` 和 `Subscription` 接口把自己的事件总线适配进来。

## 接入地址

网关注册的地址：

```text
GET /v1/events/ws
```

HTTP 请求会升级为 WebSocket。支持以下查询过滤参数：

```text
/v1/events/ws?sessionId=sess_1&agentId=agent_1&lastEventId=evt_1
```

## 服务端事件

事件源中的事件会以 JSON-RPC notification 推送，method 固定为 `event`：

```json
{
  "jsonrpc": "2.0",
  "method": "event",
  "params": {
    "id": "evt_1",
    "type": "message",
    "agentId": "agent_1",
    "sessionId": "sess_1",
    "payload": {}
  }
}
```

当连接参数中设置了 `sessionId` 或 `agentId` 时，事件会按这些条件过滤。
连接建立后会回放 `lastEventId` 之后的缓冲事件。

过滤逻辑由业务层通过 `WithFilter` 注入，`wshub` 本身不理解业务字段：

```go
hub := wshub.New(source, wshub.WithFilter(func(event any, r *http.Request) bool {
    return true
}))
```

## 事件源适配

业务层需要实现事件源接口：

```go
type EventSource interface {
    Subscribe(ctx context.Context, lastEventID string) (wshub.Subscription, []any)
}

type Subscription interface {
    Events() <-chan any
    Close()
}
```

`Subscribe` 返回两部分内容：

- `Subscription`: 实时事件订阅。
- `[]any`: 连接建立时需要回放的缓冲事件。

## 客户端请求

客户端可以发送 JSON-RPC request 或 notification：

```json
{
  "jsonrpc": "2.0",
  "id": "req_1",
  "method": "message block",
  "params": {
    "content": "hello"
  }
}
```

带 `id` 的 request 会收到响应：

```json
{
  "jsonrpc": "2.0",
  "id": "req_1",
  "result": {
    "ok": true
  }
}
```

不带 `id` 的 notification 只会被分发，不会返回响应。

## 路由注册

普通路由优先使用带类型的注册助手：

```go
type MessageBlock struct {
    Content string `json:"content"`
}

wshub.Handle[MessageBlock](hub, "message block", func(ctx context.Context, data MessageBlock) error {
    return nil
})
```

Go 不支持方法级泛型参数，所以这里使用包级函数，而不是 `hub.Handle[T](...)`。

需要自定义 JSON 解码时，可以使用原始路由：

```go
hub.Handle("raw method", func(ctx context.Context, params json.RawMessage) error {
    return nil
})
```

## 错误映射

`wshub` 返回标准 JSON-RPC 错误码：

- `-32700`: 解析错误
- `-32600`: 非法请求
- `-32601`: 方法不存在
- `-32602`: 参数非法
- `-32603`: 内部错误
