# agent_chat

`agent_chat` 是 icoo-ai 的桌面聊天客户端目录，当前阶段提供 Wails 3 + Vite + Vue 3 + JavaScript + Pinia + Tailwind 的最小可运行骨架。

## 当前能力

- 三栏桌面聊天布局：左侧导航、中间会话列表、右侧聊天区。
- 展示消息气泡、工具调用摘要卡片、审批卡片、subagent 运行卡片。
- 使用无边框桌面窗口，并提供全局 header 承载拖拽、品牌、当前模块和窗口操作。
- 工具结果只展示安全摘要字段，例如输出大小、摘要 hash、是否落盘，不保存完整大输出。
- 前端 store 统一通过 `agentBridge` 读写 Wails bindings，不再依赖 mock fallback。
- Go bridge 已优先对接本地 `agent_gateway`（HTTP + SSE），并在启动阶段自动尝试唤醒 gateway。
- `Prompt` 已兼容 gateway 结构化响应（`run/messages/approval`）并标准化为前端 `MessageEvent`。
- 已沉淀 QQ 桌面端风格 UED 规范，并在 `frontend/src/styles/globals.css` 中提供统一 `qq-*` CSS token 和组件类。

## 本地运行

```bash
wails3 dev
```

网关与日志配置统一写入仓库根目录 `chat.toml`：

```toml
gateway_binary_path = "E:/codes/icoo_ai/agent_gateway/dist/agent-gateway.exe"
gateway_host = "127.0.0.1"
gateway_port = 17889
log_level = "info"
log_format = "text"
```

`agent_chat` 启动时读取该文件；设置页保存后也会回写同一文件。

前端单独调试时可在 `frontend/` 目录运行 `npm run dev`。

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
cd frontend
npm run build:script
```

## 构建验证

```bash
wails3 build
go test ./...
```

说明：Go 命令仅用于测试；桌面应用统一使用 `wails3 build` 构建。`wails3 build` 会先生成 Wails bindings，再构建前端并产出 `bin/agent_chat.exe`。

## 联调冒烟

仓库根目录执行：

```powershell
.\scripts\smoke-gateway-chat.ps1
```

该脚本会自动拉起 gateway、完成 session/prompt/cancel 闭环验证并回收进程。

构建脚本参数：

- `-Clean`：构建前清理 `frontend/dist/` 和 `bin/`。
- `-RunTests`：构建前执行 `go test ./...`。
- `-NoColour`：禁用 Wails 彩色输出。

## 目录说明

- `frontend/src/components/`：桌面聊天 UI 组件。
- `frontend/src/stores/`：Pinia 状态管理。
- `frontend/src/services/agentBridge.js`：未来 Wails bindings 的前端适配层。
- `frontend/src/bindings/`：Wails 3 生成的 bridge bindings。
- `internal/bridge/`：Go bridge DTO 与 gateway 对接服务。
- `docs/`：中文设计文档和多 worker 阶段计划。
- `docs/ued-guidelines.md`：桌面聊天 UI 的 UED 规范。
