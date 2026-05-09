# agent_chat 桌面聊天客户端设计文档

## 1. 项目定位

`agent_chat` 是 icoo-ai 的桌面聊天客户端，目标是提供一个类似 QQ / 微信 / Discord 桌面端的多会话 Agent 聊天体验。界面参考桌面聊天软件的三栏结构：最左侧为应用导航栏，中间为会话列表，右侧为当前会话内容和输入区。

技术栈：

- 桌面容器：Wails 3
- 前端构建：Vite
- 前端框架：Vue 3 + JavaScript
- 状态管理：Pinia
- UI 组件：shadcn-vue / shadcn/ui 风格组件
- 样式：Tailwind CSS + CSS Variables
- 图标：lucide-vue-next
- 后端桥接：Wails runtime bindings

核心目标：

- 支持主 agent 与多个 subagent 的清晰会话展示。
- 支持本地会话列表、消息流、工具调用状态和审计提示。
- 支持桌面端高密度信息布局，同时保持低噪音、可长期使用。
- 后续可对接 `agent_server` 的 ACP / Runtime API。

## 2. 设计参考

参考截图中的桌面聊天软件布局，保留以下关键模式：

- 左侧窄导航栏：头像、消息、联系人、收藏、文件、设置等入口。
- 中间会话列表：搜索框、会话项、未读数、最近消息、时间、选中态。
- 右侧主聊天区：顶部会话标题和操作按钮，中间消息流，底部输入栏。
- 浅色蓝灰背景：降低长时间聊天疲劳。
- 气泡消息：区分用户、agent、工具结果和系统提示。
- 顶部操作区：提供会话设置、搜索、日志、分屏等能力。

与传统聊天软件不同的是，本客户端需要突出 Agent 工作流：

- 工具调用过程需要可见但不打扰。
- subagent 需要独立会话 ID，并能从主会话跳转查看。
- Hook / approval / audit 事件需要有轻量提示。
- 长任务需要显示运行状态、取消入口和结果摘要。

## 3. 信息架构

### 3.1 全局结构

```text
AppShell
├── AppNavRail
├── ConversationSidebar
└── ChatWorkspace
    ├── ChatHeader
    ├── MessageTimeline
    └── ComposerPanel
```

### 3.2 左侧导航栏 AppNavRail

宽度建议：`60px`。

入口：

- 用户头像 / 在线状态
- 会话
- Agent 列表
- Skills
- 文件 / 工作区
- 审计日志
- 设置

交互：

- 当前入口使用高亮圆角背景。
- 有未读或待审批事件时显示红点。
- 底部放置设置、折叠和关于入口。

### 3.3 会话侧栏 ConversationSidebar

宽度建议：`260px - 300px`。

组成：

- 顶部搜索框。
- 新建会话按钮。
- 会话过滤 tabs：全部、主 Agent、Subagent、收藏、失败。
- 会话列表。

会话项字段：

```js
{
  id: 'sess_...',
  type: 'main' | 'subagent' | 'skill',
  title: '当前任务标题',
  subtitle: '最近一条消息或工具状态',
  unreadCount: 0,
  status: 'idle' | 'running' | 'failed' | 'waiting_approval',
  updatedAt: '2026-05-09T10:00:00Z',
  avatar: 'agent' | 'skill' | 'workspace'
}
```

主会话和 subagent 会话需要视觉区分：

- 主 agent：使用 `sess_` 前缀和主色头像。
- Subagent：使用 `subsess_` 前缀和小型分支标识。
- Skill subagent：在头像角标显示 skill 名称首字母。

### 3.4 主聊天区 ChatWorkspace

由三部分组成：

- `ChatHeader`：会话标题、状态、模型、操作按钮。
- `MessageTimeline`：消息、工具事件、审批事件、subagent 卡片。
- `ComposerPanel`：输入框、附件、命令、发送、模型选项。

## 4. 核心页面

### 4.1 会话聊天页

用于普通主 agent 会话。

内容类型：

- 用户消息。
- Agent 文本消息。
- 工具调用卡片。
- Hook 阻断提示。
- Approval 请求卡片。
- Subagent 派生卡片。
- Run completed / failed / cancelled 状态。

工具调用卡片设计：

```text
[tool] read_file              42ms
README.md                     OK
path: README.md, bytes: 7283
```

默认折叠，只展示工具名、状态、耗时和安全摘要。点击后展开详情，但不展示敏感大输出，和 session 持久化策略保持一致。

### 4.2 Subagent 会话页

Subagent 不应该混在主会话里作为匿名消息流，而是独立会话：

- 会话 ID 使用 `subsess_` 前缀。
- 侧栏中作为独立会话项出现。
- 主会话中展示 `SubagentRunCard`，点击跳转到对应 subagent 会话。
- subagent 会话顶部展示父会话 ID、任务来源和 skill 信息。

SubagentRunCard 字段：

```js
{
  sessionId: 'subsess_skill_review_...',
  parentSessionId: 'sess_...',
  task: 'Review internal/agent changes',
  status: 'running' | 'completed' | 'failed',
  summary: '发现 2 个风险点',
  eventCount: 18,
  startedAt: '...',
  completedAt: '...'
}
```

### 4.3 Skills 页面

用于浏览和管理 skill。

功能：

- skill 列表。
- skill 详情。
- `SKILL.md` 预览。
- resources 索引：scripts、references、assets。
- 执行 skill：创建新的 skill subagent 会话。

### 4.4 审计日志页面

用于查看关键事件：

- tool call
- approval
- hook use
- subagent run
- skill use
- network access
- MCP call

支持过滤：事件类型、会话 ID、时间、状态。

## 5. 视觉风格

### 5.1 设计方向

关键词：桌面聊天、浅蓝灰、低噪音、高密度、Agent 工作台。

整体风格接近截图中的 QQ 桌面端，但减少娱乐化装饰，增强工程工具感。

### 5.2 布局尺寸

```css
--nav-rail-width: 60px;
--conversation-sidebar-width: 280px;
--chat-header-height: 64px;
--composer-min-height: 112px;
--message-max-width: 680px;
```

### 5.3 色彩 token

```css
:root {
  --background: 204 80% 92%;
  --foreground: 210 40% 12%;
  --panel: 204 70% 88%;
  --panel-foreground: 210 40% 14%;
  --sidebar: 204 75% 86%;
  --sidebar-active: 196 92% 54%;
  --bubble-agent: 0 0% 100%;
  --bubble-user: 197 92% 76%;
  --bubble-tool: 210 40% 96%;
  --bubble-system: 45 95% 92%;
  --border: 204 32% 78%;
  --muted: 205 24% 58%;
  --danger: 0 82% 60%;
  --warning: 38 92% 50%;
  --success: 142 70% 42%;
  --ring: 196 92% 44%;
}
```

### 5.4 字体

推荐：

- 中文 UI：`Microsoft YaHei UI` / `PingFang SC` / `Noto Sans SC`
- 等宽内容：`JetBrains Mono` / `Cascadia Code`

正文字号：

- 会话列表标题：`14px / 20px`
- 最近消息：`12px / 18px`
- 聊天正文：`14px / 22px`
- 工具元信息：`12px / 18px`

### 5.5 圆角和阴影

```css
--radius-sm: 6px;
--radius-md: 10px;
--radius-lg: 16px;
--shadow-card: 0 8px 24px rgb(15 23 42 / 0.08);
--shadow-float: 0 12px 36px rgb(15 23 42 / 0.14);
```

## 6. 组件设计

### 6.1 基础 shadcn 组件

优先引入：

- Button
- Input
- Textarea
- ScrollArea
- Avatar
- Badge
- Card
- Separator
- Tooltip
- DropdownMenu
- Dialog
- Sheet
- Tabs
- Command
- Toast / Sonner

### 6.2 业务组件

```text
src/components/app/AppShell.vue
src/components/app/AppNavRail.vue
src/components/conversation/ConversationSidebar.vue
src/components/conversation/ConversationItem.vue
src/components/chat/ChatHeader.vue
src/components/chat/MessageTimeline.vue
src/components/chat/MessageBubble.vue
src/components/chat/ToolCallCard.vue
src/components/chat/ApprovalCard.vue
src/components/chat/SubagentRunCard.vue
src/components/chat/ComposerPanel.vue
src/components/skills/SkillList.vue
src/components/skills/SkillDetail.vue
src/components/audit/AuditEventTable.vue
```

### 6.3 MessageBubble

Props：

```js
{
  message: {
    id: String,
    sessionId: String,
    role: 'user' | 'assistant' | 'system',
    content: String,
    createdAt: String,
    status: 'sending' | 'streaming' | 'done' | 'failed'
  }
}
```

规则：

- 用户消息右侧对齐。
- Agent 消息左侧对齐。
- 系统消息居中弱化显示。
- streaming 状态显示光标动画。

### 6.4 ToolCallCard

Props：

```js
{
  event: {
    id: String,
    name: String,
    ok: Boolean,
    result: Object,
    startedAt: String,
    completedAt: String
  }
}
```

规则：

- 默认折叠。
- 成功使用绿色状态点。
- 失败使用红色状态点。
- 审批需要使用黄色状态点。
- 只展示 session summary 中允许的字段。

### 6.5 ApprovalCard

显示权限审批请求。

操作：

- 允许一次。
- 总是允许。
- 拒绝。
- 查看详情。

审批完成后卡片变为只读，并显示 `approval_decided` 结果。

### 6.6 ComposerPanel

功能：

- 多行输入。
- `/skill` 命令提示。
- `@subagent` 或 `@skill:name` 快捷引用。
- 附件入口。
- 发送按钮。
- 停止运行按钮。

快捷键：

- `Enter`：发送。
- `Shift + Enter`：换行。
- `Ctrl/Cmd + K`：打开命令面板。
- `Esc`：关闭弹层或取消选择。

## 7. 状态管理 Pinia

### 7.1 Store 拆分

```text
src/stores/app.js
src/stores/conversations.js
src/stores/messages.js
src/stores/runs.js
src/stores/approvals.js
src/stores/skills.js
src/stores/audit.js
```

### 7.2 conversations store

```js
export const useConversationsStore = defineStore('conversations', {
  state: () => ({
    items: [],
    activeId: null,
    filter: 'all',
    loading: false,
  }),
  getters: {
    activeConversation: (state) => state.items.find((item) => item.id === state.activeId),
  },
  actions: {
    async loadConversations() {},
    async createConversation(payload) {},
    setActive(id) {},
    upsertConversation(conversation) {},
  },
})
```

### 7.3 messages store

```js
export const useMessagesStore = defineStore('messages', {
  state: () => ({
    bySessionId: {},
    streamingBySessionId: {},
  }),
  actions: {
    appendMessage(sessionId, message) {},
    appendDelta(sessionId, delta) {},
    upsertEvent(sessionId, event) {},
    clearSession(sessionId) {},
  },
})
```

### 7.4 runs store

负责运行状态：

```js
{
  activeRuns: {
    [sessionId]: {
      status: 'running',
      startedAt: '...',
      currentTool: 'read_file',
      cancellable: true
    }
  }
}
```

## 8. Wails 3 集成设计

### 8.1 前端调用后端

前端通过 Wails bindings 调用 Go 服务：

```js
await AgentService.NewSession({ cwd })
await AgentService.Prompt({ sessionId, prompt })
await AgentService.Cancel(sessionId)
await AgentService.LoadSession(sessionId)
await AgentService.ListSessions()
```

### 8.2 事件流

建议后端将 runtime event 转换为 Wails event：

```js
EventsOn('agent:event', (event) => {
  messagesStore.upsertEvent(event.session_id, event)
})
```

事件类型：

- `run_started`
- `message_delta`
- `tool_call_started`
- `tool_call_completed`
- `approval_requested`
- `approval_decided`
- `run_completed`
- `run_failed`
- `run_cancelled`

### 8.3 后端服务建议

```go
type AgentService struct {
  Runtime agent.Runtime
}

func (s *AgentService) NewSession(ctx context.Context, req agent.NewSessionRequest) (agent.Session, error)
func (s *AgentService) Prompt(ctx context.Context, req agent.PromptRequest) error
func (s *AgentService) Cancel(ctx context.Context, sessionID string) error
func (s *AgentService) LoadSession(ctx context.Context, sessionID string) (agent.Session, error)
func (s *AgentService) ListSessions(ctx context.Context) ([]agent.Session, error)
```

`Prompt` 不直接返回完整响应，而是启动 goroutine 消费 runtime events，并推送给前端。

## 9. 路由设计

使用 Vue Router：

```text
/                      -> 默认跳转最近会话
/chats/:sessionId      -> 会话详情
/skills                -> Skills 管理
/audit                 -> 审计日志
/settings              -> 设置
```

桌面端也可以不明显暴露 URL，但内部路由仍有利于状态恢复和调试。

## 10. 目录结构建议

```text
agent_chat/
├── README.md
├── docs/
│   └── desktop-chat-design.md
├── frontend/
│   ├── index.html
│   ├── package.json
│   ├── vite.config.js
│   ├── src/
│   │   ├── main.js
│   │   ├── App.vue
│   │   ├── router/
│   │   ├── stores/
│   │   ├── components/
│   │   ├── lib/
│   │   └── styles/
│   └── components.json
├── app.go
├── main.go
└── wails.json
```

说明：

- `frontend/` 放 Vite + Vue + shadcn 前端。
- Wails Go 入口保留在 `agent_chat/` 根目录。
- 后续可通过 Go module workspace 或 replace 引用 `agent_server`。

## 11. 空状态、加载和错误

### 11.1 空状态

- 无会话：展示“创建第一个 Agent 会话”。
- 无搜索结果：展示“没有找到匹配会话”。
- 无 skill：展示“当前没有可用 Skill”。

### 11.2 加载状态

- 会话列表 skeleton。
- 消息流首次加载 skeleton。
- streaming 使用打字光标。
- 工具运行显示 spinner 和当前工具名。

### 11.3 错误状态

- 模型调用失败：消息流底部展示可重试卡片。
- 工具失败：工具卡片红色状态。
- 权限拒绝：审批卡片变为灰色只读。
- Wails bridge 断开：顶部全局 toast。

## 12. 可访问性

- 所有按钮必须有 `aria-label`。
- 输入框支持键盘操作和 focus ring。
- 颜色对比至少满足 WCAG AA。
- 工具状态不能只依赖颜色，需配合文本或图标。
- 消息列表使用合理的 tab 顺序。
- 长列表使用虚拟滚动时，要保持屏幕阅读器可读摘要。

## 13. 性能策略

- 会话列表使用分页或虚拟滚动。
- 消息流超过一定数量后使用虚拟滚动。
- streaming delta 合并到 animation frame 批量更新。
- 工具详情默认折叠，避免大 JSON 渲染。
- Pinia store 中只保存必要状态，大型详情按需加载。

## 14. 分阶段实现计划

### Phase 1：静态壳和基础聊天

- 初始化 Wails 3 + Vite + Vue + JavaScript。
- 接入 Tailwind 和 shadcn-vue。
- 实现 AppShell、导航栏、会话列表、聊天区。
- 使用 mock 数据展示完整 UI。

验收：

- 桌面窗口能启动。
- 三栏布局稳定。
- 消息、工具卡片、审批卡片能正常展示 mock 数据。

### Phase 2：接入 Agent Runtime

- 实现 `AgentService`。
- 支持 NewSession、Prompt、Cancel、LoadSession、ListSessions。
- Wails event 推送 runtime event。
- Pinia store 消费事件流。

验收：

- 可以创建真实会话。
- 可以看到 message delta streaming。
- 可以看到 tool call started/completed。

### Phase 3：Subagent 与 Skills

- subagent 独立会话展示。
- `SubagentRunCard` 跳转对应 `subsess_` 会话。
- Skills 页面展示和执行。
- `/skill` 命令补全。

验收：

- 主会话与 subagent 会话前缀区分清楚。
- skill 执行生成独立 subagent 会话。
- 会话侧栏能过滤 subagent 会话。

### Phase 4：审计和稳定性

- 审计日志页面。
- 工具失败和重试展示。
- MCP / web retry attempt 展示。
- 设置页接入配置检查。

验收：

- 能按事件类型过滤审计日志。
- 能查看 tool / hook / approval / subagent 关键事件。
- 错误和取消流程有明确反馈。

## 15. 验收标准

- Wails 桌面应用可启动。
- UI 接近桌面聊天软件体验，支持三栏布局。
- 主会话 `sess_` 和 subagent 会话 `subsess_` 明确区分。
- 消息流支持 streaming、工具事件、审批事件、运行状态。
- Pinia store 能恢复会话列表和当前会话。
- 工具输出不在前端默认展开大内容。
- shadcn 组件、Tailwind token 和业务组件分层清晰。
- 键盘操作和基础无障碍可用。
