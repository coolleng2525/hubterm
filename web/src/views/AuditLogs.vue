<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center">
      <h2 style="margin-top:0">审计日志</h2>
      <div style="display:flex;gap:10px">
        <el-select v-model="actionFilter" placeholder="操作类型" clearable style="width:150px" @change="fetchLogs">
          <el-option label="全部" value="" />
          <el-option label="指令下发" value="command" />
          <el-option label="踢掉会话" value="kick_session" />
          <el-option label="指派主控" value="assign_master" />
        </el-select>
      </div>
    </div>

    <el-table :data="logs" stripe style="width:100%;margin-top:15px">
      <el-table-column prop="user" label="用户" width="100" />
      <el-table-column prop="action" label="操作" width="120">
        <template #default="{ row }">
          <el-tag size="small">{{ row.action }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="target" label="目标" width="200" />
      <el-table-column prop="detail" label="详情" min-width="200" />
      <el-table-column prop="ip" label="IP" width="140" />
      <el-table-column prop="created_at" label="时间" width="170">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>
    </el-table>

    <div style="display:flex;justify-content:center;margin-top:20px">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="fetchLogs"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getAuditLogs } from '../api'

const logs = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const actionFilter = ref('')

async function fetchLogs() {
  try {
    const params = { page: page.value, page_size: pageSize.value }
    if (actionFilter.value) params.action = actionFilter.value
    const res = await getAuditLogs(params)
    logs.value = res.data.logs
    total.value = res.data.total
  } catch (e) {
    console.error(e)
  }
}

function formatTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

onMounted(fetchLogs)
</script>
