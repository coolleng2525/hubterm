<template>
  <div>
    <div class="terminal-header">
      <div class="terminal-title-row">
        <el-button link @click="$router.back()">← 返回</el-button>
        <span class="terminal-title">共享终端 · {{ sessionDisplayName || route.params.sessionId }}</span>
        <el-tag :type="connected ? 'success' : 'info'" size="small">
          {{ connected ? '已连接' : '未连接' }}
        </el-tag>
      </div>
      <div class="connection-actions">
        <el-button type="warning" size="small" @click="connect">重新连接</el-button>
        <el-button type="danger" size="small" @click="disconnect">断开</el-button>
      </div>
    </div>

    <div class="terminal-toolbar">
      <div class="terminal-actions">
        <el-input
          ref="searchInputRef"
          v-model="searchQuery"
          class="terminal-search"
          clearable
          size="small"
          placeholder="查找终端内容"
          @keyup.enter="findNext"
          @clear="clearSearch"
        />
        <el-checkbox v-model="caseSensitive" size="small" @change="repeatSearch">Aa</el-checkbox>
        <el-button size="small" :disabled="!searchQuery" @click="findPrevious">上一个</el-button>
        <el-button size="small" :disabled="!searchQuery" @click="findNext">下一个</el-button>
      </div>

      <div class="terminal-sender" style="display:flex;align-items:center;gap:8px;margin-left:15px">
        <span class="config-label">快速发送</span>
        <el-select
          v-model="selectedScriptSource"
          clearable
          size="small"
          placeholder="选择预设脚本/命令"
          style="width:180px"
          @change="handleScriptChange"
        >
          <el-option
            v-for="script in scripts"
            :key="script.script_id"
            :label="script.name"
            :value="script.source"
          />
        </el-select>
        <el-input
          v-model="customSendText"
          size="small"
          placeholder="输入要发送的命令内容"
          style="width:200px"
          @keyup.enter="handleQuickSend"
        />
        <el-button type="primary" size="small" :disabled="!customSendText" @click="handleQuickSend">发送</el-button>
      </div>

      <div class="terminal-config">
        <span class="config-label">保留行数</span>
        <el-select
          v-model="scrollback"
          class="scrollback-select"
          size="small"
          title="终端保留行数"
          @change="applyScrollback"
        >
          <el-option v-for="option in scrollbackOptions" :key="option" :label="`${option} 行`" :value="option" />
        </el-select>
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
import { SearchAddon } from 'xterm-addon-search'
import 'xterm/css/xterm.css'
import { getSessions, getScripts } from '../api'

const route = useRoute()
const terminalContainer = ref(null)
const searchInputRef = ref(null)
const searchQuery = ref('')
const caseSensitive = ref(false)
const scrollbackStorageKey = 'hubterm.sharedTerminal.scrollback'
const defaultScrollback = 10000
const scrollbackOptions = [1000, 5000, 10000, 50000]
const scrollback = ref(loadScrollback())
const connected = ref(false)
const sessionDisplayName = ref('')

// Quick Send state
const selectedScriptSource = ref('')
const customSendText = ref('')
const scripts = ref([])

let term
let fitAddon
let searchAddon
let ws

async function fetchSessionInfo() {
  try {
    const response = await getSessions(route.params.nodeId)
    const currentSession = response.data.find(s => s.session_id === route.params.sessionId)
    if (currentSession) {
      sessionDisplayName.value = currentSession.display_name || currentSession.port_name || ''
    }
  } catch (error) {
    console.error('Failed to fetch session info:', error)
  }
}

async function fetchScripts() {
  try {
    const res = await getScripts()
    scripts.value = res.data
  } catch (error) {
    console.error('Failed to fetch scripts:', error)
  }
}

function handleScriptChange(val) {
  if (val) {
    customSendText.value = val
  }
}

function sendTextToTerminal(text) {
  const data = text + '\r'
  if (ws && ws.readyState === WebSocket.OPEN) {
    send('terminal_input', {
      node_id: route.params.nodeId,
      session_id: route.params.sessionId,
      data: bytesToBase64(data),
    })
  }
}

function handleQuickSend() {
  if (!customSendText.value) return
  sendTextToTerminal(customSendText.value)
  customSendText.value = ''
  selectedScriptSource.value = ''
  term?.focus()
}

function loadScrollback() {
  const saved = Number(localStorage.getItem(scrollbackStorageKey))
  return scrollbackOptions.includes(saved) ? saved : defaultScrollback
}

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

function applyScrollback() {
  localStorage.setItem(scrollbackStorageKey, String(scrollback.value))
  if (term) {
    term.options.scrollback = scrollback.value
  }
}

function searchOptions() {
  return { caseSensitive: caseSensitive.value }
}

function findNext() {
  if (!searchQuery.value) return
  searchAddon?.findNext(searchQuery.value, searchOptions())
}

function findPrevious() {
  if (!searchQuery.value) return
  searchAddon?.findPrevious(searchQuery.value, searchOptions())
}

function repeatSearch() {
  if (searchQuery.value) findNext()
}

function clearSearch() {
  searchQuery.value = ''
  term?.clearSelection()
  term?.focus()
}

function handleKeydown(event) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'f') {
    event.preventDefault()
    searchInputRef.value?.focus()
    searchInputRef.value?.select?.()
  }
  if (event.key === 'Escape' && searchQuery.value) {
    clearSearch()
  }
}

onMounted(() => {
  fetchSessionInfo()
  fetchScripts()
  term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: 'Menlo, Monaco, "Courier New", monospace',
    scrollback: scrollback.value,
    theme: { background: '#1e1e1e', foreground: '#d4d4d4' },
  })
  fitAddon = new FitAddon()
  searchAddon = new SearchAddon()
  term.loadAddon(fitAddon)
  term.loadAddon(searchAddon)
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
  window.addEventListener('keydown', handleKeydown)
  connect()
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  window.removeEventListener('keydown', handleKeydown)
  disconnect()
  term?.dispose()
})
</script>

<style scoped>
.terminal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}
.terminal-title-row,
.connection-actions,
.terminal-toolbar,
.terminal-actions,
.terminal-config {
  display: flex;
  align-items: center;
}
.terminal-title-row {
  min-width: 0;
}
.terminal-title {
  margin: 0 10px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.connection-actions {
  gap: 8px;
  flex-shrink: 0;
}
.terminal-toolbar {
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 10px;
  padding: 8px 10px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  background: var(--el-fill-color-lighter);
}
.terminal-actions,
.terminal-config {
  gap: 8px;
  flex-wrap: wrap;
}
.terminal-config {
  margin-left: auto;
}
.config-label {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  white-space: nowrap;
}
.terminal-search {
  width: 190px;
}
.scrollback-select {
  width: 112px;
}
.terminal-container {
  width: 100%;
  height: calc(100vh - 230px);
  min-height: 360px;
  background: #1e1e1e;
}
</style>
