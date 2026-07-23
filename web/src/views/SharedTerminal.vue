<template>
  <div>
    <div class="terminal-header">
      <div class="terminal-title-row">
        <el-button link @click="$router.back()">← 返回</el-button>
        <span class="terminal-title">共享终端 · {{ sessionDisplayName || route.params.sessionId }}</span>
        <el-tag :type="connected ? 'success' : 'info'" size="small">
          {{ connected ? '已连接' : '未连接' }}
        </el-tag>
        <el-tag :type="participantRole === 'master' ? 'danger' : 'info'" size="small">
          {{ participantRole === 'master' ? '主控' : '观察者' }}
        </el-tag>
        <span v-if="sessionPortName" class="serial-params">{{ sessionPortName }}<template v-if="serialParams"> · {{ serialParams }}</template></span>
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
        <el-autocomplete
          v-model="scriptSearchText"
          :fetch-suggestions="queryScripts"
          clearable
          size="small"
          placeholder="搜索预设名称..."
          style="width:200px"
          value-key="name"
          @select="handleScriptSelect"
          @clear="handleScriptClear"
        >
          <template #default="{ item }">
            <div style="display:flex;justify-content:space-between;align-items:center;gap:8px">
              <span>{{ item.name }}</span>
              <el-tag size="small" :type="item.language === 'python' ? 'success' : item.language === 'shell' ? 'warning' : 'primary'">
                {{ item.language === 'python' ? 'py' : item.language === 'shell' ? 'sh' : 'txt' }}
              </el-tag>
            </div>
          </template>
        </el-autocomplete>
        <el-button
          size="small"
          type="warning"
          link
          title="设为默认发送项 (再次点击取消默认)"
          style="padding: 0 4px;"
          @click="setAsDefaultScript"
        >
          <el-icon v-if="selectedScriptId && selectedScriptId === defaultScriptId"><StarFilled /></el-icon>
          <el-icon v-else><Star /></el-icon>
        </el-button>
        <el-input
          v-model="customSendText"
		  :disabled="!canInput"
          type="textarea"
          :autosize="{ minRows: 1, maxRows: 4 }"
          size="small"
          placeholder="输入/粘贴命令内容 (回车发送，Shift+回车换行)"
          style="width:250px"
          @keydown.enter.exact.prevent="handleQuickSend"
        />
        <el-button type="primary" size="small" :disabled="!canInput || (!customSendText && !selectedScript)" @click="handleQuickSend">发送</el-button>
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

    <div v-if="participants.length" class="participants-bar">
      <span class="config-label">在线参与者</span>
      <el-tag v-for="participant in participants" :key="participant.id" :type="participant.role === 'master' ? 'danger' : 'info'" size="small">
        {{ participant.username }} · {{ participant.role === 'master' ? '主控' : '观察' }}
      </el-tag>
      <template v-if="currentUserRole === 'admin'">
        <el-dropdown trigger="click">
          <el-button size="small">管理参与者</el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-for="participant in participants" :key="participant.id" :disabled="participant.id === participantId">
                <span>{{ participant.username }}</span>
                <el-button v-if="participant.role !== 'master'" type="primary" link size="small" @click.stop="assignParticipantMaster(participant.id)">设为主控</el-button>
                <el-button type="danger" link size="small" @click.stop="kickParticipant(participant.id)">踢出</el-button>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </template>
    </div>

    <div ref="terminalContainer" class="terminal-container"></div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import { SearchAddon } from 'xterm-addon-search'
import 'xterm/css/xterm.css'
import { getSessions, getScripts, getProfile, getNode } from '../api'
import { ElMessage } from 'element-plus'
import { Star, StarFilled } from '@element-plus/icons-vue'

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
const sessionPortName = ref('')
const serialParams = ref('')
const participantId = ref('')
const participantRole = ref('observer')
const participants = ref([])
const currentUserRole = ref('')
const canInput = computed(() => connected.value && participantRole.value === 'master')

// Quick Send state
const selectedScriptId = ref('')
const scriptSearchText = ref('')
const customSendText = ref('')
const scripts = ref([])
const selectedScript = ref(null)
const defaultScriptId = ref('')

function getDefaultScriptKey() {
  return `hubterm.defaultScriptId.${route.params.sessionId}`
}

let term
let fitAddon
let searchAddon
let ws

async function fetchSessionInfo() {
  try {
    const response = await getSessions(route.params.nodeId)
    const currentSession = response.data.find(s => s.session_id === route.params.sessionId)
    if (currentSession) {
      const customDisplayName = currentSession.display_name || ''
      sessionDisplayName.value = customDisplayName || currentSession.port_name || ''
      sessionPortName.value = currentSession.port_name || ''
	  if (currentSession.protocol === 'serial') {
		const nodeResponse = await getNode(route.params.nodeId)
		const port = nodeResponse.data.ports?.find(item => item.port_name === currentSession.port_name)
		if (port) {
		  if (!customDisplayName && port.alias) sessionDisplayName.value = port.alias
		  const parity = port.parity === 'odd' ? 'O' : port.parity === 'even' ? 'E' : 'N'
		  const flow = port.flow_control === 'rtscts' ? 'RTS/CTS' : '无流控'
		  serialParams.value = `${port.baud_rate} · ${port.data_bits}${parity}${port.stop_bits} · ${flow}`
		}
	  }
      return true
    }
    writeStatus('当前终端会话已不存在或不活跃，请重新创建终端并打开新的共享链接。', '33')
    return false
  } catch (error) {
    console.error('Failed to fetch session info:', error)
    writeStatus('无法加载终端会话信息，请重新登录后重试。', '31')
    return null
  }
}

async function fetchScripts() {
  try {
    const res = await getScripts()
    scripts.value = res.data
    // Auto-select default script on load
    const key = getDefaultScriptKey()
    defaultScriptId.value = localStorage.getItem(key) || ''
    if (defaultScriptId.value) {
      const found = scripts.value.find(s => (s.script_id || s.id) === defaultScriptId.value)
      if (found) {
        selectedScriptId.value = defaultScriptId.value
        selectedScript.value = found
        scriptSearchText.value = found.name
      }
    }
  } catch (error) {
    console.error('Failed to fetch scripts:', error)
  }
}

function setAsDefaultScript() {
  const key = getDefaultScriptKey()
  if (selectedScriptId.value && selectedScriptId.value !== defaultScriptId.value) {
    defaultScriptId.value = selectedScriptId.value
    localStorage.setItem(key, selectedScriptId.value)
    ElMessage.success(`已设为默认发送项`)
  } else {
    defaultScriptId.value = ''
    localStorage.removeItem(key)
    ElMessage.info('已取消默认发送项')
  }
}


// Autocomplete fuzzy query
function queryScripts(query, cb) {
  const q = query.trim().toLowerCase()
  const results = q
    ? scripts.value.filter(s =>
        s.name.toLowerCase().includes(q) || (s.description || '').toLowerCase().includes(q)
      )
    : scripts.value
  cb(results)
}

function handleScriptSelect(item) {
  selectedScript.value = item
  selectedScriptId.value = item.script_id || item.id
  scriptSearchText.value = item.name
}

function handleScriptClear() {
  selectedScript.value = null
  selectedScriptId.value = ''
  scriptSearchText.value = ''
}


async function sendTextToTerminal(text, language = 'shell') {
  const trimmed = text.trim()
  const hasShebang = trimmed.startsWith('#!')
  const isPython = language === 'python' || trimmed.startsWith('#!/usr/bin/env python') || trimmed.startsWith('#!/usr/bin/python')

  if (hasShebang || isPython) {
    const ext = isPython ? 'py' : 'sh'
    const tmpFile = `/tmp/hubterm_run_${Date.now()}.${ext}`
    const cleanText = text.replace(/\r/g, '')
    
    let runCmd = ''
    if (isPython) {
      runCmd = `cat << 'EOF' > ${tmpFile}\n${cleanText}\nEOF\npython3 ${tmpFile}\nrm -f ${tmpFile}\n`
    } else {
      runCmd = `cat << 'EOF' > ${tmpFile}\n${cleanText}\nEOF\nchmod +x ${tmpFile}\n${tmpFile}\nrm -f ${tmpFile}\n`
    }

    if (ws && ws.readyState === WebSocket.OPEN) {
      send('terminal_input', {
        node_id: route.params.nodeId,
        session_id: route.params.sessionId,
        data: bytesToBase64(runCmd),
      })
    }
    return
  }

  // Otherwise (e.g. text/shell), send line-by-line with 100ms delay
  const lines = text.split(/\r?\n/)
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    const data = line + '\r'
    if (ws && ws.readyState === WebSocket.OPEN) {
      send('terminal_input', {
        node_id: route.params.nodeId,
        session_id: route.params.sessionId,
        data: bytesToBase64(data),
      })
    }
    if (i < lines.length - 1) {
      await new Promise(resolve => setTimeout(resolve, 100))
    }
  }
}

async function handleQuickSend() {
  if (!canInput.value) {
    ElMessage.warning('当前为观察模式，只有主控可以发送输入')
    return
  }
  let text = ''
  let lang = 'shell'
  if (selectedScript.value) {
    text = selectedScript.value.source
    lang = selectedScript.value.language
  } else if (customSendText.value) {
    text = customSendText.value
  }

  if (!text) return

  customSendText.value = ''
  selectedScriptId.value = ''
  selectedScript.value = null

  await sendTextToTerminal(text, lang)
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

function writeStatus(message, color = '33') {
  term?.writeln(`\r\n\x1b[${color}m${message}\x1b[0m`)
}

function connect() {
  disconnect()
  const token = localStorage.getItem('token')
  if (!token) {
    writeStatus('当前浏览器未登录，请先打开 HubTerm 登录，再重新打开此共享终端链接。', '31')
    return
  }
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
    // Auto-send default script after terminal is ready
    if (selectedScript.value) {
      setTimeout(() => handleQuickSend(), 500)
    }
  }
  ws.onmessage = event => {
    try {
      const message = JSON.parse(event.data)
      if (message.type === 'error') {
        term.writeln(`\r\n\x1b[31m${message.data?.message || 'Request failed'}\x1b[0m`)
        return
      }
      if (message.type === 'terminal_subscribed') {
        participantId.value = message.data?.participant_id || ''
        participantRole.value = message.data?.role || 'observer'
        participants.value = message.data?.participants || []
        return
      }
      if (message.type === 'terminal_participants' && message.data?.session_id === route.params.sessionId) {
        participants.value = message.data?.participants || []
        const self = participants.value.find(item => item.id === participantId.value)
        participantRole.value = self?.role || 'observer'
        return
      }
      if (message.type === 'terminal_kicked') {
        writeStatus('你已被管理员移出该终端', '31')
        disconnect()
        return
      }
      const terminal = message.data?.terminal
      if (message.type === 'terminal_state' && terminal?.session_id === route.params.sessionId && terminal.status !== 'open') {
        writeStatus(terminal.error || '串口连接已关闭', '31')
        connected.value = false
        return
      }
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
    writeStatus('连接已断开', '31')
  }
  ws.onerror = () => writeStatus('连接失败', '31')
}

function disconnect() {
  if (ws) {
    ws.onclose = null
    ws.close()
    ws = null
  }
  connected.value = false
  participantRole.value = 'observer'
  participants.value = []
}

function assignParticipantMaster(id) {
  send('terminal_assign_master', { session_id: route.params.sessionId, participant_id: id })
}

function kickParticipant(id) {
  send('terminal_kick_participant', { session_id: route.params.sessionId, participant_id: id })
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

onMounted(async () => {
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
    if (!canInput.value) return
    send('terminal_input', {
      node_id: route.params.nodeId,
      session_id: route.params.sessionId,
      data: bytesToBase64(data),
    })
  })
  window.addEventListener('resize', handleResize)
  window.addEventListener('keydown', handleKeydown)
  await fetchSessionInfo()
  try {
    currentUserRole.value = (await getProfile()).data.role || ''
  } catch {
    currentUserRole.value = ''
  }
  fetchScripts()
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
.serial-params {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-left: 8px;
}
.participants-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 10px;
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
