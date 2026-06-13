<template>
  <div>
    <h2 style="margin-top:0">仪表盘</h2>
    <el-row :gutter="20">
      <el-col :span="6">
        <el-card shadow="hover">
          <div style="text-align:center">
            <div style="font-size:36px;color:#409eff">{{ stats.totalNodes }}</div>
            <div style="color:#999;font-size:13px;margin-top:8px">总节点数</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div style="text-align:center">
            <div style="font-size:36px;color:#67c23a">{{ stats.onlineNodes }}</div>
            <div style="color:#999;font-size:13px;margin-top:8px">在线节点</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div style="text-align:center">
            <div style="font-size:36px;color:#e6a23c">{{ stats.totalPorts }}</div>
            <div style="color:#999;font-size:13px;margin-top:8px">串口总数</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div style="text-align:center">
            <div style="font-size:36px;color:#f56c6c">{{ stats.activeSessions }}</div>
            <div style="color:#999;font-size:13px;margin-top:8px">活跃会话</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card style="margin-top:20px">
      <template #header>
        <span>节点列表</span>
      </template>
      <el-table :data="nodes" stripe style="width:100%" @row-click="goNodeDetail">
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column prop="ip" label="IP" width="140" />
        <el-table-column prop="hostname" label="主机名" width="140" />
        <el-table-column prop="os" label="系统" width="100" />
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status === 'online' ? 'success' : 'info'" size="small">
              {{ row.status === 'online' ? '在线' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="cpu_percent" label="CPU" width="80">
          <template #default="{ row }">
            {{ row.cpu_percent?.toFixed(1) }}%
          </template>
        </el-table-column>
        <el-table-column prop="memory_percent" label="内存" width="80">
          <template #default="{ row }">
            {{ row.memory_percent?.toFixed(1) }}%
          </template>
        </el-table-column>
        <el-table-column prop="last_seen" label="最后上报" width="170">
          <template #default="{ row }">
            {{ formatTime(row.last_seen) }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { getNodes } from '../api'

const router = useRouter()
const nodes = ref([])
const stats = ref({ totalNodes: 0, onlineNodes: 0, totalPorts: 0, activeSessions: 0 })

async function fetchData() {
  try {
    const res = await getNodes()
    nodes.value = res.data
    stats.value.totalNodes = res.data.length
    stats.value.onlineNodes = res.data.filter(n => n.status === 'online').length
  } catch (e) {
    console.error(e)
  }
}

function formatTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

function goNodeDetail(row) {
  router.push(`/nodes/${row.node_id}`)
}

onMounted(fetchData)
</script>
