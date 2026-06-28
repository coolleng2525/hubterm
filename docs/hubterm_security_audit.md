# HubTerm 安全审计跟踪

审计日期：2026-06-28  
复核日期：2026-06-28  
范围：HubTerm Center / Agent / Web / Tabby HubTerm 插件 / Dockerfile

本文由原始审计报告整理为跟踪清单。状态含义：

- 已修复：本轮或既有代码已经处理。
- 已不成立：当前代码路径已经不是原报告描述的风险。
- 待办：问题仍存在，但需要单独设计或较大改动。
- 接受风险：当前部署模型下风险较低，暂不修改。

## 安全风险

| 编号 | 原问题 | 状态 | 说明 |
| --- | --- | --- | --- |
| #1 | 脚本参数字符串替换导致注入 | 已修复 | `internal/pkg/script/engine.go` 不再把参数替换进源码；参数通过 argv 和环境变量传入。 |
| #2 | Dockerfile 硬编码内网代理 | 已修复 | 运行时镜像移除代理 ENV；构建阶段改为 `ARG HTTP_PROXY/HTTPS_PROXY`。 |
| #3 | Tabby Agent WebSocket 无认证 | 已不成立 | 当前插件用 `hubterm.node.<token>` WebSocket 子协议；Center 从 token 反查节点身份，不信任 query node_id。 |
| #4 | `JWT_SECRET` 同时用于 JWT 和凭据加密 | 已修复 | `securestore` 优先使用 `ENCRYPTION_KEY`；保留 `JWT_SECRET` fallback 兼容旧部署。新部署应配置 `ENCRYPTION_KEY`。 |
| #5 | 登录无速率限制 | 已修复 | `AuthHandler.Login` 增加按 IP 的 1 分钟 5 次内存限流；成功登录会清除该 IP 计数。 |
| #6 | `/api/nodes/report` CORS 通配符 | 接受风险 | 该接口不使用 Cookie，已有节点需要 Bearer token；典型 CSRF 条件不成立。首次注册仍是公开副作用入口，后续应配置化 Origin 白名单。 |
| #7 | Shell 参数二次注入 | 已修复 | shell 参数通过环境变量传入；`${name}` 展开为字面值，不会执行参数内的 `$(...)`。 |
| #8 | ProxySession map 无清理 | 已修复 | `ProxyHandler` 增加 24 小时 TTL 懒清理。 |
| #9 | ExecResult 全局 map 无清理 | 已修复 | exec result 增加 1 小时 TTL 懒清理。 |
| #10 | JWT 存 localStorage | 待办 | 当前仍使用 localStorage。改 HttpOnly Cookie 会影响前端鉴权和 WebSocket 子协议取 token，需要单独设计迁移。 |
| #11 | WebSocket token 在 URL query | 已不成立 | 当前浏览器和 agent WebSocket 均通过 `Sec-WebSocket-Protocol` 传 token。 |
| #12 | `os.Setenv` 传密钥存在隐含竞态 | 待办 | 当前仍通过环境变量桥接配置。需要把 JWT/加密密钥从配置对象注入到 middleware/securestore，属于架构改造。 |

## 架构与代码质量

| 编号 | 原问题 | 状态 | 说明 |
| --- | --- | --- | --- |
| #13 | 全局 DB 变量 | 待办 | handler 多数已显式持有 `*gorm.DB`，但 model 包仍有全局 DB；彻底移除需要迁移初始化和测试辅助。 |
| #14 | SQLite 未启用 WAL | 待办 | 仍建议在 DB 初始化中设置 `journal_mode=WAL`、`busy_timeout` 和 `foreign_keys`。 |
| #15 | 无请求追踪 ID | 待办 | 建议增加 Gin middleware 生成/透传 `X-Request-ID`。 |
| #16 | 缺少 pprof | 待办 | 建议以配置开关在 localhost 独立端口暴露，避免默认公网暴露。 |
| #17 | Tabby nodeId 使用 `Math.random()` | 已修复 | 新 nodeId 优先使用 `crypto.randomUUID()`，仅在不可用时 fallback。 |
| #18 | Tabby btoa/atob 不支持 UTF-8 | 已不成立 | 当前插件已用 `TextEncoder/TextDecoder` 做 base64 编解码。 |

## 功能与稳定性

| 编号 | 原问题 | 状态 | 说明 |
| --- | --- | --- | --- |
| #19 | 批量命令审计不完整 | 待办 | 属合规增强；建议记录每个节点的结果摘要、exit code 和失败原因。 |
| #20 | 脚本 stdout/stderr 无大小限制 | 已修复 | stdout/stderr 各限制为 10 MiB，超出追加 `[output truncated]`。 |
| #21 | 缺少节点离线检测 | 待办 | Agent WebSocket 断连会影响连接状态，但节点表持久状态仍需后台超时扫描统一标记 offline。 |

## 本轮修改摘要

- `Dockerfile`：移除硬编码代理，构建代理改为 build arg。
- `internal/pkg/script/engine.go`：删除运行时参数源码替换，改用 argv/env；增加输出大小限制。
- `internal/pkg/securestore/securestore.go`：引入 `ENCRYPTION_KEY`，兼容旧 `JWT_SECRET`。
- `internal/center/handler/auth.go`：增加登录限流。
- `internal/center/handler/proxy.go`：增加 proxy session TTL 清理。
- `internal/center/handler/agent_ws.go`：增加 exec result TTL 清理。
- `tabby-hubterm/src/hubterm.service.ts`：nodeId 优先用 `crypto.randomUUID()`。

## 后续优先级

1. 迁移 JWT 到 HttpOnly Cookie，同时保留 WebSocket 子协议鉴权方案。
2. 配置化 CORS Origin 白名单，并保留 Electron/file origin 的明确支持策略。
3. 将 JWT/加密密钥从 `os.Getenv` 改为启动时配置注入。
4. DB 初始化增加 SQLite WAL/busy timeout/foreign key pragma。
5. 增加节点离线扫描和批量命令结果审计。
