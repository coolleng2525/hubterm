# HubTerm — 串口/SSH 集群管控平台

## Pipeline（自动执行，无需人工介入）

```
Phase 1: 代码生成 ✅
  └─ 模型: DeepSeek V4 Pro
  └─ 产出: Go 后端 1138 行 + Vue 前端，编译通过

Phase 2: Code Review ✅
  └─ 模型: 豆包 Doubao Seed
  └─ 发现 15+ 问题（6 个 HIGH，6 个 MEDIUM，3 个 LOW）

Phase 3+4: 修复 + 基础设施 ✅
  └─ 模型: DeepSeek V4 Pro
  └─ 修复 12 个问题（6 HIGH / 3 MEDIUM / 3 LOW），go build + vet 通过
  └─ 新增:
       ├─ internal/pkg/log/ — 结构化日志
       ├─ internal/pkg/health/ — 健康自检
       ├─ internal/pkg/repair/ — 自修复（backoff）
       ├─ 节点日志上报 /api/logs
       ├─ JWT refresh token 机制
       └─ 健康检查端点 GET /api/health

Phase 5: 写测试 ✅
  └─ 模型: DeepSeek V4 Pro
  └─ 11 个测试文件，36 个测试用例，全部通过
  └─ 模块级测试（改哪个跑哪个）

Phase 6: 构建验证 ✅
  └─ go build ./... ✅
  └─ go vet ./... ✅
  └─ go test ./... -count=1 ✅（全部通过）
  └─ npm run build ✅（前端构建通过）

### 模块测试映射表

| 模块路径 | 测试文件 | 测试范围 | 触发条件 |
|----------|---------|---------|---------|
| `internal/center/model/` | `model_test.go` | 数据模型 CRUD、数据库初始化 | 改 model |
| `internal/center/service/` | `service_test.go` | 密码哈希、JWT 生成/验证、用户认证逻辑 | 改 service |
| `internal/center/middleware/` | `middleware_test.go` | JWT 中间件鉴权、角色权限校验 | 改 middleware |
| `internal/center/handler/auth.go` | `handler/auth_test.go` | 登录/注册 API、token 返回 | 改 auth handler |
| `internal/center/handler/node.go` | `handler/node_test.go` | 节点列表/详情/上报/指令下发 | 改 node handler |
| `internal/center/handler/session.go` | `handler/session_test.go` | 会话列表/踢人/指派主控 | 改 session handler |
| `internal/center/handler/serial_port.go` | `handler/serial_port_test.go` | 串口列表查询 | 改 serial handler |
| `internal/center/handler/audit_log.go` | `handler/audit_log_test.go` | 审计日志查询/分页 | 改 audit handler |
| `internal/center/handler/ws.go` | `handler/ws_test.go` | WebSocket 连接/消息推送 | 改 ws handler |
| `internal/agent/collector/` | `collector_test.go` | 串口扫描、CPU/内存采集 | 改 collector |
| `internal/agent/reporter/` | `reporter_test.go` | 上报逻辑、重试、backoff | 改 reporter |
| `internal/pkg/log/` | `log_test.go` | 日志写入、级别过滤、JSON 格式 | 改 log |
| `internal/pkg/health/` | `health_test.go` | 健康检查注册/执行/结果收集 | 改 health |
| `internal/pkg/repair/` | `repair_test.go` | 重启/重连/backoff 策略 | 改 repair |
| `internal/proto/` | `proto_test.go` | 序列化/反序列化 | 改 proto |

### 测试运行命令

```bash
# 改某个模块 → 只跑该模块测试
go test ./internal/center/handler/ -v -run TestAuth

# 改 service → 跑 service 测试
go test ./internal/center/service/ -v

# 改 agent collector → 跑 collector 测试
go test ./internal/agent/collector/ -v

# 全量测试（提交前）
go test ./... -v -race -count=1
```

### 测试规范
- 每个测试函数用 `TestXxx` 命名，清晰对应被测函数
- 使用 `t.Run()` 子测试分组
- handler 测试用 `httptest.NewRecorder()` + `gin.NewContext()`
- service 测试用 mock DB 或 SQLite in-memory
- agent 测试用 mock HTTP server (`httptest.NewServer()`)
- 不依赖外部环境（不连真实串口、不连真实中心）

Phase 6: 构建验证 ────────────────────────────────────── ⏳
  └─ go test ./... -v -race -count=1（全绿）
  └─ go build ./...（编译通过）
  └─ cd web && npm run build（前端构建通过）

Phase 7: 汇报 ────────────────────────────────────────── ⏳
  └─ 项目结构 + 启动方式 + 日志查看方式 + 测试运行方式
```

## 工业级标准（本项目目标）

### 网络层（核心能力）
- [ ] 自发现：节点上线自动广播，中心发现拓扑变化
- [ ] 自组网：节点间自动探测可达路径（SSH/串口/跳板）
- [ ] 多跳路由：A→B→C→D 自动规划最优路径
- [ ] 自愈：节点/链路故障自动切换备用路径
- [ ] 拓扑可视化：中心展示完整网络拓扑图
- [ ] 路径探测：每节点定期探测到其他节点的可达性
- [ ] 链路质量：记录延迟、成功率，路由决策参考

### 设备管理
- [ ] 设备类型：AP / 服务器 / Station / 网络设备 / 工控机
- [ ] 设备能力标注：SSH可达 / 仅串口 / 可做跳板 / 有GPU
- [ ] 设备标签：按项目/机房/内网段分组
- [ ] 批量命令：选一组设备同时下发命令

### AI 执行环境
- [ ] 设备发现 API：AI 查询所有可用设备及能力
- [ ] 命令执行 API：AI 指定目标设备 + 命令，平台自动路由
- [ ] 执行历史：AI 可查询历史命令和结果
- [ ] 安全沙箱：AI 执行命令受角色/设备权限限制

### 可靠性
- [ ] Graceful Shutdown：SIGTERM/SIGINT 时优雅关闭所有连接、完成正在处理的请求
- [ ] 数据库连接池：限制最大连接数、超时、重试
- [ ] 请求超时控制：每个 API 有超时限制，防止 goroutine 泄漏
- [ ] 熔断机制：节点上报连续失败时降级，不阻塞主流程
- [ ] 数据一致性：关键操作（节点上报、踢人）用事务包裹

### 安全性
- [ ] JWT 密钥从环境变量读取，不硬编码
- [ ] 密码用 bcrypt（已用）
- [ ] WebSocket 需要认证
- [ ] 节点上报用 node token 认证
- [ ] CORS 配置（生产环境限制来源）
- [ ] Rate Limiting：登录接口限制频率
- [ ] 敏感操作审计日志（已实现）

### 可观测性
- [ ] 结构化日志（JSON 格式，含 module/level/request_id/error）
- [ ] 健康检查端点 GET /health（返回各组件状态）
- [ ] 指标暴露（请求数、错误率、延迟）
- [ ] 日志分级（debug/info/warn/error/fatal）
- [ ] 错误追踪（每个 error 包含精确位置）

### 可配置性
- [ ] 配置文件（YAML/TOML）或环境变量
- [ ] 数据库路径可配置
- [ ] 监听地址和端口可配置
- [ ] 日志级别可配置
- [ ] JWT 密钥和过期时间可配置

### 部署
- [ ] Dockerfile（多阶段构建）
- [ ] docker-compose.yml（中心 + 数据库）
- [ ] 健康检查端点供容器编排用
- [ ] 信号处理（SIGTERM 优雅退出）

## 文档规范（所有代码必须配套）

### 1. Function Spec（写在代码注释里，每个导出函数必须有）

```go
// Login 处理用户登录
//
// 参数:
//   - c: gin.Context，需包含 JSON body: {username, password}
//
// 返回:
//   200: {token, user: {id, username, role}}
//   400: 缺少 username 或 password
//   401: 用户名或密码错误
//
// 错误定位:
//   - [auth.go:45] JSON 解析失败 → 400
//   - [auth.go:52] 用户不存在 → 401
//   - [auth.go:58] 密码不匹配 → 401
//   - [auth.go:65] JWT 生成失败 → 500
//
// 日志:
//   - INFO "user_login" username=xxx ip=xxx
//   - WARN "login_failed" username=xxx reason=xxx
```

### 2. 模块文档（`docs/` 目录下，每个模块一个）

```
docs/
├── ARCH.md           # 整体架构
├── api.md            # API 接口文档（含请求/响应示例）
├── center/           # 中心服务模块
│   ├── handler.md    # handler 层：路由注册、参数校验、响应
│   ├── service.md    # service 层：业务逻辑、认证、审计
│   ├── model.md      # model 层：数据模型、数据库操作
│   └── middleware.md  # 中间件：JWT 鉴权、角色校验
├── agent/            # 节点代理模块
│   ├── collector.md  # 状态采集：串口扫描、资源监控
│   └── reporter.md   # 上报逻辑：定时上报、重试、backoff
├── proto/            # 通信协议
│   └── protocol.md   # 节点上报格式、指令格式、WS 消息格式
└── web/              # 前端
    └── frontend.md   # 页面结构、组件树、API 调用
```

### 3. 错误定位规范

所有错误必须能追溯到精确位置：

```go
// 正确 ❌ 不要这种
if err != nil {
    return nil, fmt.Errorf("failed to process")
}

// 正确 ✅ 要这种
if err != nil {
    return nil, fmt.Errorf("[node.go:128] parse node report: %w", err)
}
```

日志必须包含：
- 模块名 + 文件名 + 行号
- 关键上下文（node_id, session_id, username）
- 错误原因

## 技术栈
- 后端: Go (gin/echo) + SQLite (gorm)
- 前端: Vue 3 + Vite + xterm.js
- 日志: zerolog（结构化 JSON 日志）
- 认证: JWT (bcrypt)
- 节点通信: HTTP REST + WebSocket
