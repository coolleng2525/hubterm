# HubTerm — 串口/SSH 集群管控平台

## 定位
AI 的执行环境基础设施层 — 让 AI 发现设备、了解能力、自动路由、执行命令。

## 当前进度（2026-06-20 更新）

### ✅ 已完成（全量 P0-P6）

| 模块 | 说明 | 版本 |
|------|------|------|
| Go 中心服务 | Gin + GORM + SQLite，15+ API 端点 | v1.4.0 |
| Go 节点代理 | 串口扫描 + 系统采集 + 远程命令执行 + WebSocket 长连接 | v1.4.0 |
| Vue 3 前端 | 10 个页面（含 SharedTerminal），xterm.js Web 终端 | v1.1.0 |
| 结构化日志 | JSON 格式，含 module/level/node_id/request_id | v1.0.0 |
| 健康自检 | 组件注册 + 统一执行 | v1.0.0 |
| 自修复 | 失败重试 + 指数 backoff | v1.0.0 |
| JWT 认证 | bcrypt + WebSocket 认证 + 节点 token + 权限控制（admin/operator/readonly） | v1.1.0 |
| 模块测试 | 15+ 测试文件，覆盖认证/节点/会话/终端/协议 | v1.1.0 |
| GitHub Release | goreleaser，跨平台二进制自动构建 | v1.0.0-v1.4.0 |
| Docker 部署 | Dockerfile（Alpine + CGO_ENABLED=0）+ docker-compose.yml | v1.3.0 |
| YAML 配置文件 | 环境变量 > 配置文件 > 默认值 | v1.2.0 |
| 启动脚本 | `scripts/start-center.sh` | v1.2.0 |
| WindTerm 改造 | hubterm-v1 分支，`src/HubTerm/` 模块（Agent/Reporter/Commander/TerminalShare） | v1.0.0 |
| V2 核心模块移植 | SSH 客户端/隧道/会话管理/录制/WebSocket 终端协议（从 Next Terminal） | v1.1.0 |
| 轻量节点代理 | 远程命令执行 + 系统服务安装（systemd/launchd/Windows Service） | v1.3.0 |
| 中英文 README | 双语文档 + 参考项目列表 | v1.1.0 |
| 调研报告 | Next Terminal / Wave / Warp / WindTerm / Tabby / ser2net / Headscale | v1.1.0 |
| **P1 ser2net 兼容** | Agent 自动检测 ser2net 安装和运行状态 | v1.3.0 |
| **P1 Graceful Shutdown** | center + agent 均支持 SIGTERM/SIGINT 优雅退出 | v1.3.0 |
| **P1 Tabby 内置插件** | 已集成到 coolleng2525/tabby fork，构建通过，协议已对齐 | v1.1.0 |
| **P1 节点安全** | 节点认证防劫持、token 持久化、串口清理 | v1.4.0 |
| **P1 共享终端** | Tabby ↔ Center ↔ 浏览器 终端流闭环（report/input/output/kick/assign_master） | v1.4.0 |
| **P2 脚本引擎** | Python 脚本引擎 + 脚本中心（上传/执行/结果查询） | v1.3.0 |
| **P3 AI 执行环境** | 设备发现 API + 命令执行 API + 执行历史 | v1.3.0 |
| **P4 网络层** | 拓扑发现/多跳路由(BFS)/自愈检测/拓扑可视化(D3.js) | v1.3.0 |
| **P5 分布式与隧道** | 虚拟设备名/会话代理/跨中心转发 | v1.3.0 |
| **P6 设备管理** | 设备CRUD+能力标注+标签/批量命令/设备分组 | v1.3.0 |
| **节点客户端来源识别** | 识别节点客户端来源（Tabby/WindTerm/Agent/Web） | v1.4.0 |
| **Windows 本机终端** | 节点详情页启动 CMD/PowerShell/PowerShell 7/Git Bash | v1.4.0 |
| **纯 Go SQLite** | 切换为 modernc.org/sqlite，支持 CGO_ENABLED=0 构建 | v1.3.0 |
| **Admin 密码同步** | 启动时从 ADMIN_PASSWORD 同步现有 admin 密码 | v1.3.0 |

### 🔧 进行中

| 模块 | 说明 | 状态 |
|------|------|------|
| Tabby 插件联调 | 协议已对齐，构建通过，待真实环境联调 | 🟡 待联调 |
| WindTerm 构建 | 缺少完整 Qt 工程和闭源部分，无法从当前仓库构建 | 🔴 阻塞 |
| Tabby Web | fork 待评估 | 📝 待评估 |
| frp 隧道接入 | 需冷哥提供服务器信息 | ⏳ 等待 |

### 待办

| 任务 | 优先级 |
|------|--------|
| Tabby 插件真实环境联调（启动 Center + Tabby，验证终端流） | 🔴 高 |
| WindTerm 完整源码获取或独立 CMake 工程 | 🟡 中 |
| 前端构建产物（stash 中的 codex-generated UI）apply 并验证 | 🟡 中 |
| README-en.md 同步更新 | 🟢 低 |
| frp 隧道接入 | 🟢 低 |

### 已放弃/变更方案

| 原计划 | 新方案 | 原因 |
|--------|--------|------|
| Tabby 插件发布到 npm | 直接集成到 Tabby 源码作为内置插件 | 外部插件依赖版本冲突，Node 22 不兼容 |
| 自研 Web 终端 | 考虑接管 Tabby Web（Angular + xterm.js） | 现成方案，原作者已放弃，直接拿来改 |
| WindTerm 完整集成 | `src/HubTerm/` 定位为独立原型库 | 当前 fork 缺少完整 Qt 工程和闭源部分，无法构建完整应用 |
| CGO SQLite (go-sqlite3) | 纯 Go SQLite (modernc.org/sqlite) | 支持 CGO_ENABLED=0 构建，Docker/GoReleaser 不再需要 C 工具链 |

## 技术栈
- 后端: Go (Gin) + SQLite (modernc.org/sqlite, 纯 Go)
- 前端: Vue 3 + Vite + Element Plus + xterm.js
- 日志: 结构化 JSON（自研 log 包）
- 认证: JWT (bcrypt) + WebSocket 子协议鉴权
- 节点通信: HTTP REST + WebSocket
- 终端: Tabby（内置插件）+ WindTerm（hubterm-v1 分支，原型阶段）
- 构建: CGO_ENABLED=0（纯 Go，无需 C 工具链）

## 仓库
| 项目 | 地址 | 状态 |
|------|------|------|
| HubTerm | https://github.com/coolleng2525/hubterm | ✅ 活跃 |
| WindTerm | https://github.com/coolleng2525/WindTerm/tree/hubterm-v1 | ✅ 已改造 |
| Tabby | https://github.com/coolleng2525/tabby | ✅ 已集成 |
| Tabby Web | https://github.com/coolleng2525/tabby-web | 📝 待评估 |
| NAS | `/mnt/nas/output/git-repos/self/hubterm/` | ✅ 已同步 |
