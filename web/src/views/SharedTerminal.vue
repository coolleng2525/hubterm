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
        <el-select
          v-model="scrollback"
          class="scrollback-select"
          size="small"
          title="终端保留行数"
          @change="applyScrollback"
        >
          <el-option v-for="option in scrollbackOptions" :key="option" :label="`${option} 行`" :value="option" />
        </el-select>
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
import { SearchAddon } from 'xterm-addon-search'
import 'xterm/css/xterm.css'

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
let term
let fitAddon
let searchAddon
let ws

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
.terminal-title {
  margin: 0 10px;
  font-weight: 600;
}
.terminal-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}
.terminal-search {
  width: 190px;
}
.scrollback-select {
  width: 112px;
}
.terminal-container {
  width: 100%;
  height: calc(100vh - 180px);
  min-height: 360px;
  background: #1e1e1e;
}
</style>
