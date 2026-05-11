# agent_chat 与 gateway 对接优化开发计划（不兼容改造）

## 1. 目标与原则

### 1.1 目标
- 将管理模块数据中心化到 `agent_gateway`，`agent_chat` 仅负责 UI 与桥接调用。
- 管理模块统一为网关 HTTP API 驱动，消除本地双写与兜底逻辑。
- 建立统一 CRUD 范式（查询 + 分页表格 + 弹窗保存即持久化）。

### 1.2 改造原则（不兼容）
- 不保留旧版本地存储兼容路径。
- 不保留网关失败时的本地回退逻辑。
- 一次性收敛接口语义，允许旧版本前端/配置失效。

---

## 2. 现状与问题

### 2.1 已完成现状
- 前端管理模块（Agent / MCP / 定时任务）已切换统一 CRUD 页面模式。
- `agent_chat` 通过网关 API `/v1/management/settings` 读取与更新管理数据。
- `agent_chat` 本地管理存储代码已删除，`chat.toml` 不再持久化管理数据块。

### 2.2 核心问题
- `agents` 存在双语义来源：`/v1/agents` 与 `/v1/management/settings.agents`。
- 网关管理配置当前为内存态，重启后可能丢失。
- `channels` 仍在 `agent_chat` 本地配置，不完全中心化。

---

## 3. 目标架构（最终态）

- `agent_gateway`
  - 负责管理配置持久化（Agent/MCP/定时任务/渠道）。
  - 暴露统一配置 API：`GET/PUT /v1/management/settings`。
  - `/v1/agents` 由同一配置源派生，保持语义一致。
  - 启动参数策略：CLI 仅允许 `host/port`，其余运行配置统一来自网关配置文件。
- `agent_chat`
  - 所有管理页面仅调用 `GetAppSettings/UpdateAppSettings`（底层转发网关 API）。
  - 本地仅保存网关启动参数与桥接运行参数。

---

## 4. 开发范围

### 4.1 网关侧（必须）
1. 引入管理配置持久化层（SQLite 或 JSON 文件，推荐 SQLite）。
2. 将 `ManagementSettings` 的读写改为持久化实现。
3. 统一 `agents` 数据源：
   - `/v1/agents` 从 `ManagementSettings.agents` 派生。
4. 增加配置版本号与更新时间字段（便于审计与前端展示）。
5. CLI 参数收敛（不兼容）：
   - 保留：`-host`、`-port`
   - 移除：`-acp-enabled`、`-acp-command`、`-acp-args`、`-acp-pool-size`、`-data-dir` 等运行配置项
   - 以上参数统一迁移至网关配置文件读取。

### 4.2 agent_chat 侧（必须）
1. 继续保持无本地管理存储，移除残留兼容代码。
2. `GetAppSettings` / `UpdateAppSettings` 失败直接透出，不回退。
3. 管理页交互统一为“弹窗保存即持久化”，失败回滚 UI。

### 4.3 前端（建议）
1. 管理页统一错误态文案：
   - 网关不可达
   - 鉴权失败
   - 参数校验失败
2. 新增“配置来源：Gateway”标识，避免误解为本地配置。

---

## 5. 分阶段计划

## 阶段 A：语义收敛（1-2 天）
- 完成 `agents` 单一数据源改造。
- 修改 `/v1/agents` 实现，基于 `management/settings.agents` 返回。
- 验收：
  - Agent 管理页新增/修改后，会话模式列表与 `/v1/agents` 立刻一致。

## 阶段 B：持久化落地（1-2 天）
- 将 gateway 管理配置从内存迁移到持久化存储。
- 增加启动加载、更新写回、并发写保护。
- 验收：
  - 网关重启后，管理配置完整保留。

## 阶段 C：全链路清理（1 天）
- 删除遗留兼容分支、无用结构体字段、无效测试桩。
- 更新文档与测试用例。
- 验收：
  - 全量测试通过，且无旧存储路径引用。

---

## 6. 数据与接口变更清单（不兼容）

1. 删除本地管理数据持久化能力（已执行）。
2. `chat.toml` 不再承载 Agent/MCP/定时任务配置（已执行）。
3. 网关管理配置 API 成为唯一写入口（已执行）。
4. `/v1/agents` 返回规则将调整为基于管理配置派生（待执行）。
5. 网关 CLI 参数不再支持除 `host/port` 外的运行配置项（待执行）。

---

## 6.1 配置来源矩阵（新增）

| 配置项 | 变更前来源 | 变更后来源 |
|---|---|---|
| host | CLI / 默认值 | CLI（可覆盖配置文件） |
| port | CLI / 默认值 | CLI（可覆盖配置文件） |
| acp.enabled | CLI | 网关配置文件 |
| acp.command | CLI | 网关配置文件 |
| acp.args | CLI | 网关配置文件 |
| acp.pool_size | CLI | 网关配置文件 |
| mcpServers | agent_chat 本地/桥接 | 网关配置文件 + 管理 API |
| scheduleTasks | agent_chat 本地/桥接 | 网关配置文件 + 管理 API |
| agents | gateway 内置/多源 | 网关配置文件 + 管理 API（单一来源） |

---

## 7. 测试计划

### 7.1 单元测试
- gateway:
  - 管理配置读写、重启恢复、并发更新。
  - `/v1/agents` 与 `/v1/management/settings` 一致性。
- agent_chat:
  - 网关失败直出错误，不回退。
  - 更新后回读一致。

### 7.2 集成测试
1. 启动 gateway + agent_chat。
2. 在 MCP/定时任务/Agent 页面执行：新增、编辑、删除。
3. 重启 gateway。
4. 验证配置仍存在且前端回显一致。
5. 启动参数回归：
   - 使用旧参数（如 `-acp-command`）启动应失败并提示参数不支持。
   - 仅传 `-host/-port` 启动成功，且 ACP 行为由配置文件决定。

### 7.3 验收标准
- 管理配置唯一来源为 gateway。
- 无本地兼容回退逻辑。
- 前后端构建与测试全部通过。

---

## 8. 风险与控制

1. 风险：存量环境依赖旧本地配置。
   - 控制：发布前提供迁移脚本（一次性导入到 gateway 存储）。
2. 风险：网关不可达导致管理页不可用。
   - 控制：明确错误提示 + 重试 + 网关状态可视化。
3. 风险：`agents` 语义切换影响会话创建。
   - 控制：阶段 A 完成后补会话创建回归测试。

---

## 9. 交付物

1. 代码
- gateway 管理配置持久化实现。
- `/v1/agents` 语义收敛实现。
- agent_chat 清理后的 bridge 与前端管理模块。

2. 文档
- 本优化计划文档。
- 接口契约文档（管理配置 API 字段与错误码）。
- 发布迁移说明（旧配置迁移到 gateway）。
