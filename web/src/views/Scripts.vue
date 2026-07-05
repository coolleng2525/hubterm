<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:20px">
      <h2 style="margin:0">发送管理 (预设命令与脚本)</h2>
      <div style="display:flex;gap:8px;align-items:center">
        <el-input
          v-model="searchText"
          placeholder="搜索名称/备注..."
          size="small"
          clearable
          style="width:200px"
          prefix-icon="Search"
        />
        <el-button size="small" @click="handleCheckUpdate" :loading="checkingUpdate">
          检查更新
        </el-button>
        <el-button size="small" @click="handleExport">导出</el-button>
        <el-upload
          ref="uploadRef"
          action=""
          :auto-upload="false"
          :show-file-list="false"
          accept=".json"
          :on-change="handleImportFile"
        >
          <el-button size="small">导入</el-button>
        </el-upload>
        <el-button type="primary" @click="openDialog()">新增预设</el-button>
      </div>
    </div>

    <el-card shadow="never">
      <el-table :data="filteredScripts" stripe style="width:100%">
        <el-table-column prop="name" label="名称" width="220" />
        <el-table-column prop="description" label="说明/备注" min-width="200" />
        <el-table-column prop="language" label="类型" width="150">
          <template #default="{ row }">
            <el-tag :type="row.language === 'python' ? 'success' : row.language === 'shell' ? 'warning' : 'primary'" size="small">
              {{ row.language === 'python' ? 'Python 脚本' : row.language === 'shell' ? 'Shell 脚本' : '文本' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="最后更新" width="180">
          <template #default="{ row }">
            {{ formatTime(row.updated_at || row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="openDialog(row)">编辑</el-button>
            <el-button type="danger" link size="small" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Edit/Create Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑预设内容' : '新增预设内容'"
      width="620px"
      destroy-on-close
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-position="top">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="输入易于识别的名称，如：查看系统负载" />
        </el-form-item>
        <el-form-item label="说明/备注" prop="description">
          <el-input v-model="form.description" placeholder="说明该命令/脚本的用途" />
        </el-form-item>
        <el-form-item label="类型" prop="language">
          <el-radio-group v-model="form.language">
            <el-radio-button label="text">文本 (按行发送)</el-radio-button>
            <el-radio-button label="shell">Shell 脚本 (整块发送)</el-radio-button>
            <el-radio-button label="python">Python 脚本 (整块发送)</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="命令/脚本内容 (支持多行)" prop="source">
          <el-input
            v-model="form.source"
            type="textarea"
            :rows="8"
            placeholder="在此输入需要发送的多行命令或脚本内容..."
            style="font-family:monospace"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>

    <!-- Import Preview Dialog -->
    <el-dialog v-model="importDialogVisible" title="导入预设内容" width="680px" destroy-on-close>
      <div style="margin-bottom:12px">
        <el-alert type="info" :closable="false">
          共 <strong>{{ importPreview.length }}</strong> 条预设内容。已存在同名条目将被跳过（或覆盖，取决于选项）。
        </el-alert>
      </div>
      <el-table :data="importPreview" max-height="320" size="small">
        <el-table-column prop="name" label="名称" width="200" />
        <el-table-column prop="description" label="说明" min-width="160" />
        <el-table-column prop="language" label="类型" width="120">
          <template #default="{ row }">
            <el-tag size="small" :type="row.language === 'python' ? 'success' : row.language === 'shell' ? 'warning' : 'primary'">
              {{ row.language || 'text' }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:12px;display:flex;align-items:center;gap:8px">
        <el-checkbox v-model="importOverwrite">覆盖同名已有项</el-checkbox>
      </div>
      <template #footer>
        <el-button @click="importDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="importing" @click="confirmImport">确认导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getScripts, createScript, updateScript, deleteScript, exportScripts, importScripts } from '../api'

const PRESETS_GITHUB_URL = 'https://raw.githubusercontent.com/coolleng2525/hubterm/main/presets/default.json'

const scripts = ref([])
const searchText = ref('')
const dialogVisible = ref(false)
const editingId = ref(null)
const submitting = ref(false)
const formRef = ref(null)
const checkingUpdate = ref(false)

// Import state
const importDialogVisible = ref(false)
const importPreview = ref([])
const importBundle = ref(null)
const importOverwrite = ref(false)
const importing = ref(false)

const filteredScripts = computed(() => {
  const q = searchText.value.trim().toLowerCase()
  if (!q) return scripts.value
  return scripts.value.filter(s =>
    (s.name || '').toLowerCase().includes(q) ||
    (s.description || '').toLowerCase().includes(q)
  )
})

const form = reactive({
  name: '',
  description: '',
  language: 'text',
  source: '',
})

const rules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  source: [{ required: true, message: '请输入命令/脚本内容', trigger: 'blur' }],
}

async function fetchScripts() {
  try {
    const res = await getScripts()
    scripts.value = res.data
  } catch (e) {
    console.error(e)
    ElMessage.error('获取列表失败')
  }
}

function openDialog(row = null) {
  if (row) {
    editingId.value = row.script_id || row.id
    form.name = row.name
    form.description = row.description || ''
    form.language = row.language || 'text'
    form.source = row.source
  } else {
    editingId.value = null
    form.name = ''
    form.description = ''
    form.language = 'text'
    form.source = ''
  }
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      const payload = {
        name: form.name,
        description: form.description,
        language: form.language,
        source: form.source,
        timeout: 30,
      }
      if (editingId.value) {
        await updateScript(editingId.value, payload)
        ElMessage.success('更新成功')
      } else {
        await createScript(payload)
        ElMessage.success('创建成功')
      }
      dialogVisible.value = false
      fetchScripts()
    } catch (e) {
      ElMessage.error(e.response?.data?.error || '操作失败')
    } finally {
      submitting.value = false
    }
  })
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确定要删除预设内容 "${row.name}" 吗？`, '提示', {
      type: 'warning',
      confirmButtonText: '确定',
      cancelButtonText: '取消',
    })
    await deleteScript(row.script_id || row.id)
    ElMessage.success('删除成功')
    fetchScripts()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// Export
async function handleExport() {
  try {
    const res = await exportScripts()
    const blob = new Blob([JSON.stringify(res.data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `hubterm-presets-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch (e) {
    ElMessage.error('导出失败')
  }
}

// Import from file
function handleImportFile(file) {
  const reader = new FileReader()
  reader.onload = (e) => {
    try {
      const bundle = JSON.parse(e.target.result)
      if (!bundle.scripts || !Array.isArray(bundle.scripts)) {
        ElMessage.error('文件格式不正确，缺少 scripts 数组')
        return
      }
      importBundle.value = bundle
      importPreview.value = bundle.scripts
      importDialogVisible.value = true
    } catch {
      ElMessage.error('文件解析失败，请确认是有效的 JSON 格式')
    }
  }
  reader.readAsText(file.raw)
}

async function confirmImport() {
  if (!importBundle.value) return
  importing.value = true
  try {
    const bundle = { ...importBundle.value, overwrite: importOverwrite.value }
    const res = await importScripts(bundle)
    const { imported, updated } = res.data
    ElMessage.success(`导入完成：新增 ${imported} 条，更新 ${updated} 条`)
    importDialogVisible.value = false
    fetchScripts()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '导入失败')
  } finally {
    importing.value = false
  }
}

// Check for updates from GitHub presets/default.json
async function handleCheckUpdate() {
  checkingUpdate.value = true
  try {
    const resp = await fetch(PRESETS_GITHUB_URL)
    if (!resp.ok) throw new Error('fetch failed')
    const bundle = await resp.json()
    if (!bundle.scripts) throw new Error('invalid format')

    const localNames = new Set(scripts.value.map(s => s.name))
    const newScripts = bundle.scripts.filter(s => !localNames.has(s.name))

    if (newScripts.length === 0) {
      ElMessage.success('已是最新，没有新的预设可以更新')
      return
    }

    importBundle.value = { ...bundle, scripts: newScripts }
    importPreview.value = newScripts
    importOverwrite.value = false
    importDialogVisible.value = true
    ElMessage.info(`发现 ${newScripts.length} 条新预设，请确认导入`)
  } catch {
    ElMessage.error('获取更新失败，请检查网络连接')
  } finally {
    checkingUpdate.value = false
  }
}

function formatTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

onMounted(fetchScripts)
</script>

<style scoped>
pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
}
</style>
