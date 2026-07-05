<template>
  <div>
    <div class="page-header">
      <div>
        <el-button link @click="$router.back()">← 返回</el-button>
        <span class="page-title">SSH 终端</span>
        <el-tag v-if="node" size="small" type="info">{{ node.ip }}</el-tag>
      </div>
      <div class="header-actions">
        <el-button size="small" @click="settingsVisible = !settingsVisible">
          {{ settingsVisible ? '隐藏连接设置' : '显示连接设置' }}
        </el-button>
        <el-tag :type="connected ? 'success' : connecting ? 'warning' : 'info'">
          {{ connected ? '已连接' : connecting ? '连接中' : '未连接' }}
        </el-tag>
      </div>
    </div>

    <el-card v-show="settingsVisible" class="connection-card" shadow="never">
      <el-form :model="form" label-position="top" @submit.prevent="connect">
        <div class="profile-row">
          <el-select v-model="selectedProfileId" clearable placeholder="选择已保存配置" @change="selectProfile">
            <el-option
              v-for="profile in profiles"
              :key="profile.id"
              :label="`${profile.name} · ${profile.username}@${profile.host}:${profile.port}`"
              :value="profile.id"
            />
          </el-select>
          <el-input v-model.trim="form.name" placeholder="配置名称，例如：开发服务器" />
          <el-button :loading="saving" @click="saveProfile">保存配置</el-button>
          <el-button v-if="selectedProfileId" type="danger" plain @click="removeProfile">删除</el-button>
        </div>
        <div class="form-grid">
          <el-form-item label="主机">
            <el-input :model-value="node?.ip || '加载中…'" disabled />
          </el-form-item>
          <el-form-item label="端口">
            <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" />
          </el-form-item>
          <el-form-item label="用户名">
            <el-input v-model.trim="form.username" autocomplete="username" placeholder="例如 lleng" />
          </el-form-item>
          <el-form-item label="认证方式">
            <el-radio-group v-model="form.authType">
              <el-radio-button label="password">密码</el-radio-button>
              <el-radio-button label="key">私钥</el-radio-button>
            </el-radio-group>
          </el-form-item>
        </div>

        <el-form-item v-if="form.authType === 'password'" label="密码">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            autocomplete="current-password"
            placeholder="SSH 登录密码"
            @keyup.enter="connect"
          />
        </el-form-item>

        <template v-else>
          <el-form-item label="私钥">
            <el-input
              v-model="form.privateKey"
              type="textarea"
              :rows="5"
              autocomplete="off"
              placeholder="粘贴 OpenSSH 或 PEM 私钥内容"
            />
          </el-form-item>
          <el-form-item label="私钥口令（可选）">
            <el-input
              v-model="form.passphrase"
              type="password"
              show-password
              autocomplete="off"
              placeholder="没有口令可留空"
              @keyup.enter="connect"
            />
          </el-form-item>
        </template>

        <div class="form-actions">
          <span class="error-text">{{ errorMessage }}</span>
          <div>
            <el-button v-if="connected || connecting" @click="disconnect">断开</el-button>
            <el-button type="primary" :loading="connecting" :disabled="connected" native-type="submit">
              连接
            </el-button>
          </div>
        </div>
      </el-form>
    </el-card>

    <div class="terminal-toolbar" v-if="connected" style="display:flex;align-items:center;gap:8px;margin-bottom:10px;padding:8px 10px;border:1px solid var(--el-border-color-light);border-radius:6px;background:var(--el-fill-color-lighter)">
      <span style="font-size: 12px; color: var(--el-text-color-secondary); white-space: nowrap;">快速发送</span>
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
        type="textarea"
        :autosize="{ minRows: 1, maxRows: 4 }"
        size="small"
        placeholder="输入/粘贴命令内容 (回车发送，Shift+回车换行)"
        style="width:250px"
        @keydown.enter.exact.prevent="handleQuickSend"
      />
      <el-button type="primary" size="small" :disabled="!customSendText" @click="handleQuickSend">发送</el-button>
    </div>

    <div ref="terminalContainer" class="terminal-container"></div>
  </div>
</template>

<script setup>
import { reactive, ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'
import { ElMessage } from 'element-plus'
import { getNode, getSSHProfiles, createSSHProfile, updateSSHProfile, deleteSSHProfile, getScripts } from '../api'

const route = useRoute()
const terminalContainer = ref(null)
const node = ref(null)
const connecting = ref(false)
const connected = ref(false)
const settingsVisible = ref(true)

// Quick Send state
const selectedScriptSource = ref('')
const customSendText = ref('')
const scripts = ref([])
const selectedScript = ref(null)

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
    const found = scripts.value.find(s => s.source === val)
    selectedScript.value = found || null
    customSendText.value = val
  } else {
    selectedScript.value = null
  }
}

async function sendTextToTerminal(text, language = 'shell') {
  if (language === 'python') {
    // Send python/scripts as a single block immediately
    const data = text + '\r'
    if (ws && ws.readyState === WebSocket.OPEN && connected.value) {
      ws.send(JSON.stringify({ type: 2, content: data }))
    }
    return
  }

  // Otherwise (e.g. text/shell), send line-by-line with 100ms delay
  const lines = text.split(/\r?\n/)
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    const data = line + '\r'
    if (ws && ws.readyState === WebSocket.OPEN && connected.value) {
      ws.send(JSON.stringify({ type: 2, content: data }))
    }
    if (i < lines.length - 1) {
      await new Promise(resolve => setTimeout(resolve, 100))
    }
  }
}

async function handleQuickSend() {
  if (!customSendText.value) return
  const text = customSendText.value
  const lang = selectedScript.value ? selectedScript.value.language : 'shell'
  customSendText.value = ''
  selectedScriptSource.value = ''
  selectedScript.value = null
  await sendTextToTerminal(text, lang)
  term?.focus()
}
const profiles = ref([])
const selectedProfileId = ref(null)
const saving = ref(false)
const errorMessage = ref('')
const form = reactive({
	name: '',
  port: 22,
  username: 'lleng',
  authType: 'password',
  password: '',
  privateKey: '',
  passphrase: '',
})

let term = null
let fitAddon = null
let ws = null
let manualDisconnect = false

function validateForm() {
  if (!node.value) return '节点信息尚未加载'
  if (!form.username) return '请输入用户名'
  const selected = profiles.value.find((profile) => profile.id === selectedProfileId.value)
  if (form.authType === 'password' && !form.password && !selected?.has_password) return '请输入密码'
  if (form.authType === 'key' && !form.privateKey.trim() && !selected?.has_private_key) return '请粘贴私钥'
  return ''
}

async function loadProfiles() {
  profiles.value = (await getSSHProfiles(route.params.nodeId)).data
}

function selectProfile(id) {
  const profile = profiles.value.find((item) => item.id === id)
  if (!profile) return
  form.name = profile.name
  form.port = profile.port
  form.username = profile.username
  form.authType = profile.auth_type
  form.password = ''
  form.privateKey = ''
  form.passphrase = ''
}

function profilePayload() {
  return {
    name: form.name,
    node_id: route.params.nodeId,
    host: node.value?.ip || '',
    port: form.port,
    username: form.username,
    auth_type: form.authType,
    password: form.password,
    private_key: form.privateKey,
    passphrase: form.passphrase,
  }
}

async function saveProfile() {
  if (!form.name) { errorMessage.value = '请输入配置名称'; return }
  const validationError = validateForm()
  if (validationError) { errorMessage.value = validationError; return }
  saving.value = true
  try {
    const response = selectedProfileId.value
      ? await updateSSHProfile(selectedProfileId.value, profilePayload())
      : await createSSHProfile(profilePayload())
    await loadProfiles()
    selectedProfileId.value = response.data.id
    selectProfile(response.data.id)
    ElMessage.success('SSH 配置已保存')
  } catch (error) {
    errorMessage.value = error.response?.data?.error || '保存失败'
  } finally {
    saving.value = false
  }
}

async function removeProfile() {
  await deleteSSHProfile(selectedProfileId.value)
  selectedProfileId.value = null
  form.name = ''
  await loadProfiles()
  ElMessage.success('SSH 配置已删除')
}

function connect() {
  const validationError = validateForm()
  if (validationError) {
    errorMessage.value = validationError
    return
  }

  disconnect(false)
  const token = localStorage.getItem('token')
  if (!token) {
    errorMessage.value = '登录已失效，请重新登录 HubTerm'
    return
  }

  manualDisconnect = false
  connecting.value = true
  errorMessage.value = ''
  term.clear()
  term.writeln(`\x1b[36m正在连接 ${node.value.ip}:${form.port}…\x1b[0m`)

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  ws = new WebSocket(`${protocol}//${window.location.host}/api/v1/terminal/connect`, [
    'hubterm',
    `hubterm.auth.${token}`,
  ])

  ws.onopen = () => {
    const sessionId = crypto.randomUUID
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random().toString(16).slice(2)}`
    ws.send(JSON.stringify({
      session_id: sessionId,
      protocol: 'ssh',
      profile_id: selectedProfileId.value || 0,
      ip: node.value.ip,
      port: form.port,
      username: form.username,
      password: form.authType === 'password' ? form.password : '',
      private_key: form.authType === 'key' ? form.privateKey : '',
      passphrase: form.authType === 'key' ? form.passphrase : '',
      cols: term.cols,
      rows: term.rows,
      term_type: 'xterm-256color',
    }))
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 2) term.write(msg.content)
      if (msg.type === 1) {
        connecting.value = false
        connected.value = true
        term.writeln('\x1b[32mSSH 已连接\x1b[0m')
      }
      if (msg.type === 0) {
        connecting.value = false
        connected.value = false
        errorMessage.value = msg.content || '连接已断开'
        term.writeln(`\r\n\x1b[31m${errorMessage.value}\x1b[0m`)
      }
    } catch {
      term.write(event.data)
    }
  }

  ws.onclose = () => {
    connecting.value = false
    connected.value = false
    if (!manualDisconnect && !errorMessage.value) {
      errorMessage.value = '连接已关闭'
    }
  }

  ws.onerror = () => {
    connecting.value = false
    connected.value = false
    errorMessage.value = 'WebSocket 连接失败'
  }
}

function disconnect(showMessage = true) {
  manualDisconnect = true
  if (ws) {
    ws.close()
    ws = null
  }
  connecting.value = false
  connected.value = false
  if (showMessage && term) term.writeln('\r\n\x1b[33m连接已断开\x1b[0m')
}

function handleResize() {
  if (!fitAddon) return
  fitAddon.fit()
  if (ws && ws.readyState === WebSocket.OPEN && connected.value) {
    const size = btoa(JSON.stringify({ rows: term.rows, cols: term.cols }))
    ws.send(JSON.stringify({ type: 3, content: size }))
  }
}

onMounted(async () => {
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
  term.writeln('\x1b[90m填写上方 SSH 信息后点击“连接”\x1b[0m')
  term.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN && connected.value) {
      ws.send(JSON.stringify({ type: 2, content: data }))
    }
  })

  try {
    node.value = (await getNode(route.params.nodeId)).data.node
    await loadProfiles()
    await fetchScripts()
  } catch {
    errorMessage.value = '无法加载节点信息'
  }
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  disconnect(false)
  term?.dispose()
})
</script>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.page-title {
  margin: 0 10px;
  font-size: 18px;
  font-weight: 600;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.connection-card {
  margin-bottom: 12px;
}

.profile-row {
  display: grid;
  grid-template-columns: minmax(260px, 2fr) minmax(180px, 1fr) auto auto;
  gap: 10px;
  align-items: center;
  margin-bottom: 14px;
}

.form-grid {
  display: grid;
  grid-template-columns: minmax(180px, 2fr) 130px minmax(160px, 1fr) auto;
  gap: 12px;
}

.form-grid :deep(.el-form-item) {
  margin-bottom: 12px;
}

.form-grid :deep(.el-input-number) {
  width: 100%;
}

.form-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.error-text {
  color: var(--el-color-danger);
  font-size: 13px;
}

.terminal-container {
  width: 100%;
  height: calc(100vh - 390px);
  min-height: 320px;
  padding: 8px;
  box-sizing: border-box;
  background: #1e1e1e;
  border-radius: 6px;
}

@media (max-width: 900px) {
  .profile-row,
  .form-grid {
    grid-template-columns: 1fr 1fr;
  }

  .terminal-container {
    height: 420px;
  }
}
</style>
