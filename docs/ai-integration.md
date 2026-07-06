# HubTerm AI Agent Integration Guide

HubTerm Center provides a set of AI-friendly REST API endpoints designed to enable LLM-based autonomous agents (e.g., Antigravity, XiaoZhu) to perform device discovery, network topology mapping, remote diagnostics, and automated script execution.

This document outlines the available endpoints, authentication mechanisms, JSON payloads, and integration guidelines.

---

## 1. Authentication

All AI-friendly API routes require operator-level authentication. You must include the authentication token in the request header:

```http
Authorization: Bearer <your_operator_token>
Content-Type: application/json
```

---

## 2. API Endpoints Reference

### 2.1 Device Discovery
Retrieves all currently online devices along with their hardware capabilities, serial ports, and supported protocols.

*   **URL:** `GET /api/v1/devices`
*   **Response (200 OK):**
    ```json
    {
      "devices": [
        {
          "device_id": "ap-03",
          "name": "AccessPoint-Floor3",
          "type": "cisco_ap",
          "status": "online",
          "capabilities": ["console", "ping", "system_log"],
          "protocols": ["serial", "ssh"],
          "node_id": "node-latitude-e7470"
        }
      ]
    }
    ```

### 2.2 Get Device Capabilities
Queries capabilities and port maps for a specific target device.

*   **URL:** `GET /api/v1/devices/:id/capabilities`
*   **Response (200 OK):**
    ```json
    {
      "device_id": "ap-03",
      "name": "AccessPoint-Floor3",
      "type": "cisco_ap",
      "capabilities": ["console", "ping", "system_log"],
      "protocols": ["serial", "ssh"]
    }
    ```

### 2.3 Execute Command on Device
Dispatches a raw command to be run on the target device via its managing agent node. Command execution is asynchronous.

*   **URL:** `POST /api/v1/devices/:id/exec`
*   **Request Body:**
    ```json
    {
      "command": "show ip interface brief",
      "timeout": 15
    }
    ```
*   **Response (200 OK):**
    ```json
    {
      "cmd_id": "550e8400-e29b-41d4-a716-446655440000",
      "status": "pending"
    }
    ```

### 2.4 Query Execution Result
Checks the status and output of an asynchronously executed command.

*   **URL:** `GET /api/v1/devices/:id/exec/:cmd_id`
*   **Response (200 OK):**
    *   *If Pending:*
        ```json
        {
          "status": "pending"
        }
        ```
    *   *If Completed:*
        ```json
        {
          "status": "completed",
          "result": {
            "stdout": "Interface                  IP-Address      OK? Method Status                Protocol\nGigabitEthernet0/0         192.168.1.10    YES NVRAM  up                    up",
            "stderr": "",
            "exit_code": 0,
            "duration_ms": 420
          }
        }
        ```

### 2.5 Asynchronously Upload & Run Script
Uploads a script (Python or Shell) and executes it on one or multiple target devices or agent nodes simultaneously.

*   **URL:** `POST /api/v1/scripts`
*   **Request Body:**
    ```json
    {
      "name": "cpu-check",
      "description": "Collects CPU load percentages",
      "language": "python",
      "source": "import os\nload = os.getloadavg()\nprint(f'CPU Load: {load}')",
      "params": [],
      "targets": ["node-latitude-e7470", "ap-03"],
      "timeout": 30
    }
    ```
*   **Response (201 Created):**
    ```json
    {
      "script_id": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb7d",
      "results": [
        {
          "target": "node-latitude-e7470",
          "cmd_id": "8c7d6e5a-4b3c-2d1e-0f9a-8b7c6d5e4f3a",
          "status": "pending"
        },
        {
          "target": "ap-03",
          "status": "failed",
          "error": "target ap-03 not found or offline"
        }
      ]
    }
    ```

---

## 3. How to Equip an LLM Agent with HubTerm Tools

To enable an AI Agent (like OpenAI GPT, Claude, or Gemini) to use these APIs, define them as **Function Tools** in the agent's workspace configuration.

Below are the tool definitions in standard JSON Schema format:

```json
[
  {
    "name": "hubterm_discover_devices",
    "description": "Discover all online devices and agent nodes managed by HubTerm Center.",
    "parameters": {
      "type": "object",
      "properties": {}
    }
  },
  {
    "name": "hubterm_get_device",
    "description": "Get full details for a specific device using its ID.",
    "parameters": {
      "type": "object",
      "properties": {
        "device_id": {
          "type": "string",
          "description": "The unique ID of the target device."
        }
      },
      "required": ["device_id"]
    }
  },
  {
    "name": "hubterm_get_device_capabilities",
    "description": "Get capabilities and protocols for a specific device using its ID.",
    "parameters": {
      "type": "object",
      "properties": {
        "device_id": {
          "type": "string",
          "description": "The unique ID of the target device."
        }
      },
      "required": ["device_id"]
    }
  },
  {
    "name": "hubterm_execute_command",
    "description": "Execute a CLI command asynchronously on a specific device using its ID.",
    "parameters": {
      "type": "object",
      "properties": {
        "device_id": {
          "type": "string",
          "description": "The unique ID of the target device."
        },
        "command": {
          "type": "string",
          "description": "The CLI command to run (e.g. 'show ip route', 'df -h')."
        },
        "timeout": {
          "type": "integer",
          "description": "Command execution timeout in seconds (default: 30)."
        }
      },
      "required": ["device_id", "command"]
    }
  },
  {
    "name": "hubterm_get_command_result",
    "description": "Retrieve stdout, stderr, and execution status of a previously triggered command.",
    "parameters": {
      "type": "object",
      "properties": {
        "cmd_id": {
          "type": "string",
          "description": "The command execution ID returned from hubterm_execute_command."
        }
      },
      "required": ["cmd_id"]
    }
  },
  {
    "name": "hubterm_send_terminal_input",
    "description": "Send input to an online terminal session discovered from active sessions or SSH terminals.",
    "parameters": {
      "type": "object",
      "properties": {
        "device_id": {
          "type": "string",
          "description": "The discovered terminal device ID, for example 'com9-r770'."
        },
        "session_id": {
          "type": "string",
          "description": "Optional raw HubTerm session ID. Use this instead of device_id when available."
        },
        "input": {
          "type": "string",
          "description": "Text to send to the terminal."
        },
        "append_newline": {
          "type": "boolean",
          "description": "Append Enter/CR after input. Default: true."
        }
      },
      "required": ["input"]
    }
  },
  {
    "name": "hubterm_get_terminal_output",
    "description": "Fetch recent output from an online terminal session.",
    "parameters": {
      "type": "object",
      "properties": {
        "device_id": {
          "type": "string",
          "description": "The discovered terminal device ID, for example 'com9-r770'."
        },
        "session_id": {
          "type": "string",
          "description": "Optional raw HubTerm session ID."
        },
        "limit": {
          "type": "integer",
          "description": "Maximum recent output records to return. Default: 50, max: 200."
        },
        "include_input": {
          "type": "boolean",
          "description": "Include echoed input records. Default: false."
        }
      }
    }
  },
  {
    "name": "hubterm_upload_and_run_script",
    "description": "Upload a Python or shell script and execute it on one or more devices or agent nodes.",
    "parameters": {
      "type": "object",
      "properties": {
        "name": { "type": "string", "description": "Script name." },
        "description": { "type": "string", "description": "Optional script description." },
        "language": { "type": "string", "description": "python or shell. Default python." },
        "source": { "type": "string", "description": "Script source code." },
        "params": { "type": "array", "description": "Optional script parameter definitions." },
        "targets": {
          "type": "array",
          "items": { "type": "string" },
          "description": "Device IDs or node IDs."
        },
        "timeout": { "type": "integer", "description": "Execution timeout in seconds. Default 30." }
      },
      "required": ["name", "source", "targets"]
    }
  }
]
```

### AI Agent Loop Example:
```
1. User: "检查 3 楼 AP 的 CPU 和接口状态。"
2. Agent calls: hubterm_discover_devices()
3. Tool Output: {"devices": [{"device_id": "ap-03", "name": "AccessPoint-Floor3", "capabilities": ["console"]}]}
4. Agent calls: hubterm_execute_command(device_id="ap-03", command="show cpu; show ip interface brief")
5. Tool Output: {"cmd_id": "cmd-abc-123", "status": "pending"}
6. Agent waits 1 second, then calls: hubterm_get_command_result(device_id="ap-03", cmd_id="cmd-abc-123")
7. Tool Output: {"status": "completed", "result": {"stdout": "CPU load: 12% ...", "exit_code": 0}}
8. Agent reports back to the user with the diagnosed CPU load and interface states.
```


---

## 4. MCP Server Endpoint

HubTerm Center also exposes an MCP-compatible JSON-RPC endpoint for AI tools that can connect to HTTP MCP servers.

* **URL:** `POST /api/mcp`
* **Auth:** `Authorization: Bearer <operator_or_admin_token>`
* **Transport:** HTTP JSON-RPC 2.0

`hubterm_discover_devices` returns both registered online devices and active HubTerm terminal sessions. If an AP console session is named `com9-r770` in the HubTerm UI, MCP exposes it as a discoverable terminal device with `device_id: "com9-r770"` and capability `terminal_input`. `hubterm_execute_command` accepts that same `device_id`; for terminal-session devices it sends the command into the live terminal and `hubterm_get_command_result` returns the recent terminal output as `stdout`.

Available tools:

| Tool | Purpose |
| --- | --- |
| `hubterm_discover_devices` | Discover online HubTerm devices. |
| `hubterm_get_device` | Get full details for one device. |
| `hubterm_get_device_capabilities` | Get capabilities and protocols for one device. |
| `hubterm_execute_command` | Execute a command asynchronously on a registered device, or send command text to a discovered online terminal session such as `com9-r770`. |
| `hubterm_get_command_result` | Fetch command status/output by `cmd_id`. |
| `hubterm_send_terminal_input` | Send input to an online AP console/SSH terminal session. |
| `hubterm_get_terminal_output` | Fetch recent output from an online AP console/SSH terminal session. |
| `hubterm_upload_and_run_script` | Upload and execute a Python or shell script on devices or nodes. |

Example initialization request:

```bash
curl -sS "$HUBTERM_CENTER_URL/api/mcp" \
  -H "Authorization: Bearer $HUBTERM_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"example","version":"0.1"}}}'
```

Example tool discovery request:

```bash
curl -sS "$HUBTERM_CENTER_URL/api/mcp" \
  -H "Authorization: Bearer $HUBTERM_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
```

Example tool call:

```bash
curl -sS "$HUBTERM_CENTER_URL/api/mcp" \
  -H "Authorization: Bearer $HUBTERM_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"hubterm_discover_devices","arguments":{}}}'
```

Example sending input to an online AP console session:

```bash
curl -sS "$HUBTERM_CENTER_URL/api/mcp" \
  -H "Authorization: Bearer $HUBTERM_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"hubterm_send_terminal_input","arguments":{"device_id":"com9-r770","input":"show version"}}}'
```

Example reading recent output from that terminal session:

```bash
curl -sS "$HUBTERM_CENTER_URL/api/mcp" \
  -H "Authorization: Bearer $HUBTERM_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"hubterm_get_terminal_output","arguments":{"device_id":"com9-r770","limit":20}}}'
```

Client configuration example: [`docs/mcp-client-config.example.json`](mcp-client-config.example.json)

```json
{
  "mcpServers": {
    "HubTerm": {
      "url": "http://<HUBTERM_CENTER_HOST>:<PORT>/api/mcp",
      "headers": {
        "Authorization": "Bearer <HUBTERM_MCP_TOKEN>"
      }
    }
  }
}
```

For clients that only support command-based MCP servers, use a small bridge/adapter command that forwards MCP JSON-RPC requests to `http://<HUBTERM_CENTER_HOST>:<PORT>/api/mcp` with the bearer token header.
