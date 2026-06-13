# HubTerm — 串口/SSH 集群管控平台

## 定位
AI 的执行环境基础设施层 — 让 AI 发现设备、了解能力、自动路由、执行命令。

## 优先级路线图

### P0 — 基础可用（当前状态 ✅）
- [x] Go 中心服务（Gin + GORM + SQLite，12 个 API 端点）
- [x] Go 节点代理（串口扫描 + 系统采集，每 3 秒上报）
- [x] Vue 3 前端（9 个页面，xterm.js Web 终端）
- [x] 结构化日志 / 健康自检 / 自修复
- [x] JWT 认证 / bcrypt / WebSocket 认证 / 节点 token / 权限控制
- [x] 11 个模块测试，36 个用例
- [x] GitHub Release（goreleaser，跨平台二进制）
- [x] WindTerm 源码改造（hubterm-v1 分支，自发现/自上报/被管理/数据透传）
- [x] Tabby 插件（tabby-hubterm，终端 I/O 透传 + 远程管理）

### P1 — 部署与兼容（下一轮）
- [ ] **Docker 部署** — 中心服务容器化，`docker compose up` 一键启动
- [ ] **ser2net 兼容** — Agent 可选模式：完整模式 / 轻量模式（仅管理 ser2net 映射的串口）
- [ ] **Tabby 插件发布** — 编译后发 npm + GitHub Release
- [ ] **配置文件** — YAML 配置，替代环境变量硬编码
- [ ] **Graceful Shutdown** — SIGTERM/SIGINT 优雅退出

### P2 — 脚本引擎（核心差异化）
- [ ] **Python 脚本引擎** — HubTerm 内置 Python 运行时，你的现有脚本直接上传执行
- [ ] **脚本中心** — 上传/版本管理/分类/分发/定时执行
- [ ] **交互式脚本** — expect 风格：等待→发送→等待→发送，处理复杂认证流程
- [ ] **设备接入脚本** — 每类设备配一个接入脚本（AP/服务器/交换机），自动匹配执行

### P3 — AI 执行环境（核心差异化）
- [ ] **设备发现 API** — AI 查询所有可用设备及能力
- [ ] **命令执行 API** — AI 指定目标设备 + 命令，平台自动路由
- [ ] **执行历史** — AI 可查询历史命令和结果
- [ ] **安全沙箱** — AI 执行命令受角色/设备权限限制
- [ ] **终端权限粒度** — 只读/可写/命令过滤/sudo 控制

### P4 — 网络层（自发现 + 自组网 + 自愈）
- [ ] **自发现** — 节点上线自动广播，中心发现拓扑变化
- [ ] **自组网** — 节点间自动探测可达路径（SSH/串口/跳板）
- [ ] **多跳路由** — A→B→C→D 自动规划最优路径
- [ ] **自愈** — 节点/链路故障自动切换备用路径
- [ ] **拓扑可视化** — 中心展示完整网络拓扑图
- [ ] **路径探测** — 每节点定期探测到其他节点的可达性
- [ ] **链路质量** — 记录延迟、成功率，路由决策参考

### P5 — 分布式与隧道
- [ ] **分布式多中心** — 任意节点可管理全网，无单点故障（gossip 协议）
- [ ] **VPN/隧道** — 跨互联网加密通道，一个账号登录自动建立
- [ ] **虚拟设备名** — `hubterm://ap-03` 映射到真实路径，不管底层怎么连
- [ ] **会话代理** — 说"连 ap-03"，平台自动找路径建连接

### P6 — 设备管理增强
- [ ] **设备类型** — AP / 服务器 / Station / 网络设备 / 工控机
- [ ] **设备能力标注** — SSH可达 / 仅串口 / 可做跳板 / 有GPU
- [ ] **设备标签** — 按项目/机房/内网段分组
- [ ] **批量命令** — 选一组设备同时下发命令

## 技术栈
- 后端: Go (Gin) + SQLite (GORM)
- 前端: Vue 3 + Vite + xterm.js
- 日志: 结构化 JSON（自研 log 包）
- 认证: JWT (bcrypt)
- 节点通信: HTTP REST + WebSocket
- 终端插件: Tabby（npm）+ WindTerm（C++ Qt）

## 仓库
- HubTerm: https://github.com/coolleng2525/hubterm
- WindTerm: https://github.com/coolleng2525/WindTerm/tree/hubterm-v1
- Tabby 插件: `hubterm/tabby-hubterm-plugin/`
- NAS: `/mnt/nas/output/git-repos/self/hubterm/`
