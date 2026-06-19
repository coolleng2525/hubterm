# HubTerm — 串口/SSH 集群管控平台

## 定位
AI 的执行环境基础设施层 — 让 AI 发现设备、了解能力、自动路由、执行命令。

## 当前进度（2026-06-13 更新）

### ✅ 已完成

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

### 🔄 进行中

| 模块 | 当前状态 | 卡点 |
|------|---------|------|
| **Tabby 内置插件** | 子代理正在集成到 Tabby 源码中（9分钟+） | 外部插件方式失败（依赖地狱），改为直接改 Tabby 源码 |
| **Tabby Web 接管** | 已拉取到本地，原作者已放弃维护 | 待评估是否作为 HubTerm Web 前端替代 Vue |

### ❌ 已放弃/变更方案

| 原计划 | 新方案 | 原因 |
|--------|--------|------|
| Tabby 插件发布到 npm | 直接集成到 Tabby 源码作为内置插件 | 外部插件依赖版本冲突，Node 22 不兼容 |
| 自研 Web 终端 | 考虑接管 Tabby Web（Angular + xterm.js） | 现成方案，原作者已放弃，直接拿来改 |

### 📋 待办（按优先级）

#### P1 — 部署与兼容
- [ ] **ser2net 兼容** — Agent 可选模式：完整模式 / 轻量模式（仅管理 ser2net）
- [ ] **Graceful Shutdown** — SIGTERM/SIGINT 优雅退出
- [ ] **Tabby 内置插件完成** — 构建通过后 push 到 coolleng2525/tabby

#### P2 — 脚本引擎（核心差异化）
- [ ] **Python 脚本引擎** — HubTerm 内置 Python 运行时，现有脚本直接上传执行
- [ ] **脚本中心** — 上传/版本管理/分类/分发/定时执行
- [ ] **交互式脚本** — expect 风格：等待→发送→等待→发送
- [ ] **设备接入脚本** — 每类设备配一个接入脚本，自动匹配执行

#### P3 — AI 执行环境（核心差异化）
- [ ] **设备发现 API** — AI 查询所有可用设备及能力
- [ ] **命令执行 API** — AI 指定目标设备 + 命令，平台自动路由
- [ ] **执行历史** — AI 可查询历史命令和结果
- [ ] **安全沙箱** — AI 执行命令受角色/设备权限限制
- [ ] **终端权限粒度** — 只读/可写/命令过滤/sudo 控制

#### P4 — 网络层（自发现 + 自组网 + 自愈）
- [ ] **自发现** — 节点上线自动广播，中心发现拓扑变化
- [ ] **自组网** — 节点间自动探测可达路径（SSH/串口/跳板）
- [ ] **多跳路由** — A→B→C→D 自动规划最优路径
- [ ] **自愈** — 节点/链路故障自动切换备用路径
- [ ] **拓扑可视化** — 中心展示完整网络拓扑图

#### P5 — 分布式与隧道
- [ ] **分布式多中心** — 任意节点可管理全网，无单点故障（gossip 协议）
- [ ] **VPN/隧道** — 跨互联网加密通道，一个账号登录自动建立
- [ ] **虚拟设备名** — `hubterm://ap-03` 映射到真实路径
- [ ] **会话代理** — 说"连 ap-03"，平台自动找路径建连接

#### P6 — 设备管理增强
- [ ] **设备类型** — AP / 服务器 / Station / 网络设备 / 工控机
- [ ] **设备能力标注** — SSH可达 / 仅串口 / 可做跳板 / 有GPU
- [ ] **设备标签** — 按项目/机房/内网段分组
- [ ] **批量命令** — 选一组设备同时下发命令

## 技术栈
- 后端: Go (Gin) + SQLite (GORM)
- 前端: Vue 3 + Vite + xterm.js（考虑迁移到 Tabby Web）
- 日志: 结构化 JSON（自研 log 包）
- 认证: JWT (bcrypt)
- 节点通信: HTTP REST + WebSocket
- 终端: Tabby（内置插件）+ WindTerm（hubterm-v1 分支）

## 仓库
| 项目 | 地址 | 状态 |
|------|------|------|
| HubTerm | https://github.com/coolleng2525/hubterm | ✅ 活跃 |
| WindTerm | https://github.com/coolleng2525/WindTerm/tree/hubterm-v1 | ✅ 已改造 |
| Tabby | https://github.com/coolleng2525/tabby | 🔄 集成中 |
| Tabby Web | https://github.com/coolleng2525/tabby-web | 📝 待接管 |
| NAS | `/mnt/nas/output/git-repos/self/hubterm/` | ✅ 已同步 |
