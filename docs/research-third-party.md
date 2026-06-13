# HubTerm 第三方项目调研报告

> 调研日期: 2026-06-13
> 用途: 作为 HubTerm V2 架构参考，避免重复造轮子

---

## 一、堡垒机/跳板机类

### 1. Next Terminal

| 项目 | 值 |
|------|-----|
| 地址 | https://github.com/dushixiang/next-terminal |
| 语言 | Go + React |
| 协议 | AGPL-3.0（v2 起后端闭源） |
| 本地位置 | `/mnt/nas/output/git-repos/third-party/next-terminal/` |
| 参考版本 | v1.3.9（最后一个开源版本） |
| Star | 4k+ |

**核心能力：**
- 资产管理（SSH/RDP/VNC/Telnet/HTTP）
- 接入网关（SSH 隧道代理，跳板机访问内网设备）
- 会话管理 + 观察者模式（多人共享终端）
- WebSocket 终端协议（数据/缩放/心跳）
- 会话录制 + 回放（asciicast v2 格式）
- 内置 SSH 服务端
- 凭证管理（AES-256 加密存储）

**已移植到 HubTerm 的模块：**

| 模块 | 文件 | 行数 |
|------|------|------|
| SSH 客户端 | `internal/pkg/sshclient/ssh.go` | 153 |
| SSH 隧道/跳板机 | `internal/pkg/tunnel/tunnel.go` | 258 |
| 会话管理器 | `internal/pkg/session/manager.go` | 154 |
| 会话录制器 | `internal/pkg/recorder/recorder.go` | 135 |
| WebSocket 终端 | `internal/center/handler/terminal.go` | 449 |

**可进一步参考：**
- 凭证加密存储（AES-256）
- 资产标签分类
- 登录策略（IP 限制、时间限制）
- 命令过滤/拦截

---

## 二、AI 原生终端类

### 2. Wave Terminal

| 项目 | 值 |
|------|-----|
| 地址 | https://github.com/wavetermdev/waveterm |
| 语言 | Go + React（前端） |
| 协议 | MIT |
| 本地位置 | `/mnt/nas/output/git-repos/third-party/waveterm/` |
| Star | 10k+ |

**核心能力：**
- AI 集成（OpenAI/Claude/Gemini/Ollama/LM Studio）
- 持久 SSH 会话（断网自动重连）
- 内联渲染（图片/文件预览/Web 浏览）
- 工作区管理
- 跨平台（macOS/Linux/Windows）

**与 HubTerm 的关联：**
- 技术栈相同（Go + React），架构可参考
- 持久 SSH 会话设计 → 适合我们的跳板机场景
- AI 模型配置方式 → 参考其 API Key 管理模式

**值得深入阅读的目录：**
```
waveterm/
├── db/           ← 数据库层
├── pkg/          ← 核心逻辑
├── app/          ← 前端
└── build/        ← 构建脚本
```

---

### 3. Warp

| 项目 | 值 |
|------|-----|
| 地址 | https://github.com/warpdotdev/Warp |
| 语言 | Rust |
| 协议 | AGPL-3.0 + MIT |
| 本地位置 | `/mnt/nas/output/git-repos/third-party/warp/` |
| Star | 30k+ |
| 开源时间 | 2026 年 4 月 |

**核心能力：**
- GPU 加速终端渲染
- Agent Mode（内置 AI 编码代理）
- MCP 协议支持（Model Context Protocol）
- 云同步（Oz 平台）
- 协作功能

**与 HubTerm 的关联：**
- Agent Mode 的设计思路 → 参考 AI 执行接口
- MCP 协议 → 参考 AI ↔ 设备通信协议设计
- Rust 性能架构 → 长期可考虑用 Rust 重写性能敏感组件

**值得深入阅读的目录：**
```
warp/
├── app/          ← 主应用
├── Cargo.toml    ← Rust 依赖
└── .mcp.json     ← MCP 协议配置
```

---

## 三、终端客户端类

### 4. WindTerm

| 项目 | 值 |
|------|-----|
| 地址 | https://github.com/kingToolbox/WindTerm |
| 语言 | C |
| 协议 | Apache-2.0（部分开源） |
| 本地位置 | `/mnt/nas/output/git-repos/self/hubterm/WindTerm/` |
| 分支 | `hubterm-v1`（已集成 HubTerm Agent） |

**核心能力：**
- SSH/Serial/Telnet/SFTP/Raw TCP
- 跨平台（Windows/macOS/Linux）
- GPU 加速渲染
- 会话管理

**HubTerm 集成状态：**
- `src/HubTerm/` — Agent/Config/Reporter/Commander/TerminalShare
- `src/Pty/` — 数据透传 hook（dataReceived 信号）
- 能力：自发现、自上报、被管理、终端共享

---

### 5. Tabby

| 项目 | 值 |
|------|-----|
| 地址 | https://github.com/Eugeny/tabby |
| 语言 | TypeScript + Angular + Electron |
| 协议 | MIT |
| Star | 60k+ |

**核心能力：**
- 插件系统（npm 包，`tabby-plugin` 关键字）
- SSH/Serial/Telnet
- 分屏/标签/主题
- 跨平台

**HubTerm 集成状态：**
- `tabby-hubterm-plugin/` — 插件已开发
- 能力：WebSocket 连接、终端 I/O 透传、远程管理

---

## 四、串口服务器类

### 6. ser2net

| 项目 | 值 |
|------|-----|
| 地址 | https://sourceforge.net/projects/ser2net/ |
| 语言 | C |
| 协议 | GPL-2.0 |
| 安装 | `apt install ser2net` |

**核心能力：**
- 串口 → TCP 映射
- 支持 Telnet/RFC2217 协议
- 多端口配置
- 访问控制

**HubTerm 集成方案（规划中）：**
- Agent 可选模式：完整模式（Go Agent）/ 轻量模式（仅管理 ser2net）
- Agent 检测到 ser2net 在跑，直接接管管理

---

## 五、自组网/隧道类

### 7. Headscale

| 项目 | 值 |
|------|-----|
| 地址 | https://github.com/juanfont/headscale |
| 语言 | Go |
| 协议 | BSD-3-Clause |
| 定位 | Tailscale 控制服务器的开源实现 |

**核心能力：**
- WireGuard 网状 VPN
- 节点自动发现
- ACL 访问控制
- DERP 中继

**HubTerm 集成方案（规划中）：**
- 作为 HubTerm 的网络层
- 节点间自动建立 WireGuard 隧道
- 跨互联网加密通信

---

### 8. Tailscale

| 项目 | 值 |
|------|-----|
| 地址 | https://github.com/tailscale/tailscale |
| 语言 | Go |
| 协议 | BSD-3-Clause |
| 定位 | 商业产品，部分开源 |

**核心能力：**
- 零配置 VPN
- 自发现 + 自组网
- 基于 WireGuard
- 全球中继（DERP）

**参考价值：**
- 架构设计（控制面 + 数据面分离）
- 自发现协议
- NAT 穿透方案

---

## 六、调研总结

### 技术栈分布

| 语言 | 项目 | HubTerm 适用性 |
|------|------|---------------|
| Go | Next Terminal, Wave Terminal, Headscale, Tailscale | ✅ 可直接参考 |
| Rust | Warp | ⭐ 架构参考，长期可重写 |
| C | WindTerm, ser2net | ✅ 已集成 |
| TypeScript | Tabby | ✅ 插件已开发 |

### 可以拿什么

| 项目 | 拿来什么 | 优先级 |
|------|---------|--------|
| Next Terminal | 堡垒机架构、SSH 隧道、会话管理 | ✅ 已完成 |
| Wave Terminal | 持久 SSH 会话、AI 集成方式 | P2 |
| Warp | Agent Mode、MCP 协议 | P3 |
| Headscale | 自组网、WireGuard 隧道 | P4 |
| ser2net | 串口→TCP 映射 | P1 |
| WindTerm | 终端底层 | ✅ 已完成 |
| Tabby | 插件系统 | ✅ 已完成 |

### 不做的事

- 不重新发明终端（用 WindTerm/Tabby/Wave）
- 不重新发明串口服务器（用 ser2net）
- 不重新发明 VPN（用 Headscale/Tailscale）
- 不重新发明堡垒机（参考 Next Terminal）

### HubTerm 只做

- AI 执行接口（别人没有）
- 脚本引擎（别人没有）
- 集成胶水层（把上述项目串起来）
- 统一设备抽象（`hubterm://device-name`）
