# agent_chat 与 agent_gateway 对接详情梳理（2026-05-11 历史文档，已被重构取代）

## 1. 文档状态

本文最初记录的是 2026-05-11 当时的 `agent_chat` 与 `agent_gateway` 对接状态。该内容已经被后续 breaking refactor 取代，不能再作为当前接口契约依据。

当前 `agent_gateway` 已重构为：

- Gin HTTP 路由
- GORM 数据访问
- SQLite no cgo 本地存储
- WebSocket 事件通道
- ACP 执行链路

旧版设计中的以下接口与契约已经废弃：

- 不再提供 `GET /v1/events/stream` SSE 事件流。
- 不再提供全局 `GET/POST /v1/sessions`、`/v1/sessions/:id/*` 会话接口。
- 不再提供全局 `GET /v1/runs`。
- 不再提供 `GET/POST /v1/management/settings`。

后续排障、联调、测试与文档更新，应以本文下方“当前接口契约”为准。

---

## 2. 当前对接结论

`agent_chat` 当前通过本地发现文件连接 `agent_gateway`，使用 Bearer Token 访问 `/v1/*` 接口，并通过 WebSocket `GET /v1/events` 接收网关事件。

会话生命周期已调整为 Agent-scoped 模型：会话必须挂在具体 Agent 下创建、发送 prompt、取消和删除。客户端不应再调用旧的全局 session 或 run 接口。

---

## 3. 启动与发现链路

### 3.1 `agent_chat` 启动流程

1. `ServiceStartup` 保障本地网关可用。
2. 若已有网关实例可探活，则复用；否则拉起 `agent_gateway` 并轮询健康状态。
3. 启动后读取 `endpoint.json` 和 token，生成网关代理客户端。
4. 使用当前事件通道 `GET /v1/events` 建立 WebSocket 连接。

### 3.2 `agent_gateway` 运行时行为

1. 网关启动后绑定本地地址，生成随机或指定端口。
2. 写入运行时发现文件供 `agent_chat` 读取。
3. `/v1/*` 统一 Bearer Token 鉴权。
4. 使用 Gin 承载 HTTP/WebSocket 路由。
5. 使用 GORM + SQLite no cgo 持久化本地状态。
6. 通过 ACP 承接 Agent prompt 执行与事件产出。

---

## 4. 当前接口契约

### 4.1 发现与基础资源

`agent_chat` 可调用：

- `GET /v1/agents`
- `GET /v1/skills`
- `GET /v1/approvals`
- `POST /v1/approvals/:id/decision`

### 4.2 Agent-scoped 会话接口

当前会话接口必须带 Agent ID：

- `POST /v1/agents/:id/sessions`
- `POST /v1/agents/:id/sessions/:sessionId/prompts`
- `POST /v1/agents/:id/sessions/:sessionId/cancel`
- `DELETE /v1/agents/:id/sessions/:sessionId`

这些接口替代了旧版全局会话接口：

- 废弃：`GET /v1/sessions`
- 废弃：`POST /v1/sessions`
- 废弃：`GET /v1/sessions/:id`
- 废弃：`POST /v1/sessions/:id/prompt`
- 废弃：`POST /v1/sessions/:id/cancel`
- 废弃：`GET /v1/sessions/:id/messages`

### 4.3 事件接口

当前事件接口为：

- `GET /v1/events`

该接口为 WebSocket 连接，不是 SSE。旧版 `GET /v1/events/stream` 已废弃，客户端不应再依赖 `Last-Event-ID`、SSE keep-alive 注释帧或 SSE 回放语义。

事件消费模型应按 WebSocket 连接生命周期处理：

- 建连时携带 Bearer Token。
- 连接断开后由客户端按当前策略重连。
- 事件内容按网关当前事件 schema 映射到 `agent_chat` 前端事件。

### 4.4 已移除接口

以下旧接口不属于当前设计：

- `GET /v1/events/stream`
- `GET /v1/runs`
- `GET /v1/management/settings`
- `POST /v1/management/settings`
- 所有全局 `/v1/sessions/*` 接口

---

## 5. 事件对接细节

### 5.1 网关侧

网关通过 WebSocket `GET /v1/events` 推送事件。事件来源包括 Agent 会话执行、ACP 过程、审批、工具调用和运行状态变化等。

该通道替代旧 SSE 事件流，不再描述以下旧行为：

- `Last-Event-ID` header 续传
- `lastEventId` query 回放
- SSE comment keep-alive
- SSE ring buffer replay

### 5.2 `agent_chat` 侧

`agent_chat` 应维护 WebSocket 订阅状态，并在连接失败或断开时进入重连流程。鉴权失败、网关不可达和连接异常应映射到现有网关状态机。

旧版 `StreamEventsWithFilter` / SSE 语义如果仍出现在注释、测试或外部文档中，应按 WebSocket 事件流重新命名和改写。

---

## 6. 会话与运行模型变化

旧版文档将 session 和 run 视作全局资源：

- 全局列出 sessions
- 全局创建 session
- 全局查询 runs
- 通过 `/v1/sessions/:id/prompt` 触发执行

当前设计中，session 是 Agent-scoped 资源：

- 创建会话时必须指定 Agent：`POST /v1/agents/:id/sessions`
- 发送 prompt 时必须同时指定 Agent 和 Session：`POST /v1/agents/:id/sessions/:sessionId/prompts`
- 取消执行绑定到具体 Agent Session：`POST /v1/agents/:id/sessions/:sessionId/cancel`
- 删除会话也绑定到具体 Agent Session：`DELETE /v1/agents/:id/sessions/:sessionId`

因此，客户端状态、缓存 key、错误日志和前端事件关联字段都应同时保留 `agentId` 与 `sessionId`，避免继续假设 `sessionId` 是全局入口。

---

## 7. 配置与管理接口变化

旧版关于 `/v1/management/settings` 的描述已失效。当前不应通过该接口读取或修改网关设置。

当前配置来源应以实际启动参数、配置文件和运行时发现文件为准。若需要新增管理能力，应在新接口设计中显式定义，不应复用已移除的 `/v1/management/settings` 路径。

---

## 8. 迁移检查清单

更新或排查 `agent_chat` / `agent_gateway` 对接时，优先检查：

1. 是否仍调用 `GET /v1/events/stream`。如有，应迁移到 WebSocket `GET /v1/events`。
2. 是否仍调用全局 `/v1/sessions/*`。如有，应迁移到 `/v1/agents/:id/sessions/*`。
3. 是否仍调用 `GET /v1/runs`。如有，应改为从会话事件或当前 Agent-scoped 状态模型获取信息。
4. 是否仍调用 `/v1/management/settings`。如有，应移除或改为当前配置来源。
5. 日志、测试、README、冒烟脚本是否仍宣称 SSE、global sessions/runs 或 management settings 可用。

---

## 9. 历史内容处理说明

本文件保留 2026-05-11 日期是为了延续文档位置和历史索引，但原始结论已经不再适用。尤其是旧文档中的以下判断已被当前重构推翻：

- “SSE 事件流接入可运行”
- “接口路径与方法当前已对齐”
- “`GET /v1/runs` 属于当前调用矩阵”
- “SSE 回放边界风险是当前主要事件流风险”
- “`/v1/management/settings` 是可对接的管理设置接口”

当前集成判断应基于 Gin + GORM + SQLite(no cgo) + WebSocket + ACP 的新网关实现，以及 Agent-scoped session API。
