<template>
  <div>
    <el-tabs v-model="activeTab" @tab-change="handleTabChange" style="background: #fff; padding: 20px; border-radius: 8px; box-shadow: 0 2px 12px 0 rgba(0,0,0,0.05)">
      <el-tab-pane name="active">
        <template #label>
          <span style="font-size: 16px; font-weight: bold;">
            <el-icon style="margin-right: 4px; vertical-align: middle;"><Monitor /></el-icon>在线活动会话
          </span>
        </template>
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:15px">
          <h3 style="margin:0;font-weight: 500;">当前在线终端会话</h3>
          <div style="display:flex;gap:10px">
            <el-input v-model="sessionSearch" placeholder="搜索备注/配置/IP/用户" clearable style="width:250px" />
            <el-input v-model="nodeFilter" placeholder="节点ID" clearable style="width:200px" @clear="fetchSessions" @keyup.enter="fetchSessions" />
            <el-input v-model="portFilter" placeholder="端口名" clearable style="width:150px" @clear="fetchSessions" @keyup.enter="fetchSessions" />
            <el-button type="primary" @click="fetchSessions">查询</el-button>
          </div>
        </div>

        <el-table :data="filteredSessions" stripe style="width:100%">
          <el-table-column label="节点/IP" width="190" fixed>
            <template #default="{ row }">
              <div class="node-cell">
                <strong>{{ row.node_ip || '-' }}</strong>
                <span>{{ row.node_name || shortId(row.node_id, 12) }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="session_id" label="会话ID" width="170">
            <template #default="{ row }">
              <code style="font-size:12px">{{ shortId(row.session_id, 12) }}</code>
            </template>
          </el-table-column>
          <el-table-column label="备注/配置" min-width="150">
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
      </el-tab-pane>

      <el-tab-pane name="profiles">
        <template #label>
          <span style="font-size: 16px; font-weight: bold;">
            <el-icon style="margin-right: 4px; vertical-align: middle;"><Connection /></el-icon>SSH 终端列表
          </span>
        </template>
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:15px">
          <h3 style="margin:0;font-weight: 500;">所有配置的 SSH 终端</h3>
          <div style="display:flex;gap:10px">
            <el-input v-model="profileSearch" placeholder="搜索终端名称/主机" clearable style="width:250px" @clear="filterProfiles" @input="filterProfiles" />
          </div>
        </div>

        <el-table :data="filteredProfiles" stripe style="width:100%">
          <el-table-column prop="name" label="配置名称" min-width="150" fixed />
          <el-table-column label="归属节点" width="200">
            <template #default="{ row }">
              <div class="node-cell" v-if="nodesMap[row.node_id]">
                <strong>{{ nodesMap[row.node_id].ip || '-' }}</strong>
                <span>{{ nodesMap[row.node_id].name || nodesMap[row.node_id].hostname || shortId(row.node_id, 12) }}</span>
              </div>
              <span v-else>{{ shortId(row.node_id, 12) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="连接地址" width="180">
            <template #default="{ row }">
              <code>{{ row.host }}:{{ row.port }}</code>
            </template>
          </el-table-column>
          <el-table-column prop="username" label="用户名" width="120" />
          <el-table-column label="认证方式" width="120">
            <template #default="{ row }">
              <el-tag :type="row.auth_type === 'password' ? 'success' : 'warning'" size="small">
                {{ row.auth_type === 'password' ? '密码认证' : '密钥认证' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="终端类型" width="120">
            <template #default="{ row }">
              <el-tag :type="getNodeTypeTag(row.node_id)" size="small">
                {{ getNodeTypeName(row.node_id) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="220" fixed="right">
            <template #default="{ row }">
              <el-button type="success" link size="small" @click="connectAgentSSH(row)">连接</el-button>
              <el-button type="success" link size="small" @click="connectAgentSSHInNewTab(row)">新标签页连接</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Monitor, Connection } from '@element-plus/icons-vue'
import { getSessions, kickSession, assignMaster, renameSession, getSSHProfiles, getNodes, startAgentSSH } from '../api'

const router = useRouter()
const activeTab = ref('active')

// Active Sessions state
const sessions = ref([])
const nodeFilter = ref('')
const portFilter = ref('')
const sessionSearch = ref('')

const filteredSessions = computed(() => {
  const q = (sessionSearch.value || '').trim().toLowerCase()
  if (!q) return sessions.value
  return sessions.value.filter(s => {
    return (
      (s.node_ip || '').toLowerCase().includes(q) ||
      (s.node_name || '').toLowerCase().includes(q) ||
      (s.session_id || '').toLowerCase().includes(q) ||
      (s.display_name || '').toLowerCase().includes(q) ||
      (s.port_name || '').toLowerCase().includes(q) ||
      (s.user || '').toLowerCase().includes(q) ||
      (s.client_ip || '').toLowerCase().includes(q)
    )
  })
})

// SSH Profiles state
const allProfiles = ref([])
const filteredProfiles = ref([])
const profileSearch = ref('')
const nodesMap = ref({})

async function fetchSessions() {
  try {
    const res = await getSessions(nodeFilter.value || undefined, portFilter.value || undefined)
    sessions.value = res.data
  } catch (e) {
    console.error(e)
  }
}

async function fetchProfilesAndNodes() {
  try {
    // Fetch profiles
    const profRes = await getSSHProfiles()
    allProfiles.value = profRes.data
    filteredProfiles.value = profRes.data

    // Fetch nodes to map details and source types
    const nodesRes = await getNodes()
    const map = {}
    nodesRes.data.forEach(node => {
      map[node.node_id] = node
    })
    nodesMap.value = map
  } catch (e) {
    console.error(e)
  }
}

function filterProfiles() {
  const query = (profileSearch.value || '').trim().toLowerCase()
  if (!query) {
    filteredProfiles.value = allProfiles.value
    return
  }
  filteredProfiles.value = allProfiles.value.filter(p => {
    return p.name.toLowerCase().includes(query) || p.host.toLowerCase().includes(query)
  })
}

function getNodeTypeName(nodeId) {
  const node = nodesMap.value[nodeId]
  if (!node) return '未知'
  return node.source === 'tabby' ? '插件 SSH' : 'Agent SSH'
}

function getNodeTypeTag(nodeId) {
  const node = nodesMap.value[nodeId]
  if (!node) return 'info'
  return node.source === 'tabby' ? 'warning' : 'primary'
}

function handleTabChange(tab) {
  if (tab === 'active') {
    fetchSessions()
  } else if (tab === 'profiles') {
    fetchProfilesAndNodes()
  }
}

async function connectAgentSSH(profile) {
  const node = nodesMap.value[profile.node_id] || {}
  try {
    const response = await startAgentSSH(profile.node_id, {
      profile_id: profile.id,
      display_name: profile.name,
      host: profile.host,
      port: profile.port,
      username: profile.username,
      rows: 24,
      cols: 100,
    })
    router.push(`/shared-terminal/${profile.node_id}/${response.data.session_id}`)
  } catch (error) {
    ElMessage.error(error.response?.data?.error || (node.source === 'tabby' ? '无法启动插件 SSH' : '无法启动 Agent SSH'))
  }
}

async function connectAgentSSHInNewTab(profile) {
  const node = nodesMap.value[profile.node_id] || {}
  try {
    const response = await startAgentSSH(profile.node_id, {
      profile_id: profile.id,
      display_name: profile.name,
      host: profile.host,
      port: profile.port,
      username: profile.username,
      rows: 24,
      cols: 100,
    })
    const routeData = router.resolve(`/shared-terminal/${profile.node_id}/${response.data.session_id}`)
    window.open(routeData.href, '_blank')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || (node.source === 'tabby' ? '无法启动插件 SSH' : '无法启动 Agent SSH'))
  }
}

function formatTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

function shortId(value, length = 8) {
  return value ? `${value.substring(0, length)}...` : '-'
}

function sessionLabel(session) {
  return session.display_name || `${session.port_name || '会话'} · ${session.user || '未知用户'}`
}

function openSharedTerminal(session) {
  router.push(`/shared-terminal/${session.node_id}/${session.session_id}`)
}

function openSharedTerminalInNewTab(session) {
  const routeData = router.resolve(`/shared-terminal/${session.node_id}/${session.session_id}`)
  window.open(routeData.href, '_blank')
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
    fetchSessions()
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
    fetchSessions()
  } catch (e) {
    ElMessage.error('操作失败')
  }
}

async function handleAssignMaster(session) {
  try {
    await assignMaster(session.id || session.session_id)
    ElMessage.success('已设为主控')
    fetchSessions()
  } catch (e) {
    ElMessage.error('操作失败')
  }
}

onMounted(() => {
  fetchSessions()
  fetchProfilesAndNodes()
})
</script>

<style scoped>
.node-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.3;
}

.node-cell strong {
  color: var(--el-text-color-primary);
  font-size: 13px;
  font-weight: 700;
}

.node-cell span {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>
