<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px">
      <div>
        <el-button link @click="$router.back()">← 返回</el-button>
        <span style="font-weight:bold;margin-left:10px">终端 - {{ route.params.nodeId }} / {{ route.params.portName }}</span>
      </div>
      <div>
        <el-button type="warning" size="small" @click="reconnect">重新连接</el-button>
        <el-button type="danger" size="small" @click="disconnect">断开</el-button>
      </div>
    </div>
    <div ref="terminalContainer" style="width:100%;height:calc(100vh - 180px);background:#1e1e1e"></div>
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
let term = null
let fitAddon = null
let ws = null

function connect() {
  const token = localStorage.getItem('token')
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  const wsUrl = `${protocol}//${host}/api/ws?token=${token}&node_id=${route.params.nodeId}&port=${route.params.portName}`

  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    term.clear()
    term.writeln('\x1b[32mConnected to terminal\x1b[0m')
  }

  ws.onmessage = (e) => {
    term.write(e.data)
  }

  ws.onclose = () => {
    term.writeln('\x1b[31mDisconnected\x1b[0m')
  }

  ws.onerror = () => {
    term.writeln('\x1b[31mConnection error\x1b[0m')
  }
}

function initTerminal() {
  term = new Terminal({
    cursorBlink: true,
    cursorStyle: 'block',
    fontSize: 14,
    fontFamily: 'Menlo, Monaco, "Courier New", monospace',
    theme: {
      background: '#1e1e1e',
      foreground: '#d4d4d4',
    },
  })

  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(terminalContainer.value)
  fitAddon.fit()

  term.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(data)
    }
  })

  connect()
}

function reconnect() {
  if (ws) {
    ws.close()
  }
  connect()
}

function disconnect() {
  if (ws) {
    ws.close()
  }
}

function handleResize() {
  if (fitAddon) {
    fitAddon.fit()
  }
}

onMounted(() => {
  initTerminal()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (ws) {
    ws.close()
  }
  if (term) {
    term.dispose()
  }
})
</script>
