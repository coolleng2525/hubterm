# HubTerm — 串口/SSH 集群管控平台

## 定位
AI 的执行环境基础设施层 — 让 AI 发现设备、了解能力、自动路由、执行命令。

## 当前进度（2026-06-14 更新）

### ✅ 已完成（全量 P0-P6）

| 模块 | 说明 | 版本 |
|------|------|------|
| Go 中心服务 | Gin + GORM + SQLite，12 个 API 端点 | v1.3.0 |
| Go 节点代理 | 串口扫描 + 系统采集 + 远程命令执行 + WebSocket 长连接 | v1.3.0 |
| Vue 3 前端 | 9 个页面，xterm.js Web 终端 | v1.0.0 |
| 结构化日志 | JSON 格式，含 module/level/node_id/request_id | v1.0.0 |
| 健康自检 | 组件注册 + 统一执行 | v1.0.0 |
| 自修复 | 失败重试 + 指数 backoff | v1.0.0 |
| JWT 认证 | bcrypt + WebSocket 认证 + 节点 token + 权限控制 | v1.0.0 |
| 模块测试 | 11 个测试文件，36 个用例 | v1.0.0 |
| GitHub Release | goreleaser，跨平台二进制自动构建 | v1.0.0-v1.3.0 |
| Docker 部署 | Dockerfile（多阶段构建）+ docker-compose.yml | v1.2.0 |
| YAML 配置文件 | 环境变量 > 配置文件 > 默认值 | v1.2.0 |
| 启动脚本 | `scripts/start-center.sh` | v1.2.0 |
| WindTerm 改造 | hubterm-v1 分支，自发现/自上报/被管理/数据透传 | v1.0.0 |
| V2 核心模块移植 | SSH 客户端/隧道/会话管理/录制/WebSocket 终端协议（从 Next Terminal） | v1.1.0 |
| 轻量节点代理 | 远程命令执行 + 系统服务安装（systemd/launchd/Windows Service） | v1.3.0 |
| 中英文 README | 双语文档 + 参考项目列表 | v1.1.0 |
| 调研报告 | Next Terminal / Wave / Warp / WindTerm / Tabby / ser2net / Headscale | v1.1.0 |
| **P1 ser2net 兼容** | Agent 自动检测 ser2net 安装和运行状态 | v1.3.0 |
| **P1 Graceful Shutdown** | center + agent 均支持 SIGTERM/SIGINT 优雅退出 | v1.3.0 |
| **P1 Tabby 内置插件** | 已集成到 coolleng2525/tabby fork，push 到 GitHub | v1.0.0 |
| **P2 脚本引擎** | Python 脚本引擎 + 脚本中心（上传/执行/结果查询） | v1.3.0 |
| **P3 AI 执行环境** | 设备发现 API + 命令执行 API + 执行历史 | v1.3.0 |
| **P4 网络层** | 拓扑发现/多跳路由(BFS)/自愈检测/拓扑可视化(D3.js) | v1.3.0 |
| **P5 分布式与隧道** | 虚拟设备名/会话代理/跨中心转发 | v1.3.0 |
| **P6 设备管理** | 设备CRUD+能力标注+标签/批量命令/设备分组 | v1.3.0 |

### 已放弃/变更方案

| 原计划 | 新方案 | 原因 |
|--------|--------|------|
| Tabby 插件发布到 npm | 直接集成到 Tabby 源码作为内置插件 | 外部插件依赖版本冲突，Node 22 不兼容 |
| 自研 Web 终端 | 考虑接管 Tabby Web（Angular + xterm.js） | 现成方案，原作者已放弃，直接拿来改 |

## 技术栈
- 后端: Go (Gin) + SQLite (GORM)
- 前端: Vue 3 + Vite + xterm.js
- 日志: 结构化 JSON（自研 log 包）
- 认证: JWT (bcrypt)
- 节点通信: HTTP REST + WebSocket
- 终端: Tabby（内置插件）+ WindTerm（hubterm-v1 分支）

## 仓库
| 项目 | 地址 | 状态 |
|------|------|------|
| HubTerm | https://github.com/coolleng2525/hubterm | ✅ 活跃 |
| WindTerm | https://github.com/coolleng2525/WindTerm/tree/hubterm-v1 | ✅ 已改造 |
| Tabby | https://github.com/coolleng2525/tabby | ✅ 已集成 |
| Tabby Web | https://github.com/coolleng2525/tabby-web | 📝 待评估 |
| NAS | `/mnt/nas/output/git-repos/self/hubterm/` | ✅ 已同步 |
