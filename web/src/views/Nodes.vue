<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center">
      <h2 style="margin-top:0">节点管理</h2>
      <div>
        <el-select v-model="statusFilter" placeholder="状态筛选" clearable style="width:120px" @change="fetchNodes">
          <el-option label="全部" value="" />
          <el-option label="在线" value="online" />
          <el-option label="离线" value="offline" />
        </el-select>
      </div>
    </div>

    <el-table :data="nodes" stripe style="width:100%" @row-click="goDetail">
      <el-table-column prop="name" label="名称" min-width="120" />
      <el-table-column prop="node_id" label="节点ID" width="200">
        <template #default="{ row }">
          <code style="font-size:12px">{{ row.node_id?.substring(0, 8) }}...</code>
        </template>
      </el-table-column>
      <el-table-column prop="ip" label="IP" width="140" />
      <el-table-column prop="hostname" label="主机名" width="140" />
      <el-table-column prop="os" label="系统" width="80" />
      <el-table-column label="来源" width="100">
        <template #default="{ row }">
          <el-tag :type="row.source === 'tabby' ? 'warning' : 'info'" size="small">
            {{ row.source === 'tabby' ? 'Tabby' : 'Agent' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status === 'online' ? 'success' : 'info'" size="small">
            {{ row.status === 'online' ? '在线' : '离线' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="CPU/内存" width="130">
        <template #default="{ row }">
          <div style="font-size:12px">
            CPU: {{ row.cpu_percent?.toFixed(1) }}%<br/>
            内存: {{ row.memory_percent?.toFixed(1) }}%
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="last_seen" label="最后上报" width="170">
        <template #default="{ row }">
          {{ formatTime(row.last_seen) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button type="primary" link size="small" @click.stop="goDetail(row)">详情</el-button>
          <el-button type="danger" link size="small" @click.stop="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getNodes, deleteNode } from '../api'

const router = useRouter()
const nodes = ref([])
const statusFilter = ref('')

async function fetchNodes() {
  try {
    const res = await getNodes(statusFilter.value || undefined)
    nodes.value = res.data
  } catch (e) {
    console.error(e)
  }
}

async function handleDelete(node) {
  try {
    await ElMessageBox.confirm(
      `确定要删除节点 "${node.name || node.hostname}" (ID: ${node.node_id}) 及其关联的所有配置和会话吗？`,
      '警告',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    await deleteNode(node.node_id)
    ElMessage.success('节点已成功删除')
    fetchNodes()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || '删除失败')
    }
  }
}

function formatTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

function goDetail(row) {
  router.push(`/nodes/${row.node_id}`)
}

onMounted(fetchNodes)
</script>
