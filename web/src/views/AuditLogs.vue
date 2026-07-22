<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center">
      <h2 style="margin-top:0">审计日志</h2>
      <div style="display:flex;gap:10px;align-items:center;flex-wrap:wrap;justify-content:flex-end">
        <el-input
          v-model.trim="searchText"
          placeholder="搜索用户/操作/目标/详情/IP"
          clearable
          style="width:260px"
          @keyup.enter="applyFilters"
          @clear="applyFilters"
        />
        <el-select v-model="actionFilter" placeholder="操作类型" clearable filterable style="width:180px" @change="applyFilters">
          <el-option label="全部" value="" />
          <el-option v-for="action in actionOptions" :key="action" :label="actionLabel(action)" :value="action" />
        </el-select>
        <el-button type="primary" @click="applyFilters">搜索</el-button>
        <el-button @click="resetFilters">重置</el-button>
      </div>
    </div>

    <el-table :data="logs" stripe style="width:100%;margin-top:15px">
      <el-table-column prop="user" label="用户" width="100" />
      <el-table-column prop="action" label="操作" width="120">
        <template #default="{ row }">
          <el-tag size="small">{{ actionLabel(row.action) }}</el-tag>
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
import { getAuditLogs, getAuditActions } from '../api'

const logs = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const actionFilter = ref('')
const searchText = ref('')
const actionOptions = ref([])

const actionLabels = {
  agent_log: 'Agent 日志',
  ai_exec: 'AI 执行',
  assign_master: '指派主控',
  batch_exec: '批量执行',
  command: '指令下发',
  delete_node: '删除节点',
  exec_command: '执行命令',
  generate_mcp_token: '生成 MCP Token',
  group_exec: '分组执行',
  kick_session: '踢掉会话',
  login: '登录',
  mcp_exec: 'MCP 执行',
  mcp_quick_send: 'MCP 快速发送',
  mcp_script: 'MCP 脚本',
  mcp_terminal_input: 'MCP 终端输入',
  proxy_connect: '代理连接',
  rename_session: '重命名会话',
  revoke_mcp_token: '吊销 MCP Token',
}

async function fetchLogs() {
  try {
    const params = { page: page.value, page_size: pageSize.value }
    if (actionFilter.value) params.action = actionFilter.value
    if (searchText.value) params.q = searchText.value
    const res = await getAuditLogs(params)
    logs.value = res.data.logs
    total.value = res.data.total
  } catch (e) {
    console.error(e)
  }
}

async function fetchActions() {
  try {
    const res = await getAuditActions()
    actionOptions.value = res.data.actions || []
  } catch (e) {
    console.error(e)
  }
}

function applyFilters() {
  page.value = 1
  fetchLogs()
}

function resetFilters() {
  actionFilter.value = ''
  searchText.value = ''
  applyFilters()
}

function actionLabel(action) {
  return actionLabels[action] || action
}

function formatTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

onMounted(() => {
  fetchActions()
  fetchLogs()
})
</script>
