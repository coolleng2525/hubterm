# Windows 本机终端

HubTerm 可以通过 Windows 节点上的 Agent 启动本机交互式 Shell。该功能使用 Agent WebSocket 通道，不依赖 Windows OpenSSH Server，也不需要保存 Windows 登录密码。

## 支持的 Shell

Agent 每次上报时探测当前系统真实存在的程序，只在节点页面显示可用项：

| Shell | 探测方式 |
|---|---|
| PowerShell 7 | `pwsh.exe` 位于 `PATH` |
| Windows PowerShell | `powershell.exe` 位于 `PATH` |
| Command Prompt | `cmd.exe` 位于 `PATH` |
| Git Bash | `bash.exe` 位于 `PATH`，或位于标准 Git for Windows 安装目录 |

## 使用方法

1. 在 Windows 节点安装并启动最新的 `hubterm-agent.exe`。
2. 等待 Agent 完成一次节点上报。
3. 打开节点详情页，在 Shell 下拉框中选择终端类型。
4. 点击“本机终端”。HubTerm 创建会话并进入共享终端页面。

本机终端属于有副作用的远程操作，启动和关闭 API 只允许 `admin` 与 `operator` 角色。浏览器必须显式订阅对应的节点和会话，终端输入也会校验会话归属。

## 部署 Windows Agent

交叉编译示例：

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o hubterm-agent-windows-amd64.exe ./cmd/agent
```

替换正在运行的 Agent 时，应先停止 `hubterm-agent` 服务，再替换可执行文件并重新启动服务。旧版 Agent 不会上报 Shell 能力，因此节点页面不会显示“本机终端”入口。

## 当前限制

当前实现使用进程标准输入/输出管道提供持续交互，适合 CMD、PowerShell 和 Git Bash 的常规命令操作。它尚未使用 Windows ConPTY，因此存在以下限制：

- resize 消息暂不改变子进程终端尺寸；
- `vim`、`top`、交互式 TUI 等全屏程序可能显示异常；
- 光标定位、颜色和控制序列兼容性不及 Tabby 的完整 built-in 终端。

后续接入 ConPTY 时可以保留现有 Shell 探测、会话 API 和 WebSocket 协议，只替换 Agent 内部的进程承载层。

## 相关 API

### 启动会话

```http
POST /api/nodes/:id/shell
Authorization: Bearer <token>
Content-Type: application/json

{"shell":"powershell","rows":24,"cols":100}
```

返回：

```json
{"session_id":"..."}
```

### 关闭会话

```http
DELETE /api/nodes/:id/shell/:session_id
Authorization: Bearer <token>
```

终端字节通过 `/api/ws` 的 `terminal_subscribe`、`terminal_input` 和 `terminal_data` 消息传输，内容使用 Base64 编码。
