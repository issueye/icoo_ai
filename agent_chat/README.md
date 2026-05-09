# agent_chat

`agent_chat` 是 icoo-ai 的桌面聊天客户端目录，当前阶段提供 Wails 3 + Vite + Vue 3 + JavaScript + Pinia + Tailwind 的最小可运行骨架。

## 当前能力

- 三栏桌面聊天布局：左侧导航、中间会话列表、右侧聊天区。
- 使用 mock 数据展示 `sess_` 主会话与 `subsess_` subagent 独立会话。
- 展示消息气泡、工具调用摘要卡片、审批卡片、subagent 运行卡片。
- 工具结果只展示安全摘要字段，例如输出大小、摘要 hash、是否落盘，不保存完整大输出。
- 前端 store 统一通过 `agentBridge` 读写；浏览器开发态 fallback 到 mock，Wails 打包态可使用生成 bindings。
- Go bridge 已提供 `NewSession`、`Prompt`、`Cancel`、`LoadSession`、列表查询和审批决策的 mock 方法边界，后续可接入 `agent_server` Runtime。

## 本地运行

```bash
npm install
npm run dev
```

桌面壳构建：

```bash
wails3 build
```

## 构建验证

```bash
npm run build
go test ./...
wails3 build
```

说明：Go 命令在 `agent_chat/` 目录执行；`wails3 build` 会先生成 Wails bindings，再构建前端并产出 `bin/agent_chat.exe`。

## 目录说明

- `src/components/`：桌面聊天 UI 组件。
- `src/stores/`：Pinia 状态管理。
- `src/services/mockData.js`：最小 mock 数据。
- `src/services/agentBridge.js`：未来 Wails bindings 的前端适配层。
- `src/bindings/`：Wails 3 生成的 bridge bindings。
- `internal/bridge/`：Go bridge DTO 与 mock service。
- `docs/`：中文设计文档和多 worker 阶段计划。
