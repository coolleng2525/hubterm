<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center">
      <h2 style="margin-top:0">会话管理</h2>
      <div style="display:flex;gap:10px">
        <el-input v-model="nodeFilter" placeholder="节点ID" clearable style="width:200px" @clear="fetchSessions" @keyup.enter="fetchSessions" />
        <el-input v-model="portFilter" placeholder="端口名" clearable style="width:150px" @clear="fetchSessions" @keyup.enter="fetchSessions" />
        <el-button type="primary" @click="fetchSessions">查询</el-button>
      </div>
    </div>

    <el-table :data="sessions" stripe style="width:100%;margin-top:15px">
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
      <el-table-column label="操作" width="280" fixed="right">
        <template #default="{ row }">
          <el-button type="success" link size="small" @click="openSharedTerminal(row)">共享终端</el-button>
          <el-button type="primary" link size="small" @click="handleRename(row)">重命名</el-button>
          <el-button type="primary" link size="small" @click="handleAssignMaster(row)">设为主控</el-button>
          <el-button type="danger" link size="small" @click="handleKick(row)">踢掉</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getSessions, kickSession, assignMaster, renameSession } from '../api'

const router = useRouter()
const sessions = ref([])
const nodeFilter = ref('')
const portFilter = ref('')

async function fetchSessions() {
  try {
    const res = await getSessions(nodeFilter.value || undefined, portFilter.value || undefined)
    sessions.value = res.data
  } catch (e) {
    console.error(e)
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

onMounted(fetchSessions)
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
