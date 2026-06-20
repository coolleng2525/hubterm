<template>
  <div>
    <div class="terminal-header">
      <div>
        <el-button link @click="$router.back()">← 返回</el-button>
        <span class="terminal-title">共享终端 · {{ route.params.sessionId }}</span>
        <el-tag :type="connected ? 'success' : 'info'" size="small">
          {{ connected ? '已连接' : '未连接' }}
        </el-tag>
      </div>
      <div>
        <el-button type="warning" size="small" @click="connect">重新连接</el-button>
        <el-button type="danger" size="small" @click="disconnect">断开</el-button>
      </div>
    </div>
    <div ref="terminalContainer" class="terminal-container"></div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'

const route = useRoute()
const terminalContainer = ref(null)
const connected = ref(false)
let term
let fitAddon
let ws

function bytesToBase64(value) {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  for (let i = 0; i < bytes.length; i += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000))
  }
  return btoa(binary)
}

function base64ToText(value) {
  const binary = atob(value)
  const bytes = Uint8Array.from(binary, char => char.charCodeAt(0))
  return new TextDecoder().decode(bytes)
}

function send(type, data) {
  if (ws?.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type, data }))
  }
}

function connect() {
  disconnect()
  const token = localStorage.getItem('token')
  if (!token) return
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  ws = new WebSocket(`${protocol}//${window.location.host}/api/ws`, [
    'hubterm',
    `hubterm.auth.${token}`,
  ])
  ws.onopen = () => {
    connected.value = true
    send('terminal_subscribe', {
      node_id: route.params.nodeId,
      session_id: route.params.sessionId,
    })
    term.writeln('\x1b[32mConnected to shared terminal\x1b[0m')
  }
  ws.onmessage = event => {
    try {
      const message = JSON.parse(event.data)
      if (message.type === 'error') {
        term.writeln(`\r\n\x1b[31m${message.data?.message || 'Request failed'}\x1b[0m`)
        return
      }
      const terminal = message.data?.terminal
      if (
        message.type === 'terminal_data' &&
        message.data?.node_id === route.params.nodeId &&
        terminal?.session_id === route.params.sessionId &&
        terminal.direction === 'output'
      ) {
        term.write(base64ToText(terminal.data))
      }
    } catch {
      term.writeln('\r\n\x1b[31mInvalid message from center\x1b[0m')
    }
  }
  ws.onclose = () => {
    connected.value = false
    term.writeln('\r\n\x1b[31mDisconnected\x1b[0m')
  }
  ws.onerror = () => term.writeln('\r\n\x1b[31mConnection error\x1b[0m')
}

function disconnect() {
  if (ws) {
    ws.onclose = null
    ws.close()
    ws = null
  }
  connected.value = false
}

function handleResize() {
  fitAddon?.fit()
}

onMounted(() => {
  term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: 'Menlo, Monaco, "Courier New", monospace',
    theme: { background: '#1e1e1e', foreground: '#d4d4d4' },
  })
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(terminalContainer.value)
  fitAddon.fit()
  term.onData(data => {
    send('terminal_input', {
      node_id: route.params.nodeId,
      session_id: route.params.sessionId,
      data: bytesToBase64(data),
    })
  })
  window.addEventListener('resize', handleResize)
  connect()
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  disconnect()
  term?.dispose()
})
</script>

<style scoped>
.terminal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}
.terminal-title {
  margin: 0 10px;
  font-weight: 600;
}
.terminal-container {
  width: 100%;
  height: calc(100vh - 180px);
  min-height: 360px;
  background: #1e1e1e;
}
</style>
