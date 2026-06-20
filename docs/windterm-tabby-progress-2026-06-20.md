# WindTerm / Tabby HubTerm 集成进度交接（2026-06-20）

本文记录 2026-06-20 的实际检查、修改和验证结果，供后续 Agent 继续处理。

## 当前仓库位置

- HubTerm：`/code/hubterm_project/hubterm/`
- Tabby：`/code/hubterm_project/tabby-source/`
- WindTerm：`/code/hubterm_project/WindTerm/`

旧文档中的 `/mnt/nas/...` 路径已经过期。三个仓库当前都位于本地磁盘 `/code/hubterm_project/`。

## Tabby：已完成的部分

### 1. 依赖安装和完整构建已验证

环境：Node.js 22.22.2、Yarn 1.22.22。

根依赖及各子包依赖已安装。最终执行：

```bash
cd /code/hubterm_project/tabby-source
yarn build
```

结果：退出码 0。`tabby-hubterm` 的 typings 和 webpack bundle 均成功生成；完整 Tabby/Web 构建也成功，仅有原项目已有的 webpack/Sass/资源体积警告。

### 2. 已修改但尚未提交的文件

```text
tabby-hubterm/package.json
tabby-hubterm/yarn.lock
tabby-hubterm/src/configProvider.ts
tabby-hubterm/src/hubterm.service.ts
tabby-hubterm/src/settingsTabProvider.ts
tabby-hubterm/src/terminalDecorator.ts
```

修改内容：

- 删除插件目录中错误的 Angular 21、ng-bootstrap 20 等开发依赖。Tabby 主仓使用 Angular 15；嵌套安装 Angular 21 会污染 webpack 模块解析，导致 `InjectFlags`、`NgbModal` 等大量编译错误。
- `yarn.lock` 随错误依赖移除而更新。
- 删除未使用的 `Injector`、`Platform`、`TerminalDecorator` import。
- 为 `BaseTerminalTabComponent` 补充 `<any>` 泛型参数，以匹配当前 Tabby API。

不要恢复 `tabby-hubterm/package.json` 中原来的 Angular 21 devDependencies。

### 3. 构建过程中的环境信息

第一次运行子包安装脚本时，`app/node_modules/fontmanager-redux` 因缺少 `fontconfig/fontconfig.h` 构建失败，但安装脚本继续完成了各子包依赖。该 native 模块失败没有阻止最终 `yarn build` 成功。如后续需要打包可运行的 Electron 安装包，应安装 `libfontconfig1-dev` 后重建 native 模块。

## Tabby：尚未完成，下一位 Agent 应优先处理

目前只是“可编译”，尚未完成 Center 协议联调。已确认存在以下不兼容：

1. 插件默认配置是 `ws://localhost:8080/ws`，但 Center 的 Agent 端点是 `/api/ws/agent?node_id=...`。
2. Center 的 Agent WebSocket 当前只读取 `Authorization: Bearer <node-token>`。浏览器/Electron 原生 `WebSocket` 构造器不能设置 Authorization header。建议 Center 增加 `hubterm.node.<token>` WebSocket 子协议认证，插件使用 `new WebSocket(url, ['hubterm', 'hubterm.node.' + token])`。
3. Center 使用统一封装 `{ "type": "...", "data": ... }`。插件的 `report` 和 `terminal_data` 当前把字段平铺在顶层，格式不兼容。
4. Center 的 `AgentWSHandler` 对 `report` 目前仅忽略，对 `terminal_data` 没有处理。因此终端流虽然由插件发送，Center 不会消费或转发。
5. Center 下发命令格式是 `{type, data: ExecCommand}`，插件当前读取 `payload` 顶层，无法正确执行 Center 命令。
6. `TerminalDecorator` 对 `output$` 和 `input$` 都发送相同 `terminal_data`，没有方向字段，并且订阅在 detach 时没有取消，可能重复上报和泄漏订阅。
7. `btoa(data)` 不能安全处理任意 Unicode；输入流还是 `Buffer`，当前 `String(buffer)` 也不适合作为二进制协议编码。

建议先定义清晰的 Agent 终端共享消息结构（至少包含 `session_id`、`direction`、base64 bytes），同时修改 HubTerm Center 和 Tabby 插件，并增加 Go handler 测试及 TypeScript 单元/协议测试，再做真实 WebSocket 联调。

## WindTerm：检查结果

没有修改 WindTerm 仓库，仓库当前干净。

关键结论：当前 fork 是 WindTerm 的“部分开源源码”，只有约 547 个跟踪文件。仓库没有顶层 `CMakeLists.txt` 或 `.pro` 工程文件，只有 `src/libssh/CMakeLists.txt`。因此现有 README 和集成文档中的以下命令不可执行：

```bash
mkdir build && cd build
cmake .. -DCMAKE_PREFIX_PATH=/path/to/qt
make
```

服务器是 Ubuntu 24.04，已有 gcc/g++，但没有 cmake、qmake 和 Qt 开发环境。即使安装 Qt，也无法从该仓库构建完整 WindTerm 应用，因为完整应用工程及闭源部分不在仓库内。

另外，`src/HubTerm/Agent.cpp` 当前同样使用旧的顶层 `report` 消息格式、未携带符合 Center 要求的 node token 鉴权，并连接文档中的 `/ws`，所以即便单独编译通过也无法与当前 Center 工作。

后续建议二选一：

- 如果目标是完整 WindTerm 集成：必须先取得 WindTerm 完整可构建源码/官方插件接口和真实工程文件，再把 `src/HubTerm` 接入真实构建目标。
- 如果只能使用当前部分开源仓库：把 `src/HubTerm` 明确定位为独立原型库，新增自己的 CMake 工程和 mock Pty 编译测试；不要声称已集成或可构建完整 WindTerm。

## 当前 Git 状态

- `/code/hubterm_project/tabby-source/`：上述 6 个文件有未提交修改。
- `/code/hubterm_project/hubterm/`：写入本文档前为干净状态。
- `/code/hubterm_project/WindTerm/`：干净。

本轮没有提交任何仓库，也没有继续修改 Center 协议或 WindTerm 源码。


## 2026-06-20 后续推进：Center / Tabby 协议已对齐

本轮已完成以下修改（仍未提交）：

- Center Agent WebSocket 支持 `hubterm.node.<token>` 子协议鉴权，同时保留 `Authorization: Bearer`。
- token 作为节点身份的权威来源；Center 从数据库 token 反查真实 node ID，不再信任浏览器端随机 query node ID。
- Tabby 默认端点改为 `/api/ws/agent`，自动追加 `node_id`，通过 WebSocket 子协议携带 token。
- Agent 消息统一为 `{"type":"...","data":...}`；Center 命令按 `data.payload` 解析。
- 新增终端流结构：`session_id`、`direction`（input/output）、base64 bytes。
- Tabby 使用 TextEncoder/Uint8Array 安全处理 Unicode 与二进制，不再直接 `btoa(string)`。
- TerminalDecorator 在 detach 时取消 RxJS 订阅，避免重复上报与泄漏。
- Center 校验终端流方向、base64 和 1 MiB 单消息上限，并转发到已认证的浏览器 WebSocket。
- Center 的浏览器 WebSocket 广播写入已串行化，避免并发写同一 Gorilla WebSocket。
- 新增 `agent_ws_protocol_test.go`，覆盖 Header/子协议鉴权、终端流校验、真实 WebSocket token→canonical node ID 行为。

验证结果：

- `cd hubterm && go test ./...`：通过。
- `cd tabby-source/tabby-hubterm && yarn build`：通过。

仍待真实环境联调：

1. 启动 Center，准备一个已有 node token。
2. 在 Tabby 设置中填写 Center WebSocket URL 和 token，启用插件。
3. 验证连接、会话 report、input/output terminal_data，以及 Center 下发 disconnect/write。
4. 浏览器目前只收到通用 `terminal_data` 广播；还需把前端终端共享页面接到该消息并按 node/session 过滤。


## 2026-06-20 后续推进（二）：浏览器共享终端链路闭环

本轮继续完成：

- Tabby 的 WebSocket report 已改为 Center NodeReport 兼容字段；每个 Tab 分配稳定 session ID。
- Center 消费 Agent report，更新节点在线状态并增量同步会话表，自动清理该节点已消失的会话。
- report 限制最多 1000 个会话，并拒绝 Agent 抢占其他节点已有 session ID。
- 浏览器 /api/ws 增加 terminal_subscribe 与 terminal_input：只有显式订阅相同 node/session 的浏览器才能收到终端流，只有 admin/operator 可以发送输入，输入前再次校验会话真实归属。
- Center 增加浏览器到 Agent 的 write 命令，payload 包含 session_id 和 base64 data。
- Tabby 支持 write、kick_session、assign_master 命令，并按稳定 session ID 找到 Tab。
- Web 前端新增 SharedTerminal.vue 和 /shared-terminal/:nodeId/:sessionId 路由。
- 节点详情的在线会话列表新增“共享终端”入口。
- 新增 agent_terminal_e2e_test.go：真实模拟 Agent WS + Browser WS，验证 report 落库、中文输出转发、浏览器输入回传 Agent 的完整闭环。

验证结果：

- cd hubterm && go test ./...：通过。
- cd hubterm/web && npm run build：通过（仅保留原有 bundle size 警告）。
- cd tabby-source/tabby-hubterm && yarn build：通过。
- 内置 Browser 因当前桌面线程的 T: 工作目录失效而无法启动，因此尚未完成页面截图、控制台和实际点击验证；未擅自改用其他浏览器驱动。

下一步：

1. 修复或重开工作区，使本地 T:\\code\\hubterm_project 可作为进程工作目录。
2. 启动 Center 与 Web，使用真实 Tabby token 联调。
3. 在浏览器节点详情点击“共享终端”，验证状态标签、中文输出、键盘输入、重连与 readonly 拒绝提示。
