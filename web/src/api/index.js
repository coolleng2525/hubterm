import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 10000,
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response && err.response.status === 401) {
      // 登录请求的 401 不跳转，让 Login.vue 显示错误信息
      if (!err.config.url.includes('/auth/login')) {
        localStorage.removeItem('token')
        window.location.href = '/login'
      }
    }
    return Promise.reject(err)
  }
)

export default api

export function login(username, password) {
  return api.post('/auth/login', { username, password })
}

export function getProfile() {
  return api.get('/auth/profile')
}

export function generateMCPToken(days = 365) {
  return api.post('/auth/mcp-token', { days })
}

export function listMCPTokens() {
  return api.get('/auth/mcp-tokens')
}

export function revokeMCPToken(id) {
  return api.post(`/auth/mcp-tokens/${id}/revoke`)
}

export function getNodes(status) {
  const params = status ? { status } : {}
  return api.get('/nodes', { params })
}

export function getNode(id) {
  return api.get(`/nodes/${id}`)
}

export function getSerialPorts(nodeId) {
  const params = nodeId ? { node_id: nodeId } : {}
  return api.get('/serial-ports', { params })
}

export function getSSHProfiles(nodeId) {
  return api.get('/ssh-profiles', { params: nodeId ? { node_id: nodeId } : {} })
}

export function createSSHProfile(data) {
  return api.post('/ssh-profiles', data)
}

export function updateSSHProfile(id, data) {
  return api.put(`/ssh-profiles/${id}`, data)
}

export function deleteSSHProfile(id) {
  return api.delete(`/ssh-profiles/${id}`)
}

export function getSessions(nodeId, portName) {
  const params = {}
  if (nodeId) params.node_id = nodeId
  if (portName) params.port_name = portName
  return api.get('/sessions', { params })
}

export function kickSession(id) {
  return api.post(`/sessions/${id}/kick`)
}

export function assignMaster(id) {
  return api.post(`/sessions/${id}/assign-master`)
}

export function renameSession(id, displayName) {
  return api.put(`/sessions/${id}/rename`, { display_name: displayName })
}

export function getAuditLogs(params) {
  return api.get('/audit-logs', { params })
}

export function getAuditActions() {
  return api.get('/audit-logs/actions')
}

export function sendCommand(nodeId, command, params) {
  return api.post(`/nodes/${nodeId}/command`, { command, params })
}

export function startLocalShell(nodeId, shell, rows = 24, cols = 100) {
  return api.post(`/nodes/${nodeId}/shell`, { shell, rows, cols })
}

export function startAgentSSH(nodeId, data) {
  return api.post(`/nodes/${nodeId}/ssh`, data)
}

export function deleteNode(id) {
  return api.delete(`/nodes/${id}`)
}

export function getScripts() {
  return api.get('/scripts')
}

export function createScript(data) {
  return api.post('/scripts', data)
}

export function updateScript(id, data) {
  return api.put(`/scripts/${id}`, data)
}

export function deleteScript(id) {
  return api.delete(`/scripts/${id}`)
}

export function exportScripts(format = 'json', password = '') {
  if (format === 'tar.gz' || format === 'tar' || format === 'tgz') {
    const headers = password ? { 'X-HubTerm-Export-Password': password } : undefined
    return api.get('/scripts/export', { params: { format }, responseType: 'blob', headers })
  }
  return api.get('/scripts/export')
}

export function importScripts(bundle) {
  return api.post('/scripts/import', bundle)
}

export function importScriptsFile(file, password = '') {
  const form = new FormData()
  form.append('file', file)
  if (password) form.append('password', password)
  return api.post('/scripts/import', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}
