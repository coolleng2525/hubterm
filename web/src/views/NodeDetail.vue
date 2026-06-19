<template>
  <div>
    <el-button link style="margin-bottom:10px" @click="$router.push('/nodes')">← 返回节点列表</el-button>

    <el-card v-if="node" style="margin-bottom:20px">
      <template #header>
        <span>{{ node.name || node.hostname }} ({{ node.ip }})</span>
        <el-tag :type="node.status === 'online' ? 'success' : 'info'" size="small" style="margin-left:10px">
          {{ node.status === 'online' ? '在线' : '离线' }}
        </el-tag>
      </template>
      <el-descriptions :column="3" border size="small">
        <el-descriptions-item label="节点ID"><code>{{ node.node_id }}</code></el-descriptions-item>
        <el-descriptions-item label="主机名">{{ node.hostname }}</el-descriptions-item>
        <el-descriptions-item label="系统">{{ node.os }} {{ node.os_version }}</el-descriptions-item>
        <el-descriptions-item label="架构">{{ node.arch }}</el-descriptions-item>
        <el-descriptions-item label="CPU">{{ node.cpu_percent?.toFixed(1) }}%</el-descriptions-item>
        <el-descriptions-item label="内存">{{ (node.memory_used / 1024 / 1024 / 1024).toFixed(1) }}G / {{ (node.memory_total / 1024 / 1024 / 1024).toFixed(1) }}G ({{ node.memory_percent?.toFixed(1) }}%)</el-descriptions-item>
        <el-descriptions-item label="磁盘">{{ (node.disk_used / 1024 / 1024 / 1024).toFixed(1) }}G / {{ (node.disk_total / 1024 / 1024 / 1024).toFixed(1) }}G</el-descriptions-item>
        <el-descriptions-item label="最后上报">{{ formatTime(node.last_seen) }}</el-descriptions-item>
      </el-descriptions>
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
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
			<el-button type="primary" link size="small" @click="openTerminal(row)">SSH终端</el-button>
          </template>
        </el-table-column>
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
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getNode, kickSession, assignMaster } from '../api'

const route = useRoute()
const router = useRouter()
const node = ref(null)
const ports = ref([])
const sessions = ref([])

async function fetchNode() {
  try {
    const res = await getNode(route.params.id)
    node.value = res.data.node
    ports.value = res.data.ports
    sessions.value = res.data.sessions
  } catch (e) {
    console.error(e)
  }
}

function formatTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

function openTerminal(port) {
  router.push(`/terminal/${node.value.node_id}/${port.port_name}`)
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
