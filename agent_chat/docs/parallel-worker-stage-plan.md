# agent_chat 多 Worker 并行协同阶段计划

> 本计划用于并行推进 `agent_chat` 桌面聊天客户端。执行时每个 worker 必须只修改自己负责的文件集合，避免互相覆盖。

## 1. 目标

基于现有 `agent_chat/docs/desktop-chat-design.md`，启动 Wails 3 + Vite + Vue 3 + JavaScript + Pinia + shadcn 风格 UI 的桌面聊天客户端骨架开发。

阶段目标不是一次性完成全部功能，而是先形成可运行、可扩展、可并行迭代的客户端基础：

- Wails 桌面应用骨架可启动。
- 前端 Vite/Vue/Pinia/Tailwind 结构清晰。
- 桌面聊天三栏布局可见。
- 使用 mock 数据展示会话、消息、工具卡片、subagent 卡片。
- 预留后端 bridge 接口，用于后续对接 `agent_server` Runtime。

## 2. 当前基线

已有目录：

```text
agent_chat/
├── README.md
└── docs/
    └── desktop-chat-design.md
```

`agent_server/` 中已有 Go Agent 服务能力，包括 Runtime、session、tool、subagent、skill、audit、MCP 等模块。

## 3. 并行策略

采用 4 个 worker 并行推进，每个 worker 拥有明确写入范围。

| Worker | 名称 | 负责范围 | 不允许修改 |
|---|---|---|---|
| A | Shell / Tooling Worker | Wails/Vite/Tailwind/shadcn 基础工程文件 | 业务组件、store、Go bridge |
| B | UI Layout Worker | 三栏布局和聊天业务组件 | package.json、Go 文件、store |
| C | State / Mock Data Worker | Pinia stores、router、mock service、类型约定 | 业务组件样式、Go 文件 |
| D | Wails Bridge Worker | Wails Go 入口、bridge service、wails 配置草案 | 前端组件、Pinia store |

主线程负责：

- 写入阶段计划。
- 启动 worker。
- 处理冲突。
- 最终集成。
- 跑测试和构建。

## 4. 文件所有权

### Worker A：工程骨架

负责创建或修改：

```text
agent_chat/package.json
agent_chat/index.html
agent_chat/vite.config.js
agent_chat/jsconfig.json
agent_chat/components.json
agent_chat/postcss.config.js
agent_chat/tailwind.config.js
agent_chat/src/main.js
agent_chat/src/App.vue
agent_chat/src/styles/globals.css
agent_chat/src/lib/utils.js
```

要求：

- 使用 Vue 3 + JavaScript。
- 使用 Vite。
- 配置 Pinia 安装入口，但 store 具体实现由 Worker C 负责。
- 配置 Tailwind CSS 和 shadcn 风格 CSS variables。
- `App.vue` 只负责挂载顶层 layout，占位导入可用组件。

验收：

- `npm install` 后应可执行 `npm run dev`。
- `npm run build` 应完成前端构建。

### Worker B：UI 组件

负责创建：

```text
agent_chat/src/components/app/AppShell.vue
agent_chat/src/components/app/AppNavRail.vue
agent_chat/src/components/conversation/ConversationSidebar.vue
agent_chat/src/components/conversation/ConversationItem.vue
agent_chat/src/components/chat/ChatWorkspace.vue
agent_chat/src/components/chat/ChatHeader.vue
agent_chat/src/components/chat/MessageTimeline.vue
agent_chat/src/components/chat/MessageBubble.vue
agent_chat/src/components/chat/ToolCallCard.vue
agent_chat/src/components/chat/ApprovalCard.vue
agent_chat/src/components/chat/SubagentRunCard.vue
agent_chat/src/components/chat/ComposerPanel.vue
```

要求：

- 参考桌面聊天软件截图，做浅蓝灰三栏布局。
- 默认使用 mock 数据从 Pinia store 读取。
- 明确展示 `sess_` 主会话与 `subsess_` subagent 会话的差异。
- 工具卡片默认折叠风格，展示安全摘要字段。

验收：

- 页面打开后能看到左侧导航栏、中间会话列表、右侧聊天区。
- 能展示用户消息、assistant 消息、tool card、approval card、subagent card。

### Worker C：状态管理与 mock 数据

负责创建：

```text
agent_chat/src/router/index.js
agent_chat/src/stores/app.js
agent_chat/src/stores/conversations.js
agent_chat/src/stores/messages.js
agent_chat/src/stores/runs.js
agent_chat/src/stores/approvals.js
agent_chat/src/stores/skills.js
agent_chat/src/stores/audit.js
agent_chat/src/services/mockData.js
agent_chat/src/services/agentBridge.js
```

要求：

- 使用 Pinia。
- 提供 mock conversation、message、tool event、approval event、subagent run 数据。
- `agentBridge.js` 封装未来 Wails bindings 调用；当前允许 fallback 到 mock。
- store API 稳定，UI worker 只通过 store 读取数据。

验收：

- `conversations` store 能返回会话列表和 active conversation。
- `messages` store 能返回 active session messages/events。
- `runs` store 能展示当前运行状态。

### Worker D：Wails Bridge 草案

负责创建：

```text
agent_chat/go.mod
agent_chat/main.go
agent_chat/app.go
agent_chat/wails.json
agent_chat/internal/bridge/agent_service.go
agent_chat/internal/bridge/types.go
```

要求：

- 以 Wails 3 为目标组织代码。
- 不直接复制 `agent_server` 内部实现。
- bridge 类型和方法先做清晰边界，可使用 mock 返回值。
- 预留后续通过 workspace / replace 引用 `agent_server` 的空间。

验收：

- Go 代码可格式化。
- 如本地 Wails 3 CLI 不可用，也必须保证普通 Go 编译边界尽量清晰。

## 5. 主线程集成顺序

1. 合并 Worker A 工程骨架。
2. 合并 Worker C store 和 mock 数据。
3. 合并 Worker B UI 组件，并修导入路径。
4. 合并 Worker D bridge 草案。
5. 运行前端安装/构建。
6. 运行 Go 格式化/测试。
7. 更新 `agent_chat/README.md` 说明启动方式。

## 6. 阶段验收标准

本阶段完成时至少满足：

- `agent_chat/docs/parallel-worker-stage-plan.md` 存在。
- `agent_chat` 有明确 Wails + Vite 项目骨架。
- 前端有三栏聊天 UI 初版。
- Pinia store 有 mock 数据并能驱动 UI。
- subagent 会话在 UI 中使用 `subsess_` 前缀展示。
- 后端 bridge 有清晰接口草案。
- 若依赖安装成功，`npm run build` 通过。
- 若 Go/Wails 依赖可用，Go 侧至少能 `gofmt` 并通过基础编译检查。

## 7. 风险与约束

- Wails 3 API 可能和 Wails 2 不兼容，Worker D 不应硬编码不确定 API 到核心业务逻辑。
- shadcn-vue 组件可以先用本地轻量组件模拟，不阻塞整体布局。
- 不要让 UI 直接依赖真实 agent_server；先通过 `agentBridge.js` 适配层隔离。
- 不要在前端 store 中保存完整大工具输出，保持与 session summary 策略一致。
- worker 不得修改彼此拥有的文件。

## 8. 立即开始的任务

主线程立即启动 4 个 worker：

- Worker A：创建前端工程骨架。
- Worker B：创建桌面聊天 UI 组件。
- Worker C：创建 Pinia stores 和 mock 数据。
- Worker D：创建 Wails bridge 草案。

主线程在 worker 执行期间负责维护计划和最终集成。
