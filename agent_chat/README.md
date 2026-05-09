# agent_chat

`agent_chat` 是 icoo-ai 的桌面聊天客户端目录，当前阶段提供 Wails 3 + Vite + Vue 3 + JavaScript + Pinia + Tailwind 的最小可运行骨架。

## 当前能力

- 三栏桌面聊天布局：左侧导航、中间会话列表、右侧聊天区。
- 使用 mock 数据展示 `sess_` 主会话与 `subsess_` subagent 独立会话。
- 展示消息气泡、工具调用摘要卡片、审批卡片、subagent 运行卡片。
- 使用无边框桌面窗口，并提供全局 header 承载拖拽、品牌、当前模块和窗口操作。
- 工具结果只展示安全摘要字段，例如输出大小、摘要 hash、是否落盘，不保存完整大输出。
- 前端 store 统一通过 `agentBridge` 读写；浏览器开发态 fallback 到 mock，Wails 打包态可使用生成 bindings。
- Go bridge 已提供 `NewSession`、`Prompt`、`Cancel`、`LoadSession`、列表查询和审批决策的 mock 方法边界，后续可接入 `agent_server` Runtime。
- 已沉淀 QQ 桌面端风格 UED 规范，并在 `src/styles/globals.css` 中提供统一 `qq-*` CSS token 和组件类。

## 本地运行

```bash
npm install
npm run dev
```

桌面应用构建：

```bash
wails3 build
```

构建脚本：

```bash
./scripts/build.ps1
```

等价 npm 入口：

```bash
npm run build
npm run build:script
```

## 构建验证

```bash
wails3 build
go test ./...
```

说明：Go 命令仅用于测试；桌面应用统一使用 `wails3 build` 构建。`wails3 build` 会先生成 Wails bindings，再构建前端并产出 `bin/agent_chat.exe`。

构建脚本参数：

- `-Clean`：构建前清理 `dist/` 和 `bin/`。
- `-RunTests`：构建前执行 `go test ./...`。
- `-NoColour`：禁用 Wails 彩色输出。

## 目录说明

- `src/components/`：桌面聊天 UI 组件。
- `src/stores/`：Pinia 状态管理。
- `src/services/mockData.js`：最小 mock 数据。
- `src/services/agentBridge.js`：未来 Wails bindings 的前端适配层。
- `src/bindings/`：Wails 3 生成的 bridge bindings。
- `internal/bridge/`：Go bridge DTO 与 mock service。
- `docs/`：中文设计文档和多 worker 阶段计划。
- `docs/ued-guidelines.md`：桌面聊天 UI 的 UED 规范。
