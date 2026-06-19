# HubTerm - Serial/SSH Cluster Management Platform

[中文](README.md)

A serial/SSH cluster management platform based on **lightweight cluster architecture** (node reporting + central management, no traffic relay).

## Architecture

```
Center Service ← HTTP REST + WebSocket → Agent × N
    ↑                                      ↑
Web UI (Vue 3 + xterm.js)           WindTerm (HubTerm Agent integrated)
    ↑                                      ↑
 User/AI                             User/AI
```

- **Center Service**: Data aggregation, unified management UI, global audit
- **Agent**: Runs on each machine, collects system status, scans serial ports, reports to center every 3s
- **WindTerm**: HubTerm Agent integrated — auto-discovery, capability reporting, terminal I/O streaming
- **Web UI**: Vue 3 + xterm.js web terminal

## Quick Start

### Start Center

```bash
cd cmd/center
export JWT_SECRET=your-secret-key
go run main.go
```

Center listens on `:8080`. First run auto-creates admin user; password from `ADMIN_PASSWORD` env var (random generated if not set).

### Start Agent

```bash
cd cmd/agent
go run main.go --center http://localhost:8080
```

Agent reports system status and serial port info every 3 seconds.

### Start Frontend

```bash
cd web
npm install
npm run dev
```

Dev server on `:3000`, proxies `/api` to `:8080`.

### Use WindTerm (HubTerm integrated)

1. Start WindTerm — auto-connects to HubTerm center
2. Auto-reports serial ports, SSH sessions, system info
3. See the node online in HubTerm Web UI
4. Terminal I/O streamed in real-time to Web UI and AI
5. Read-only / writable permission control

## Build & Release

Tag to trigger GitHub Release:

```bash
git tag v1.0.0
git push origin v1.0.0
```

Auto-builds linux/windows/macOS × amd64/arm64 binaries.

## Project Structure

```
hubterm/
├── cmd/
│   ├── center/       # Center service entry
│   └── agent/        # Agent entry
├── internal/
│   ├── center/       # Center business logic
│   │   ├── handler/  # HTTP handlers
│   │   ├── model/    # Data models
│   │   ├── service/  # Business logic
│   │   └── middleware/ # JWT middleware
│   ├── agent/        # Agent logic
│   │   ├── collector/ # Status collection
│   │   └── reporter/  # Reporting logic
│   └── proto/        # Shared protocol definitions
├── web/              # Vue 3 frontend
│   └── src/
│       ├── views/    # Pages
│       ├── api/      # API calls
│       └── components/ # Components
└── README.md
```

## API Reference

| Method | Path | Description |
|--------|------|-------------|
| POST | /api/auth/login | Login |
| POST | /api/auth/register | Register (admin only) |
| GET | /api/auth/profile | Profile |
| GET | /api/nodes | Node list |
| GET | /api/nodes/:id | Node detail |
| POST | /api/nodes/report | Node report |
| POST | /api/nodes/:id/command | Send command |
| GET | /api/serial-ports | Serial port list |
| GET | /api/sessions | Session list |
| POST | /api/sessions/:id/kick | Kick session |
| POST | /api/sessions/:id/assign-master | Assign master |
| GET | /api/audit-logs | Audit logs |
| WS | /api/ws | WebSocket real-time push |

## Tech Stack

- **Backend**: Go + Gin + GORM + SQLite
- **Frontend**: Vue 3 + Vite + Element Plus + xterm.js
- **Auth**: JWT
- **Node Communication**: HTTP REST + WebSocket

## References & Credits

HubTerm stands on the shoulders of giants. The following open-source projects were referenced or integrated:

| Project | Usage | License |
|---------|-------|---------|
| [Next Terminal](https://github.com/dushixiang/next-terminal) | Bastion host architecture: SSH tunnel/jump host, session observer pattern, WebSocket terminal protocol, session recording | AGPL-3.0 |
| [WindTerm](https://github.com/kingToolbox/WindTerm) | Terminal core (Pty/SSH/Serial), HubTerm Agent integrated | Apache-2.0 |
| [Tabby](https://github.com/Eugeny/tabby) | Terminal plugin system, tabby-hubterm plugin developed | MIT |
| [ser2net](https://sourceforge.net/projects/ser2net/) | Serial→TCP mapping (optional agent mode) | GPL-2.0 |
| [Headscale](https://github.com/juanfont/headscale) | Auto-discovery / mesh networking / encrypted tunnel (planned) | BSD-3-Clause |
| [Tailscale](https://github.com/tailscale/tailscale) | Mesh VPN architecture reference (planned) | BSD-3-Clause |

> HubTerm doesn't reinvent the wheel — it assembles wheels into a vehicle, doing what no existing project does: AI execution environment, script engine, unified device abstraction.
