# WindTerm 改造方案 — HubTerm Agent 集成

## 目标
WindTerm 作为 HubTerm 网络的受控节点：自发现 → 自上报 → 被管理 → 数据透传

## 架构

```
WindTerm 进程
├── 主界面（Qt GUI）
├── Pty（终端核心）
│   ├── readAll()  →  终端输出
│   ├── write()    →  用户输入
│   └── readyRead  →  有数据可读
└── HubTermAgent（新增模块）
    ├── 自发现: 启动时连接中心 WebSocket
    ├── 自上报: 上报本机能力（串口/SSH会话/系统信息）
    ├── 被管理: 接收中心指令
    └── 数据透传: hook Pty 的 I/O，推送到中心
```

## 新增文件

```
src/HubTerm/
├── Agent.h / Agent.cpp        ← HubTerm Agent 主类
├── Config.h / Config.cpp      ← 配置（中心地址、域、凭证）
├── Discovery.h / Discovery.cpp ← 自发现逻辑
├── Reporter.h / Reporter.cpp  ← 能力上报
├── Commander.h / Commander.cpp ← 指令接收与执行
├── TerminalShare.h / TerminalShare.cpp ← 终端数据透传
└── CMakeLists.txt
```

## 核心接口

### Agent（主入口）

```cpp
class HubTermAgent : public QObject {
    Q_OBJECT
public:
    explicit HubTermAgent(QObject *parent = nullptr);
    ~HubTermAgent();

    void start();           // 启动：发现→连接→注册→上报
    void stop();            // 优雅关闭

    void attachPty(Pty *pty);  // 绑定终端，开始透传
    void detachPty(Pty *pty);  // 解绑终端

signals:
    void connected();       // 已连接到中心
    void disconnected();    // 与中心断开
    void commandReceived(const QJsonObject &cmd);  // 收到指令

private:
    Config m_config;
    QWebSocket *m_ws;
    QTimer *m_reportTimer;
    QList<Pty*> m_attachedPtys;
};
```

### TerminalShare（数据透传）

```cpp
class TerminalShare : public QObject {
    Q_OBJECT
public:
    explicit TerminalShare(Pty *pty, QWebSocket *ws, QObject *parent = nullptr);

    // 当 Pty 有数据可读时调用
    void onPtyReadyRead();

    // 当中心发来写指令时调用
    void onRemoteWrite(const QByteArray &data);

    // 设置权限
    void setPermission(bool writable, bool readonly);

signals:
    void dataSent(const QByteArray &data);  // 数据已发送到中心
    void dataReceived(const QByteArray &data);  // 从中心收到数据

private:
    Pty *m_pty;
    QWebSocket *m_ws;
    bool m_writable;   // 中心是否可以写
    bool m_readonly;   // 是否只读模式
};
```

### Config（配置）

```cpp
class Config {
public:
    QString centerUrl;      // ws://hubterm.mycompany.com:8080/ws
    QString nodeId;         // 节点 UUID（持久化）
    QString nodeName;       // 节点名称（默认主机名）
    QString token;          // 认证 token
    int reportInterval;     // 上报间隔（秒）
    QString domain;         // 域（自动发现用）
};
```

## 数据流

### 终端输出（设备 → 用户/AI）

```
设备输出
  → Pty::readyRead()
  → TerminalShare::onPtyReadyRead()
  → Pty::readAll()
  → WebSocket 发送到中心
  → 中心广播给 Web 页面 / AI
  → 用户/AI 实时看到
```

### 终端输入（用户/AI → 设备）

```
用户/AI 敲命令
  → 中心 WebSocket 下发
  → TerminalShare::onRemoteWrite()
  → Pty::write(data)
  → 设备收到命令
```

### 能力上报

```
QTimer 每 3 秒触发
  → Reporter::collectCapabilities()
  → 采集: 串口列表 / SSH 会话 / CPU/内存 / 系统信息
  → WebSocket 发送到中心
  → 中心更新节点状态
```

### 指令接收

```
中心 WebSocket 下发
  → Agent::commandReceived(QJsonObject)
  → 解析指令类型:
      ├─ "connect" → 打开串口/SSH
      ├─ "disconnect" → 关闭连接
      ├─ "exec_script" → 执行 Python 脚本
      ├─ "set_permission" → 设置读写权限
      ├─ "update_config" → 更新配置
      └─ "restart" → 重启 Agent
```

## 改造步骤

### Step 1: 基础框架
- 创建 `src/HubTerm/` 目录和文件
- 实现 Config 加载（从文件/环境变量）
- 实现 WebSocket 连接和重连
- 修改 CMakeLists.txt 加入新模块

### Step 2: 自发现 + 自上报
- 实现 Discovery（启动时找中心）
- 实现 Reporter（采集能力、定时上报）
- 实现 Commander（解析和执行指令）

### Step 3: 终端数据透传
- 实现 TerminalShare（hook Pty I/O）
- 在 WindTerm 主窗口创建 Pty 时自动 attach
- 数据流推送中心

### Step 4: 脚本执行
- 接收中心下发的 Python 脚本
- 拉起子进程执行，I/O 通过 TerminalShare 透传
- 结果返回中心

## 注意
- 不破坏 WindTerm 现有功能
- HubTermAgent 是可选的（没有中心时 WindTerm 照常工作）
- 所有新增代码遵循 Apache-2.0 协议
