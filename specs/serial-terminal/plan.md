# Plan: 多人共享串口终端

> Spec: [spec.md](./spec.md)

## Approach

浏览器先通过 Center 的连接接口获取或创建目标串口的唯一会话，再进入现有共享终端页面。Center 负责串口配置持久化、并发连接去重、参与者角色和服务端写权限；Agent 负责独占打开物理串口、原始字节读写和异常关闭。串口输出沿用 Agent WebSocket → Center → Browser WebSocket 的 Base64 数据通道，浏览器直接把字节写入终端，不做文本转码。

## Stack

- **New dependency:** `github.com/jacobsa/go-serial@v0.0.0-20180131005756-15cf729a72d4` — 纯 Go、支持 macOS/Linux/Windows、可在 `CGO_ENABLED=0` 下交叉编译，并提供 RTS/CTS 流控。
- **No new infrastructure:** 继续使用 SQLite、现有 HTTP/WebSocket 服务和 Agent 长连接。

## File-level breakdown

### New files

- `internal/agent/serialsession/manager.go` — 串口会话管理、独占打开、读写、关闭和状态上报。
- `internal/agent/serialsession/manager_test.go` — 使用可替换端口实现验证会话生命周期和字节透传。
- `internal/center/handler/serial_terminal.go` — 串口配置更新、连接创建/复用和关闭处理。
- `internal/center/handler/serial_terminal_test.go` — 验证参数校验、权限、并发去重和失败回滚。
- `internal/center/handler/terminal_participants.go` — 浏览器参与者、主控权、观察者和最后离开清理。
- `internal/center/handler/terminal_participants_test.go` — 验证加入、离开、主控转移、踢出和写权限。

### Changed files

- `go.mod`, `go.sum` — 锁定串口驱动依赖。
- `internal/proto/types.go` — 增加串口参数、`serial_start`、`serial_close` 和会话协议字段。
- `cmd/agent/main.go` — 初始化串口管理器并处理打开、写入、关闭及断线清理命令。
- `internal/agent/connector/connector.go` — 接收串口命令并在 Center 连接断开时触发本地清理。
- `internal/agent/reporter/reporter.go` — 将活动串口会话和占用状态加入节点上报。
- `internal/center/model/models.go` — 新增串口配置模型，并给会话增加协议类型。
- `internal/center/handler/node.go` — 同步串口发现状态时保留用户配置，并根据活动会话维护占用状态。
- `internal/center/handler/serial_port.go` — 返回合并后的发现信息和持久化参数。
- `internal/center/handler/agent_ws.go` — 下发串口命令、等待 Agent 确认、管理会话归属并清理失败状态。
- `internal/center/handler/ws.go` — 注册浏览器参与者，只允许当前主控写入，并广播角色变化。
- `internal/center/handler/session.go` — 串口会话结束时同步删除会话并释放端口。
- `cmd/center/main.go` — 注册串口配置、连接和关闭路由。
- `web/src/api/index.js` — 增加串口配置和连接请求。
- `web/src/views/NodeDetail.vue` — 增加参考图样式的操作列、参数编辑窗口和连接状态。
- `web/src/views/SharedTerminal.vue` — 显示串口参数、当前角色和参与者，支持转移主控及踢出，并禁用观察者输入。
- 相关现有测试文件 — 覆盖新增字段、兼容旧 Agent 和权限边界。

### Data model changes

- New table: `serial_port_configs`
  - `node_id`, `port_name` 组成唯一键。
  - 保存可选 `alias`、`baud_rate`, `data_bits`, `parity`, `stop_bits`, `flow_control` 和时间戳。
  - 默认值为 `115200 / 8 / none / 1 / none`。
- Modified table: `sessions`
  - 新增 `protocol`，用于区分 `serial`、`shell` 和 `ssh` 会话。
- 浏览器参与者不写数据库；它们与 WebSocket 生命周期一致，由 Center 内存注册表管理。

## Key technical decisions

- **One physical session per node and port:** Center 以节点和端口作为并发键，创建期间也保留占位，防止双击或并发请求重复打开。
- **Agent acknowledgement before success:** Center 等待 Agent 返回打开结果后才向浏览器返回会话，失败时回滚占位和数据库状态。
- **Server-enforced master role:** Center 根据已认证的浏览器连接和参与者 ID 校验输入，不信任前端按钮状态。
- **Ephemeral participants:** 浏览器参与者只在内存中存在；Center 重启后旧会话不恢复，符合已确认的非目标。
- **Short disconnect grace:** 最后一名浏览器离开后短暂等待再关闭串口，避免页面刷新造成无意义的快速关开。
- **Raw byte transport:** Base64 只用于传输；前端将解码后的 `Uint8Array` 直接交给终端。
- **Flow control options:** 第一版支持 `none` 与 `rtscts`；不提供 XON/XOFF 和厂商扩展模式。
- **Operator alias is separate from discovery:** 人工别名保存在持久配置中，不复用可能被 Agent 上报覆盖的设备描述。
- **Permissions:** `admin` 和 `operator` 可创建会话并成为主控；`readonly` 可观察但不能成为主控或写入。

## Alternatives considered

- **在 Center 所在机器直接打开串口:** rejected because 串口位于远程 Agent 节点。
- **每个浏览器各开一个物理串口:** rejected because 操作系统会冲突，也无法保证多人看到同一数据流。
- **复用现有 Session 行表示每个浏览器:** rejected because Agent 每三秒重建会话列表，浏览器参与状态会被覆盖。
- **使用 `go.bug.st/serial`:** rejected because其公开配置不支持已确认的 RTS/CTS 流控。
- **把参数保存在发现到的串口行:** rejected because设备拔出时该行会被删除，配置随之丢失。

## Rollout

- **Migration needed?** Yes；启动时由现有自动迁移创建配置表并增加会话字段，无需手写 SQL。
- **Backward compatible?** API 和协议为新增内容；旧 Agent 仍可上报，但尝试串口连接时会返回“需要升级 Agent”。
- **Feature flag?** No；仅在线且支持新命令的 Agent 显示可用连接操作。
- **Deployment order:** 先部署新 Agent，再部署 Center 和前端；同一 Docker 镜像中的二进制一起构建。

## Risks

- **Serial driver is mature but old:** 通过内部接口隔离依赖，并执行 macOS/Linux/Windows 交叉编译；真实硬件仍需最终验证。
- **Agent report currently rebuilds sessions every three seconds:** 串口管理器必须参与上报，并用协议字段区分物理会话，避免误删。
- **Browser disconnect races with refresh:** 使用短暂关闭宽限期，并在新参与者加入时取消关闭。
- **Agent disconnect can leave OS handles open:** Connector 断线回调关闭全部串口会话，再由上报修正 Center 状态。
- **No serial hardware in automated tests:** 使用假的端口实现覆盖字节流和清理逻辑，最后在 `/dev/cu.usbserial-*` 上做人工验证。

## Estimated effort

**L** — 涉及三个运行端、一个数据迁移、并发会话状态和真实设备验证；主要复杂度在生命周期一致性，而不是界面。
