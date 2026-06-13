# HubTerm - 串口/SSH 集群管控平台

基于 WindTerm 组件的串口/SSH 集群管控平台，采用**轻量集群方案**（节点上报+中心管理，不走流量中转）。

## 架构

```
中心服务 (Center) ← HTTP REST + WebSocket → 节点代理 (Agent) × N
    ↑                                      ↑
前端 (Vue 3 + xterm.js)              WindTerm (已集成 HubTerm Agent)
    ↑                                      ↑
 用户/AI                             用户/AI
```

- **中心服务**：数据汇总、统一管理后台、全局审计
- **节点代理**：运行在各机器上，采集系统状态、扫描串口，每 3 秒上报到中心
- **WindTerm**：已集成 HubTerm Agent，启动时自动发现中心、上报能力、透传终端数据
- **前端**：Vue 3 + xterm.js Web 终端

## 快速开始

### 启动中心服务

```bash
cd cmd/center
export JWT_SECRET=your-secret-key
go run main.go
```

中心服务监听 `:8080`。首次启动自动创建 admin 用户，密码从环境变量 `ADMIN_PASSWORD` 读取，未设置则生成随机密码打印到日志。

### 启动节点代理

```bash
cd cmd/agent
go run main.go --center http://localhost:8080
```

节点代理每 3 秒向中心上报系统状态和串口信息。

### 启动前端

```bash
cd web
npm install
npm run dev
```

前端开发服务器监听 `:3000`，自动代理 `/api` 到 `:8080`。

### 使用 WindTerm（已集成 HubTerm）

WindTerm 已集成 HubTerm Agent，无需额外配置：

1. 启动 WindTerm，自动连接 HubTerm 中心
2. 自动上报本机串口列表、SSH 会话、系统信息
3. 在 HubTerm Web 后台可以看到 WindTerm 节点在线
4. 终端 I/O 实时透传，Web 页面和 AI 可实时查看
5. 支持只读/可写权限控制

## 构建与发布

打 tag 自动触发 GitHub Release：

```bash
git tag v1.0.0
git push origin v1.0.0
```

自动编译 linux/windows/macOS × amd64/arm64 二进制并发布。

## 项目结构

```
hubterm/
├── cmd/
│   ├── center/       # 中心服务入口
│   └── agent/        # 节点代理入口
├── internal/
│   ├── center/       # 中心业务逻辑
│   │   ├── handler/  # HTTP handlers
│   │   ├── model/    # 数据模型
│   │   ├── service/  # 业务逻辑
│   │   └── middleware/ # JWT 中间件
│   ├── agent/        # 节点代理逻辑
│   │   ├── collector/ # 状态采集
│   │   └── reporter/  # 上报逻辑
│   └── proto/        # 共享协议定义
├── web/              # Vue 3 前端
│   └── src/
│       ├── views/    # 页面
│       ├── api/      # API 调用
│       └── components/ # 组件
└── README.md
```

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/auth/login | 登录 |
| POST | /api/auth/register | 注册（仅 admin） |
| GET | /api/auth/profile | 个人信息 |
| GET | /api/nodes | 节点列表 |
| GET | /api/nodes/:id | 节点详情 |
| POST | /api/nodes/report | 节点上报 |
| POST | /api/nodes/:id/command | 下发指令 |
| GET | /api/serial-ports | 串口列表 |
| GET | /api/sessions | 会话列表 |
| POST | /api/sessions/:id/kick | 踢掉会话 |
| POST | /api/sessions/:id/assign-master | 指派主控 |
| GET | /api/audit-logs | 审计日志 |
| WS | /api/ws | WebSocket 实时推送 |

## 技术栈

- **后端**: Go + Gin + GORM + SQLite
- **前端**: Vue 3 + Vite + Element Plus + xterm.js
- **认证**: JWT
- **节点通信**: HTTP REST + WebSocket
