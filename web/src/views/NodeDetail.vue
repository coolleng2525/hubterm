<template>
  <div>
    <el-button link style="margin-bottom:10px" @click="$router.push('/nodes')">← 返回节点列表</el-button>

    <el-card v-if="node" style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <div>
            <span>{{ node.name || node.hostname }} ({{ node.ip }})</span>
            <el-tag :type="node.status === 'online' ? 'success' : 'info'" size="small" style="margin-left:10px">
              {{ node.status === 'online' ? '在线' : '离线' }}
            </el-tag>
          </div>
          <el-button type="primary" :disabled="node.status !== 'online'" @click="openSSH">
            SSH 连接
          </el-button>
        </div>
      </template>
      <el-descriptions :column="3" border size="small">
        <el-descriptions-item label="节点ID"><code>{{ node.node_id }}</code></el-descriptions-item>
        <el-descriptions-item label="来源">
          <el-tag :type="node.source === 'tabby' ? 'warning' : 'info'" size="small">
            {{ node.source === 'tabby' ? 'Tabby 插件' : 'Agent' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="主机名">{{ node.hostname }}</el-descriptions-item>
        <el-descriptions-item label="系统">{{ node.os }} {{ node.os_version }}</el-descriptions-item>
        <el-descriptions-item label="架构">{{ node.arch }}</el-descriptions-item>
        <el-descriptions-item label="CPU">{{ formatPercent(node.cpu_percent) }}</el-descriptions-item>
        <el-descriptions-item label="内存">{{ formatMemory(node) }}</el-descriptions-item>
        <el-descriptions-item label="磁盘">{{ formatDisk(node) }}</el-descriptions-item>
        <el-descriptions-item label="最后上报">{{ formatTime(node.last_seen) }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card v-if="node" style="margin-bottom:20px">
      <template #header><span>连接方式</span></template>
      <div style="display:flex;align-items:center;gap:12px;flex-wrap:wrap">
        <el-button type="primary" :disabled="node.status !== 'online'" @click="openSSH">
          SSH 终端
        </el-button>
        <template v-if="shells.length">
          <el-select v-model="selectedShell" style="width:190px">
            <el-option v-for="shell in shells" :key="shell.id" :label="shell.name" :value="shell.id" />
          </el-select>
          <el-button type="success" :disabled="node.status !== 'online'" @click="openLocalShell">
            本机终端
          </el-button>
        </template>
        <span v-if="node.os === 'linux'" style="color:var(--el-text-color-secondary);font-size:13px">
          连接 {{ node.ip }}:22
        </span>
      </div>
      <template v-if="node.source !== 'tabby'">
        <el-divider />
        <el-form :inline="true" :model="sshForm" size="small" style="margin-bottom:-18px">
          <el-form-item label="备注">
            <el-input v-model.trim="sshForm.display_name" placeholder="例如：openclaw" style="width:160px" />
          </el-form-item>
          <el-form-item label="Host">
            <el-input v-model.trim="sshForm.host" placeholder="192.168.1.55" style="width:150px" />
          </el-form-item>
          <el-form-item label="端口">
            <el-input-number v-model="sshForm.port" :min="1" :max="65535" controls-position="right" style="width:105px" />
          </el-form-item>
          <el-form-item label="用户">
            <el-input v-model.trim="sshForm.username" placeholder="root" style="width:110px" />
          </el-form-item>
          <el-form-item label="密码">
            <el-input v-model="sshForm.password" type="password" show-password placeholder="可留空" style="width:140px" />
          </el-form-item>
          <el-form-item>
            <el-button type="success" :disabled="node.status !== 'online'" @click="openAgentSSH">
              Agent SSH
            </el-button>
          </el-form-item>
        </el-form>
      </template>
    </el-card>

    <el-card v-if="node" style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>{{ node.source === 'tabby' ? 'Tabby SSH/终端会话' : 'Agent SSH 配置' }}</span>
          <el-button v-if="node.source !== 'tabby'" type="primary" size="small" @click="showAddProfileDialog">添加配置</el-button>
        </div>
      </template>
      <el-table v-if="node.source !== 'tabby'" :data="profiles" stripe style="width:100%">
        <el-table-column prop="name" label="配置名称" min-width="150" />
        <el-table-column label="连接地址" width="240">
          <template #default="{ row }">
            <code>{{ row.username }}@{{ row.host }}:{{ row.port }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="auth_type" label="认证方式" width="100">
          <template #default="{ row }">
            <el-tag :type="row.auth_type === 'key' ? 'warning' : 'info'" size="small">
              {{ row.auth_type === 'key' ? '私钥' : '密码' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button type="success" link size="small" :disabled="node.status !== 'online'" @click="connectAgentSSH(row)">
              连接
            </el-button>
            <el-button type="success" link size="small" :disabled="node.status !== 'online'" @click="connectAgentSSHInNewTab(row)">
              新标签页连接
            </el-button>
            <el-button type="primary" link size="small" @click="showEditProfileDialog(row)">
              编辑
            </el-button>
            <el-button type="danger" link size="small" @click="removeProfile(row.id)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-table v-else :data="sessions" stripe style="width:100%">
        <el-table-column prop="session_id" label="会话ID" width="160">
          <template #default="{ row }">
            <code style="font-size:12px">{{ row.session_id?.substring(0, 8) }}...</code>
          </template>
        </el-table-column>
        <el-table-column label="备注" min-width="180">
          <template #default="{ row }">
            <span>{{ sessionLabel(row) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="port_name" label="端口" min-width="160" />
        <el-table-column prop="user" label="用户" width="100" />
        <el-table-column prop="connected_at" label="连接时间" width="170">
          <template #default="{ row }">
            {{ formatTime(row.connected_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button type="success" link size="small" @click="openSharedTerminal(row)">共享终端</el-button>
            <el-button type="success" link size="small" @click="openSharedTerminalInNewTab(row)">新标签页连接</el-button>
            <el-button type="primary" link size="small" @click="handleRename(row)">重命名</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card style="margin-bottom:20px">
      <template #header>
        <span>串口列表</span>
      </template>
      <el-table :data="ports" stripe style="width:100%">
        <el-table-column prop="port_name" label="端口" width="160" />
        <el-table-column prop="description" label="描述" min-width="150" />
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status === 'online' ? 'success' : row.status === 'busy' ? 'warning' : 'info'" size="small">
              {{ row.status === 'online' ? '空闲' : row.status === 'busy' ? '占用' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="baud_rate" label="波特率" width="80" />
      </el-table>
    </el-card>

    <el-card>
      <template #header>
        <span>在线会话</span>
      </template>
      <el-table :data="sessions" stripe style="width:100%">
        <el-table-column prop="session_id" label="会话ID" width="200">
          <template #default="{ row }">
            <code style="font-size:12px">{{ row.session_id?.substring(0, 8) }}...</code>
          </template>
        </el-table-column>
        <el-table-column label="备注" min-width="150">
          <template #default="{ row }">
            <span>{{ sessionLabel(row) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="port_name" label="端口" width="120" />
        <el-table-column prop="user" label="用户" width="100" />
        <el-table-column label="类型" width="80">
          <template #default="{ row }">
            <el-tag :type="row.type === 'master' ? 'danger' : 'info'" size="small">
              {{ row.type === 'master' ? '主控' : '观察' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="client_ip" label="客户端IP" width="140" />
        <el-table-column prop="connected_at" label="连接时间" width="170">
          <template #default="{ row }">
            {{ formatTime(row.connected_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="360" fixed="right">
          <template #default="{ row }">
            <el-button type="success" link size="small" @click="openSharedTerminal(row)">共享终端</el-button>
            <el-button type="success" link size="small" @click="openSharedTerminalInNewTab(row)">新标签页连接</el-button>
            <el-button type="primary" link size="small" @click="handleRename(row)">重命名</el-button>
            <el-button type="primary" link size="small" @click="handleAssignMaster(row)">设为主控</el-button>
            <el-button type="danger" link size="small" @click="handleKick(row)">踢掉</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 添加/编辑配置对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingProfileId ? '编辑 Agent SSH 配置' : '添加 Agent SSH 配置'"
      width="500px"
      destroy-on-close
    >
      <el-form :model="dialogForm" label-width="90px" size="small">
        <el-form-item label="配置名称">
          <el-input v-model.trim="dialogForm.name" placeholder="例如：交换机 A" />
        </el-form-item>
        <el-form-item label="主机 (Host)">
          <el-input v-model.trim="dialogForm.host" placeholder="192.168.1.55" />
        </el-form-item>
        <el-form-item label="端口">
          <el-input-number v-model="dialogForm.port" :min="1" :max="65535" controls-position="right" style="width:120px" />
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model.trim="dialogForm.username" placeholder="root" />
        </el-form-item>
        <el-form-item label="认证方式">
          <el-radio-group v-model="dialogForm.authType">
            <el-radio-button value="password">密码</el-radio-button>
            <el-radio-button value="key">私钥</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="dialogForm.authType === 'password'" label="密码">
          <el-input v-model="dialogForm.password" type="password" show-password placeholder="SSH 密码" />
        </el-form-item>
        <template v-else>
          <el-form-item label="私钥">
            <el-input
              v-model="dialogForm.privateKey"
              type="textarea"
              :rows="4"
              placeholder="粘贴 OpenSSH 或 PEM 私钥内容"
            />
          </el-form-item>
          <el-form-item label="私钥口令">
            <el-input v-model="dialogForm.passphrase" type="password" show-password placeholder="没有可留空" />
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button size="small" @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" size="small" :loading="savingProfile" @click="saveProfile">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getNode, kickSession, assignMaster, renameSession, startLocalShell, startAgentSSH, getSSHProfiles, createSSHProfile, updateSSHProfile, deleteSSHProfile } from '../api'

const route = useRoute()
const router = useRouter()
const node = ref(null)
const ports = ref([])
const sessions = ref([])
const selectedShell = ref('')
const profiles = ref([])
const sshForm = ref({
  display_name: '',
  host: '',
  port: 22,
  username: 'root',
  password: '',
})
const savingProfile = ref(false)
const dialogVisible = ref(false)
const editingProfileId = ref(null)
const dialogForm = ref({
  name: '',
  host: '',
  port: 22,
  username: 'root',
  authType: 'password',
  password: '',
  privateKey: '',
  passphrase: '',
})

const shells = computed(() => {
  try {
    const parsed = JSON.parse(node.value?.shells || '[]')
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
})

async function loadProfiles() {
  try {
    profiles.value = (await getSSHProfiles(route.params.id)).data
  } catch (e) {
    console.error(e)
  }
}

async function fetchNode() {
  try {
    const res = await getNode(route.params.id)
    node.value = res.data.node
    ports.value = res.data.ports
    sessions.value = res.data.sessions
    if (!selectedShell.value && shells.value.length) selectedShell.value = shells.value[0].id
    if (!sshForm.value.host) sshForm.value.host = node.value?.ip || ''
    await loadProfiles()
  } catch (e) {
    console.error(e)
  }
}

function formatTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

function hasNumber(value) {
  return value !== null && value !== undefined && Number.isFinite(Number(value))
}

function formatPercent(value) {
  if (!hasNumber(value)) return '-'
  return `${Number(value).toFixed(1)}%`
}

function formatBytes(value) {
  if (!hasNumber(value) || Number(value) <= 0) return '-'
  return `${(Number(value) / 1024 / 1024 / 1024).toFixed(1)}G`
}

function formatMemory(currentNode) {
  const used = formatBytes(currentNode?.memory_used)
  const total = formatBytes(currentNode?.memory_total)
  const percent = formatPercent(currentNode?.memory_percent)
  if (used === '-' && total === '-') return '-'
  return `${used} / ${total} (${percent})`
}

function formatDisk(currentNode) {
  const used = formatBytes(currentNode?.disk_used)
  const total = formatBytes(currentNode?.disk_total)
  if (used === '-' && total === '-') return '-'
  return `${used} / ${total}`
}

function openSSH() {
  router.push(`/terminal/${node.value.node_id}`)
}

async function openLocalShell() {
  try {
    const response = await startLocalShell(node.value.node_id, selectedShell.value)
    router.push(`/shared-terminal/${node.value.node_id}/${response.data.session_id}`)
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '无法启动本机终端')
  }
}

async function openAgentSSH() {
  if (!sshForm.value.host) {
    ElMessage.warning('请输入 Host')
    return
  }
  if (!sshForm.value.username) {
    ElMessage.warning('请输入用户')
    return
  }
  try {
    const response = await startAgentSSH(node.value.node_id, {
      display_name: sshForm.value.display_name || `${sshForm.value.username}@${sshForm.value.host}:${sshForm.value.port}`,
      host: sshForm.value.host,
      port: sshForm.value.port,
      username: sshForm.value.username,
      password: sshForm.value.password,
      rows: 24,
      cols: 100,
    })
    router.push(`/shared-terminal/${node.value.node_id}/${response.data.session_id}`)
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '无法启动 Agent SSH')
  }
}

function showAddProfileDialog() {
  editingProfileId.value = null
  dialogForm.value = {
    name: '',
    host: node.value?.ip || '',
    port: 22,
    username: 'root',
    authType: 'password',
    password: '',
    privateKey: '',
    passphrase: '',
  }
  dialogVisible.value = true
}

function showEditProfileDialog(profile) {
  editingProfileId.value = profile.id
  dialogForm.value = {
    name: profile.name,
    host: profile.host,
    port: profile.port,
    username: profile.username,
    authType: profile.auth_type,
    password: '',
    privateKey: '',
    passphrase: '',
  }
  dialogVisible.value = true
}

async function saveProfile() {
  if (!dialogForm.value.name) {
    ElMessage.warning('请输入配置名称')
    return
  }
  if (!dialogForm.value.host) {
    ElMessage.warning('请输入Host')
    return
  }
  if (!dialogForm.value.username) {
    ElMessage.warning('请输入用户名')
    return
  }
  savingProfile.value = true
  try {
    const payload = {
      name: dialogForm.value.name,
      node_id: route.params.id,
      host: dialogForm.value.host,
      port: dialogForm.value.port,
      username: dialogForm.value.username,
      auth_type: dialogForm.value.authType,
      password: dialogForm.value.password,
      private_key: dialogForm.value.privateKey,
      passphrase: dialogForm.value.passphrase,
    }
    if (editingProfileId.value) {
      await updateSSHProfile(editingProfileId.value, payload)
    } else {
      await createSSHProfile(payload)
    }
    await loadProfiles()
    dialogVisible.value = false
    ElMessage.success('SSH 配置已保存')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '保存失败')
  } finally {
    savingProfile.value = false
  }
}

async function removeProfile(id) {
  try {
    await ElMessageBox.confirm('确定要删除这个配置吗？', '提示', { type: 'warning' })
    await deleteSSHProfile(id)
    await loadProfiles()
    ElMessage.success('SSH 配置已删除')
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

async function connectAgentSSH(profile) {
  try {
    const response = await startAgentSSH(node.value.node_id, {
      profile_id: profile.id,
      display_name: profile.name,
      host: profile.host,
      port: profile.port,
      username: profile.username,
      rows: 24,
      cols: 100,
    })
    router.push(`/shared-terminal/${node.value.node_id}/${response.data.session_id}`)
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '无法启动 Agent SSH')
  }
}

async function connectAgentSSHInNewTab(profile) {
  try {
    const response = await startAgentSSH(node.value.node_id, {
      profile_id: profile.id,
      display_name: profile.name,
      host: profile.host,
      port: profile.port,
      username: profile.username,
      rows: 24,
      cols: 100,
    })
    const routeData = router.resolve(`/shared-terminal/${node.value.node_id}/${response.data.session_id}`)
    window.open(routeData.href, '_blank')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '无法启动 Agent SSH')
  }
}

function openSharedTerminal(session) {
  router.push(`/shared-terminal/${node.value.node_id}/${session.session_id}`)
}

function openSharedTerminalInNewTab(session) {
  const routeData = router.resolve(`/shared-terminal/${node.value.node_id}/${session.session_id}`)
  window.open(routeData.href, '_blank')
}

function sessionLabel(session) {
  return session.display_name || `${session.port_name || '会话'} · ${session.user || '未知用户'}`
}

async function handleRename(session) {
  try {
    const { value } = await ElMessageBox.prompt('给这个会话设置一个容易识别的备注', '重命名会话', {
      confirmButtonText: '保存',
      cancelButtonText: '取消',
      inputValue: session.display_name || '',
      inputPlaceholder: '例如：客户A交换机',
      inputValidator: value => (value || '').trim().length <= 128 || '备注不能超过 128 个字符',
    })
    await renameSession(session.id || session.session_id, value || '')
    ElMessage.success('会话备注已更新')
    fetchNode()
  } catch (e) {
    if (e !== 'cancel' && e !== 'close') {
      ElMessage.error('重命名失败')
    }
  }
}

async function handleKick(session) {
  try {
    await kickSession(session.id || session.session_id)
    ElMessage.success('已踢掉会话')
    fetchNode()
  } catch (e) {
    ElMessage.error('操作失败')
  }
}

async function handleAssignMaster(session) {
  try {
    await assignMaster(session.id || session.session_id)
    ElMessage.success('已设为主控')
    fetchNode()
  } catch (e) {
    ElMessage.error('操作失败')
  }
}

onMounted(fetchNode)
</script>
