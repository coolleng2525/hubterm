# Wave Terminal & Warp 源码调研

> 调研日期: 2026-06-13
> 位置: `/mnt/nas/output/git-repos/third-party/`

---

## Wave Terminal

| 指标 | 值 |
|------|-----|
| 语言 | Go + React |
| 代码量 | 344 个 Go 文件，85,614 行 |
| 协议 | MIT |
| Star | 10k+ |

### 核心架构

```
waveterm/
├── pkg/
│   ├── wshrpc/        ← RPC 通信层（客户端/服务端/远程）
│   │   ├── wshclient/   ← 客户端
│   │   ├── wshserver/   ← 服务端
│   │   └── wshremote/   ← 远程执行
│   ├── jobmanager/    ← 任务管理（终端会话管理）
│   ├── wconfig/       ← 配置管理
│   └── baseds/        ← 数据存储
├── cmd/
│   ├── server/        ← 服务端入口
│   └── wsh/           ← 客户端 CLI
└── db/                ← 数据库
```

### 值得参考的设计
1. **wshrpc** — 统一的 RPC 通信层，客户端和服务端通过它通信
2. **jobmanager** — 终端会话管理，支持持久 SSH 会话（断线重连）
3. **wshremote** — 远程命令执行，适合参考做 AI 执行接口
4. **wconfig** — 配置管理，支持配置文件 + 环境变量覆盖

### 与 HubTerm 的关联
- 技术栈完全一致（Go + React），代码可直接参考
- 持久 SSH 会话设计 → 适合跳板机场景
- RPC 通信层 → 参考设计 AI ↔ 设备的通信协议

---

## Warp

| 指标 | 值 |
|------|-----|
| 语言 | Rust |
| 代码量 | 3,459 个 Rust 文件，106,794 行 |
| 协议 | AGPL-3.0 + MIT |
| Star | 30k+ |
| 开源时间 | 2026 年 4 月 |

### 核心架构

```
warp/
├── app/               ← 主应用（Rust）
├── crates/            ← 核心库
│   ├── vim/           ← Vim 模式
│   ├── terminal/      ← 终端渲染
│   └── ...
├── Cargo.toml         ← Rust 依赖
└── .mcp.json          ← MCP 协议配置
```

### 值得参考的设计
1. **Agent Mode** — 内置 AI 编码代理，自然语言→命令
2. **MCP 协议** — Model Context Protocol，AI 与工具的标准化通信
3. **GPU 加速渲染** — 性能架构参考

### 与 HubTerm 的关联
- Agent Mode 的设计思路 → 参考做 AI 执行接口
- MCP 协议 → 参考做 AI ↔ 设备通信标准
- Rust 架构 → 长期可考虑用 Rust 重写性能敏感组件

---

## 总结

| 项目 | 可参考什么 | 优先级 |
|------|----------|--------|
| Wave Terminal | wshrpc RPC 层、jobmanager 会话管理、wconfig 配置 | P1 |
| Warp | Agent Mode、MCP 协议架构 | P2 |
