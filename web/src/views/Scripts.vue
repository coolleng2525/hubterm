<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:20px">
      <h2 style="margin:0">发送管理 (预设命令与脚本)</h2>
      <el-button type="primary" @click="openDialog()">新增预设内容</el-button>
    </div>

    <el-card shadow="never">
      <el-table :data="scripts" stripe style="width:100%">
        <el-table-column prop="name" label="名称" width="200" font-weight="bold" />
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

    <!-- Script Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑预设内容' : '新增预设内容'"
      width="600px"
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
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getScripts, createScript, updateScript, deleteScript } from '../api'

const scripts = ref([])
const dialogVisible = ref(false)
const editingId = ref(null)
const submitting = ref(false)
const formRef = ref(null)

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
