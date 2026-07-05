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
        "device_id": {
          "type": "string",
          "description": "The unique ID of the target device."
        },
        "cmd_id": {
          "type": "string",
          "description": "The command execution ID returned from hubterm_execute_command."
        }
      },
      "required": ["device_id", "cmd_id"]
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
