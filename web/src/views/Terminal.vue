<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px">
      <div>
        <el-button link @click="$router.back()">← 返回</el-button>
		<span style="font-weight:bold;margin-left:10px">SSH终端 - {{ route.params.nodeId }}</span>
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
import { getNode } from '../api'

const route = useRoute()
const terminalContainer = ref(null)
let term = null
let fitAddon = null
let ws = null

async function connect() {
	const token = localStorage.getItem('token')
	if (!token) return
	let node
	try {
		node = (await getNode(route.params.nodeId)).data.node
	} catch (e) {
		term.writeln('\x1b[31mUnable to load node information\x1b[0m')
		return
	}
	const username = window.prompt('SSH username', 'root')
	if (username === null) return
	const password = window.prompt('SSH password', '')
	if (password === null) return
	const sshPort = Number(window.prompt('SSH port', '22')) || 22
	const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
	const host = window.location.host
	const wsUrl = `${protocol}//${host}/api/v1/terminal/connect`

	ws = new WebSocket(wsUrl, ['hubterm', `hubterm.auth.${token}`])

	ws.onopen = () => {
		term.clear()
		const sessionId = crypto.randomUUID
			? crypto.randomUUID()
			: `${Date.now()}-${Math.random().toString(16).slice(2)}`
		ws.send(JSON.stringify({
			session_id: sessionId,
			protocol: 'ssh',
			ip: node.ip,
			port: sshPort,
			username,
			password,
			cols: term.cols,
			rows: term.rows,
			term_type: 'xterm-256color',
		}))
	}

	ws.onmessage = (e) => {
		try {
			const msg = JSON.parse(e.data)
			if (msg.type === 2) term.write(msg.content)
			if (msg.type === 1) term.writeln('\x1b[32mConnected to terminal\x1b[0m')
			if (msg.type === 0) term.writeln(`\r\n\x1b[31m${msg.content || 'Disconnected'}\x1b[0m`)
		} catch {
			term.write(e.data)
		}
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
			ws.send(JSON.stringify({ type: 2, content: data }))
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
		if (ws && ws.readyState === WebSocket.OPEN) {
			const size = btoa(JSON.stringify({ rows: term.rows, cols: term.cols }))
			ws.send(JSON.stringify({ type: 3, content: size }))
		}
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
