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

export function getAuditLogs(params) {
  return api.get('/audit-logs', { params })
}

export function sendCommand(nodeId, command, params) {
  return api.post(`/nodes/${nodeId}/command`, { command, params })
}
