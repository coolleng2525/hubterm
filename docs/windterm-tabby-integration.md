# WindTerm & Tabby HubTerm 集成 — 工作交接文档

> 写给 Codex：这是小祝之前做的工作，你来接着干。
> 最后更新：2026-06-19

---

## 一、整体架构

```
HubTerm Center (Go/Gin)
    ↑ WebSocket
    ├── WindTerm (C++/Qt) ← hubterm-v1 分支
    ├── Tabby (Electron/TS) ← 内置插件 tabby-hubterm
    └── Tabby Web (Angular) ← 待评估
```

目标：让 WindTerm 和 Tabby 成为 HubTerm 的受管节点，实现自发现、自上报、被管理、终端共享。

---

## 二、WindTerm 集成（已完成 ✅）

**仓库：** https://github.com/coolleng2525/WindTerm/tree/hubterm-v1
**本地：** `/mnt/nas/output/git-repos/self/hubterm/WindTerm/`
**提交：** `dabd5fc` feat: HubTerm Agent 集成

### 做了什么

新增 `src/HubTerm/` 模块（C++/Qt）：

| 文件 | 职责 |
|------|------|
| `Agent.cpp/h` | WebSocket 连接中心、注册节点身份 |
| `Config.cpp/h` | 加载 `hubterm.json` 配置 |
| `Reporter.cpp/h` | 收集并上报串口、SSH 会话、系统信息（每3秒） |
| `Commander.cpp/h` | 接收并执行中心命令（connect/disconnect/exec script） |
| `TerminalShare.cpp/h` | Hook Pty 数据透传，实时流式传输终端 I/O |

修改了 Pty 基类：
- `Pty.h` — 加数据透传 hook 接口
- `Pty.cpp` — 实现 hook 调用
- `ConPty.cpp / UnixPty.cpp / WinPty.cpp` — 适配

### 配置

WindTerm 配置目录下放 `hubterm.json`：
```json
{
  "center_url": "ws://your-center:8080/ws",
  "node_name": "my-workstation",
  "domain": "mycompany.com"
}
```

### 构建

需要 Qt + WebSocket 模块：
```bash
mkdir build && cd build
cmake .. -DCMAKE_PREFIX_PATH=/path/to/qt
make
```

### 待办

- [ ] 实际编译测试（C++/Qt 环境搭建）
- [ ] 权限控制（只读/可写模式）
- [ ] 发布 CI（`928fe9f` 加了 release workflow 但没跑过）

---

## 三、Tabby 内置插件（已完成 ✅）

**仓库：** https://github.com/coolleng2525/tabby
**本地：** `/mnt/nas/output/git-repos/third-party/tabby-source/`
**提交：** `2ac0335e` feat: add tabby-hubterm builtin plugin

### 做了什么

在 Tabby 源码 `tabby-hubterm/` 目录下新增内置插件：

| 文件 | 职责 |
|------|------|
| `hubterm.service.ts` | WebSocket 连接、节点注册、命令执行、终端数据流 |
| `terminalDecorator.ts` | Hook Tabby 终端 I/O |
| `settingsTab.component.ts/pug` | 配置 UI（Center URL、Node Name、Token） |
| `configProvider.ts` | 配置读写 |
| `index.ts` | 插件入口 |

已在 `app/package.json` 的 `builtinPlugins` 中注册（`669f5e27`）。

### 构建

```bash
cd tabby-source
yarn install
yarn build
```

### 待办

- [ ] 实际构建测试（Tabby 依赖复杂，之前外部插件方式因依赖地狱失败，改为内置插件）
- [ ] 测试 WebSocket 连接中心
- [ ] 测试终端数据流
- [ ] 测试远程命令执行

---

## 四、Tabby Web（待评估 📝）

**仓库：** https://github.com/coolleng2525/tabby-web
**本地：** `/mnt/nas/output/git-repos/third-party/tabby-web-fork/`

### 现状

- 原作者已放弃维护
- 已 fork 到 coolleng2525 名下
- 作为 HubTerm Web 前端的替代方案待评估
- 当前 HubTerm Web 前端是 Vue 3 + Vite + xterm.js

### 待办

- [ ] 评估是否适合替代现有 Vue 前端
- [ ] 如需使用，需要集成 HubTerm 连接逻辑

---

## 五、相关文件位置

| 内容 | 路径 |
|------|------|
| HubTerm 主项目 | `/code/hubterm/` |
| WindTerm fork | `/mnt/nas/output/git-repos/self/hubterm/WindTerm/` |
| Tabby fork | `/mnt/nas/output/git-repos/third-party/tabby-source/` |
| Tabby Web fork | `/mnt/nas/output/git-repos/third-party/tabby-web-fork/` |
| tabby-hubterm 插件源码 | `/code/hubterm/tabby-hubterm-plugin/` |
| ROADMAP | `/code/hubterm/ROADMAP.md` |
| 调研报告 | `/code/hubterm/docs/` (Next Terminal/Wave/Warp/WindTerm/Tabby/ser2net/Headscale) |

---

## 六、优先级建议

1. **Tabby 内置插件构建测试** — 验证 WebSocket 连接、终端数据流、远程命令
2. **WindTerm 编译测试** — 需要 C++/Qt 环境
3. **Tabby Web 评估** — 决定是否替代现有 Vue 前端
