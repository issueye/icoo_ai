# agent_chat

`agent_chat` 是 icoo-ai 的桌面聊天客户端目录，当前阶段提供 Wails 3 + Vite + Vue 3 + JavaScript + Pinia + Tailwind 的最小可运行骨架。

## 当前能力

- 三栏桌面聊天布局：左侧导航、中间会话列表、右侧聊天区。
- 使用 mock 数据展示 `sess_` 主会话与 `subsess_` subagent 独立会话。
- 展示消息气泡、工具调用摘要卡片、审批卡片、subagent 运行卡片。
- 工具结果只展示安全摘要字段，例如输出大小、摘要 hash、是否落盘，不保存完整大输出。
- Go bridge 已提供 mock DTO 和方法边界，后续可接入 `agent_server` Runtime。

## 本地运行

```bash
npm install
npm run dev
```

## 构建验证

```bash
npm run build
go test ./...
```

说明：Go 命令在 `agent_chat/` 目录执行；现阶段 Wails 3 CLI 入口保留为草案，普通前端构建和 Go bridge 编译已可验证。

## 目录说明

- `src/components/`：桌面聊天 UI 组件。
- `src/stores/`：Pinia 状态管理。
- `src/services/mockData.js`：最小 mock 数据。
- `src/services/agentBridge.js`：未来 Wails bindings 的前端适配层。
- `internal/bridge/`：Go bridge DTO 与 mock service。
- `docs/`：中文设计文档和多 worker 阶段计划。
