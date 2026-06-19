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
      <el-table-column prop="session_id" label="会话ID" width="200">
        <template #default="{ row }">
          <code style="font-size:12px">{{ row.session_id?.substring(0, 12) }}...</code>
        </template>
      </el-table-column>
      <el-table-column prop="node_id" label="节点ID" width="200">
        <template #default="{ row }">
          <code style="font-size:12px">{{ row.node_id?.substring(0, 12) }}...</code>
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
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button type="primary" link size="small" @click="handleAssignMaster(row)">设为主控</el-button>
          <el-button type="danger" link size="small" @click="handleKick(row)">踢掉</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getSessions, kickSession, assignMaster } from '../api'

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
