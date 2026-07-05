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
        />
        <el-button size="small" @click="handleCheckUpdate" :loading="checkingUpdate">
          检查更新
        </el-button>
        <el-button size="small" @click="openExportDialog">导出</el-button>
        <el-upload
          ref="uploadRef"
          action=""
          :auto-upload="false"
          :show-file-list="false"
          accept=".json,.tar,.tar.gz,.tgz"
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
      width="680px"
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
        <el-form-item label="命令/脚本内容" prop="source">
          <div style="width:100%">
            <div style="display:flex;gap:8px;margin-bottom:8px;align-items:center">
              <span style="font-size:12px;color:var(--el-text-color-secondary)">从文件加载：</span>
              <el-upload
                action=""
                :auto-upload="false"
                :show-file-list="false"
                :on-change="handleSourceFile"
              >
                <el-button size="small" plain>选择文件</el-button>
              </el-upload>
              <span v-if="sourceFileName" style="font-size:12px;color:var(--el-color-success)">
                ✓ {{ sourceFileName }}
              </span>
            </div>
            <el-input
              v-model="form.source"
              type="textarea"
              :rows="10"
              placeholder="在此输入/粘贴多行命令或脚本内容，或从上方选择文件加载..."
              style="font-family:monospace;font-size:13px"
            />
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>

    <!-- Export Password Dialog -->
    <el-dialog v-model="exportDialogVisible" title="导出预设内容" width="420px" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="导出格式">
          <el-radio-group v-model="exportFormat">
            <el-radio-button label="json">JSON</el-radio-button>
            <el-radio-button label="tar.gz">tar.gz 资源包</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="导出密码 (留空则不加密)">
          <el-input
            v-model="exportPassword"
            type="password"
            show-password
            placeholder="设置密码保护导出文件（可选）"
          />
        </el-form-item>
        <el-alert v-if="exportFormat !== 'json'" type="info" :closable="false" style="margin-top:8px">
          tar.gz 资源包会把 manifest.json、脚本文件和版本信息一起导出；设置密码后文件名会自动带 enc。
        </el-alert>
        <el-alert v-if="exportPassword" type="warning" :closable="false" style="margin-top:8px">
          导入时需要输入相同的密码才能解密。请妥善保管密码。
        </el-alert>
      </el-form>
      <template #footer>
        <el-button @click="exportDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="exporting" @click="confirmExport">确认导出</el-button>
      </template>
    </el-dialog>

    <!-- Import Preview Dialog -->
    <el-dialog v-model="importDialogVisible" title="导入预设内容" width="700px" destroy-on-close>
      <!-- Password prompt for encrypted bundles -->
      <div v-if="importNeedsPassword">
        <el-alert type="warning" :closable="false" style="margin-bottom:12px">
          此文件已加密，请输入导出时设置的密码。
        </el-alert>
        <el-input
          v-model="importPassword"
          type="password"
          show-password
          placeholder="输入解密密码"
          @keyup.enter="decryptImportBundle"
        />
        <div style="margin-top:12px;text-align:right">
          <el-button @click="importDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="decrypting" @click="decryptImportBundle">解密</el-button>
        </div>
      </div>

      <!-- Preview after decryption or plain bundle -->
      <div v-if="!importNeedsPassword">
        <div style="margin-bottom:12px">
          <el-alert type="info" :closable="false">
            共 <strong>{{ importPreview.length }}</strong> 条预设内容。
          </el-alert>
        </div>
        <el-table :data="importPreview" max-height="300" size="small">
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
        <div style="margin-top:16px;text-align:right">
          <el-button @click="importDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="importing" @click="confirmImport">确认导入</el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getScripts, createScript, updateScript, deleteScript, exportScripts, importScripts, importScriptsFile } from '../api'

const PRESETS_GITHUB_URL = 'https://raw.githubusercontent.com/coolleng2525/hubterm/main/presets/default.json'

const scripts = ref([])
const searchText = ref('')
const dialogVisible = ref(false)
const editingId = ref(null)
const submitting = ref(false)
const formRef = ref(null)
const sourceFileName = ref('')
const checkingUpdate = ref(false)

// Export state
const exportDialogVisible = ref(false)
const exportPassword = ref('')
const exportFormat = ref('tar.gz')
const exporting = ref(false)

// Import state
const importDialogVisible = ref(false)
const importPreview = ref([])
const importBundle = ref(null)
const importRawBundle = ref(null)  // encrypted raw bundle
const importRawFile = ref(null)
const importOverwrite = ref(false)
const importing = ref(false)
const importNeedsPassword = ref(false)
const importPassword = ref('')
const decrypting = ref(false)

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
    ElMessage.error('获取列表失败')
  }
}

function openDialog(row = null) {
  sourceFileName.value = ''
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

// Load source from file
function handleSourceFile(file) {
  const reader = new FileReader()
  reader.onload = (e) => {
    form.source = e.target.result
    sourceFileName.value = file.name
  }
  reader.readAsText(file.raw)
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      const payload = { name: form.name, description: form.description, language: form.language, source: form.source, timeout: 30 }
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
    await ElMessageBox.confirm(`确定要删除预设内容 "${row.name}" 吗？`, '提示', { type: 'warning', confirmButtonText: '确定', cancelButtonText: '取消' })
    await deleteScript(row.script_id || row.id)
    ElMessage.success('删除成功')
    fetchScripts()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('删除失败')
  }
}

// ── Crypto helpers (Web Crypto API, no deps) ──────────────────────────────────
function b64encode(buf) {
  const bytes = new Uint8Array(buf)
  let binary = ''
  const chunkSize = 0x8000
  for (let i = 0; i < bytes.length; i += chunkSize) {
    binary += String.fromCharCode.apply(null, bytes.subarray(i, i + chunkSize))
  }
  return btoa(binary)
}
function b64decode(str) {
  const binary = atob(str)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}

async function deriveKey(password, salt) {
  const enc = new TextEncoder()
  const km = await crypto.subtle.importKey('raw', enc.encode(password), 'PBKDF2', false, ['deriveKey'])
  return crypto.subtle.deriveKey(
    { name: 'PBKDF2', salt, iterations: 120000, hash: 'SHA-256' },
    km,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt']
  )
}

async function encryptBundle(data, password) {
  const salt = crypto.getRandomValues(new Uint8Array(16))
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const key = await deriveKey(password, salt)
  const enc = new TextEncoder()
  const encrypted = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, enc.encode(JSON.stringify(data)))
  return { encrypted: true, version: '1.0', salt: b64encode(salt), iv: b64encode(iv), data: b64encode(encrypted) }
}

async function decryptBundle(bundle, password) {
  const salt = b64decode(bundle.salt)
  const iv = b64decode(bundle.iv)
  const ciphertext = b64decode(bundle.data)
  const key = await deriveKey(password, salt)
  const plain = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, ciphertext)
  return JSON.parse(new TextDecoder().decode(plain))
}
// ─────────────────────────────────────────────────────────────────────────────

// Export
function openExportDialog() {
  exportPassword.value = ''
  exportFormat.value = 'tar.gz'
  exportDialogVisible.value = true
}

async function confirmExport() {
  exporting.value = true
  try {
    if (exportFormat.value !== 'json') {
      const res = await exportScripts(exportFormat.value, exportPassword.value)
      const blob = new Blob([res.data], { type: exportFormat.value === 'tar' ? 'application/x-tar' : 'application/gzip' })
      const suffix = exportPassword.value ? '-enc' : ''
      downloadBlob(blob, `hubterm-presets-${new Date().toISOString().slice(0, 10)}${suffix}.${exportFormat.value}`)
      exportDialogVisible.value = false
      ElMessage.success(exportPassword.value ? '导出成功（已加密）' : '导出成功')
      return
    }

    const res = await exportScripts('json')
    let payload = res.data
    if (exportPassword.value) {
      payload = await encryptBundle(res.data, exportPassword.value)
    }

    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' })
    const suffix = exportPassword.value ? '-enc' : ''
    downloadBlob(blob, `hubterm-presets-${new Date().toISOString().slice(0, 10)}${suffix}.json`)
    exportDialogVisible.value = false
    ElMessage.success(exportPassword.value ? '导出成功（已加密）' : '导出成功')
  } catch (e) {
    ElMessage.error('导出失败')
  } finally {
    exporting.value = false
  }
}

function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

// Import from file
function handleImportFile(file) {
  importBundle.value = null
  importRawBundle.value = null
  importRawFile.value = null
  importPreview.value = []
  importNeedsPassword.value = false
  importPassword.value = ''
  const filename = (file.name || '').toLowerCase()
  if (filename.endsWith('.tar') || filename.endsWith('.tar.gz') || filename.endsWith('.tgz')) {
    importTarPackage(file.raw)
    return
  }
  const reader = new FileReader()
  reader.onload = (e) => {
    try {
      const bundle = JSON.parse(e.target.result)
      if (bundle.encrypted) {
        // Encrypted bundle — need password
        importRawBundle.value = bundle
        importRawFile.value = null
        importBundle.value = null
        importPreview.value = []
        importNeedsPassword.value = true
        importPassword.value = ''
        importDialogVisible.value = true
      } else if (bundle.scripts && Array.isArray(bundle.scripts)) {
        // Plain bundle
        importBundle.value = bundle
        importPreview.value = bundle.scripts
        importNeedsPassword.value = false
        importDialogVisible.value = true
      } else {
        ElMessage.error('文件格式不正确，缺少 scripts 数组')
      }
    } catch {
      ElMessage.error('文件解析失败，请确认是有效的 JSON 格式')
    }
  }
  reader.readAsText(file.raw)
}

async function importTarPackage(file) {
  importing.value = true
  try {
    const res = await importScriptsFile(file)
    const { imported, updated, skipped } = res.data
    ElMessage.success(`导入完成：新增 ${imported} 条，更新 ${updated} 条，跳过 ${skipped || 0} 条`)
    fetchScripts()
  } catch (e) {
    const msg = e.response?.data?.error || ''
    if (msg.includes('requires password')) {
      importRawFile.value = file
      importRawBundle.value = null
      importPreview.value = []
      importNeedsPassword.value = true
      importPassword.value = ''
      importDialogVisible.value = true
    } else {
      ElMessage.error(msg || '导入失败')
    }
  } finally {
    importing.value = false
  }
}

async function decryptImportBundle() {
  if (!importPassword.value) {
    ElMessage.warning('请输入密码')
    return
  }
  decrypting.value = true
  try {
    if (importRawFile.value) {
      const res = await importScriptsFile(importRawFile.value, importPassword.value)
      const { imported, updated, skipped } = res.data
      ElMessage.success(`导入完成：新增 ${imported} 条，更新 ${updated} 条，跳过 ${skipped || 0} 条`)
      importDialogVisible.value = false
      importRawFile.value = null
      fetchScripts()
      return
    }
    const decrypted = await decryptBundle(importRawBundle.value, importPassword.value)
    if (!decrypted.scripts) throw new Error('invalid')
    importBundle.value = decrypted
    importPreview.value = decrypted.scripts
    importNeedsPassword.value = false
  } catch {
    ElMessage.error('密码错误或文件已损坏')
  } finally {
    decrypting.value = false
  }
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
    importNeedsPassword.value = false
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
