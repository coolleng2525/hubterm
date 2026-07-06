<template>
  <el-container style="height:100vh">
    <el-aside width="200px" style="background:#304156">
      <div style="height:60px;display:flex;align-items:center;justify-content:center;color:#fff;font-size:18px;font-weight:bold;border-bottom:1px solid rgba(255,255,255,0.1)">
        HubTerm
      </div>
      <el-menu :default-active="route.path" router background-color="#304156" text-color="#bfcbd9" active-text-color="#409eff">
        <el-menu-item index="/dashboard">
          <el-icon><Monitor /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>
        <el-menu-item index="/nodes">
          <el-icon><Connection /></el-icon>
          <span>节点管理</span>
        </el-menu-item>
        <el-menu-item index="/sessions">
          <el-icon><UserFilled /></el-icon>
          <span>会话管理</span>
        </el-menu-item>
        <el-menu-item index="/audit-logs">
          <el-icon><Document /></el-icon>
          <span>审计日志</span>
        </el-menu-item>
        <el-menu-item index="/scripts">
          <el-icon><Promotion /></el-icon>
          <span>发送管理</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header style="background:#fff;border-bottom:1px solid #e6e6e6;display:flex;align-items:center;justify-content:flex-end;padding:0 20px">
        <el-dropdown @command="handleCommand">
          <span style="cursor:pointer;display:flex;align-items:center;gap:5px">
            {{ user?.username || '用户' }}
            <el-icon><ArrowDown /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="profile">个人信息</el-dropdown-item>
              <el-dropdown-item command="mcp-token">MCP Token</el-dropdown-item>
              <el-dropdown-item command="logout">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>
      <el-main style="background:#f0f2f5">
        <router-view />
      </el-main>

      <el-dialog v-model="mcpDialogVisible" title="MCP Token" width="680px">
        <div class="mcp-token-panel">
          <div class="mcp-row">
            <span class="mcp-label">有效期</span>
            <el-input-number v-model="mcpDays" :min="1" :max="3650" :step="30" controls-position="right" />
            <span class="mcp-hint">天</span>
          </div>
          <div class="mcp-actions">
            <el-button type="primary" :loading="generatingToken" @click="handleGenerateMCPToken">生成 Token</el-button>
            <el-button :disabled="!mcpToken" @click="copyText(mcpConfig)">复制配置</el-button>
            <el-button :disabled="!mcpToken" @click="copyText(mcpToken)">复制 Token</el-button>
          </div>
          <div v-if="mcpToken" class="mcp-result">
            <div class="mcp-meta">过期时间：{{ mcpExpiresAt }}</div>
            <el-input v-model="mcpConfig" type="textarea" :rows="9" readonly />
          </div>
        </div>
      </el-dialog>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { generateMCPToken } from '../api'

const route = useRoute()
const router = useRouter()
const user = ref(JSON.parse(localStorage.getItem('user') || '{}'))
const mcpDialogVisible = ref(false)
const mcpDays = ref(365)
const mcpToken = ref('')
const mcpExpiresAt = ref('')
const generatingToken = ref(false)

const mcpConfig = computed(() => JSON.stringify({
  mcpServers: {
    HubTerm: {
      url: `${window.location.origin}/api/mcp`,
      headers: {
        Authorization: `Bearer ${mcpToken.value}`,
      },
    },
  },
}, null, 2))

function handleCommand(cmd) {
  if (cmd === 'mcp-token') {
    mcpDialogVisible.value = true
    return
  }
  if (cmd === 'logout') {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    router.push('/login')
  }
}

async function handleGenerateMCPToken() {
  generatingToken.value = true
  try {
    const res = await generateMCPToken(mcpDays.value)
    mcpToken.value = res.data.token
    mcpExpiresAt.value = res.data.expires_at
    ElMessage.success('MCP Token 已生成')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '生成 MCP Token 失败')
  } finally {
    generatingToken.value = false
  }
}

async function copyText(text) {
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制')
  } catch {
    ElMessage.error('复制失败')
  }
}
</script>

<style scoped>
.mcp-token-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.mcp-row,
.mcp-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}
.mcp-label {
  font-weight: 600;
}
.mcp-hint,
.mcp-meta {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.mcp-result {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
</style>
