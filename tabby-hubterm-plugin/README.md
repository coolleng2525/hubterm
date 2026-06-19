# tabby-hubterm

HubTerm Agent plugin for [Tabby](https://github.com/Eugeny/tabby) terminal.

## Features

- **Auto-discovery**: Connects to HubTerm center on startup
- **Capability reporting**: Reports terminal sessions, platform info
- **Remote management**: Receives commands from center (write, disconnect, restart)
- **Terminal sharing**: Terminal I/O is streamed to HubTerm in real-time

## Installation

### From Plugin Manager

1. Open Tabby → Settings → Plugins
2. Search for `tabby-hubterm`
3. Click Install

### Manual

```bash
npm install -g tabby-hubterm
```

Or clone and build:

```bash
git clone https://github.com/coolleng2525/hubterm
cd hubterm/tabby-hubterm-plugin
npm install
npm run build
```

Then copy the `dist/` folder to Tabby's plugins directory.

## Configuration

1. Open Tabby → Settings → HubTerm
2. Enable HubTerm
3. Set Center URL (e.g., `ws://your-center:8080/ws`)
4. Set Node Name and Domain (optional)
5. Set Token if required

## Architecture

```
Tabby Terminal
├── HubTermService          ← WebSocket connection to center
│   ├── register()         ← Register node identity
│   ├── sendReport()       ← Report sessions & capabilities
│   └── handleCommand()    ← Execute center commands
├── HubTermDecorator       ← Terminal I/O hook
│   ├── attach()           ← Hook terminal output/input
│   └── detach()           ← Unhook
└── HubTermSettingsTab     ← Configuration UI
```

## Protocol

### Node → Center

```json
{
  "type": "register",
  "node_id": "uuid",
  "node_name": "my-workstation",
  "token": "...",
  "domain": "mycompany.com"
}
```

```json
{
  "type": "report",
  "node_id": "uuid",
  "hostname": "host",
  "platform": "windows",
  "sessions": [{"id": "...", "type": "ssh", "name": "server-01"}],
  "capabilities": ["tabby-terminal", "serial", "ssh"]
}
```

```json
{
  "type": "terminal_data",
  "node_id": "uuid",
  "session": {"id": "...", "type": "ssh", "name": "server-01"},
  "data": "base64-encoded-terminal-output"
}
```

### Center → Node

```json
{"type": "ping"}
{"type": "write", "payload": {"session_id": "...", "data": "base64"}}
{"type": "disconnect", "payload": {"session_id": "..."}}
{"type": "set_permission", "payload": {"session_id": "...", "writable": false}}
{"type": "update_config", "payload": {"centerUrl": "ws://..."}}
{"type": "restart"}
```

## License

Apache-2.0
