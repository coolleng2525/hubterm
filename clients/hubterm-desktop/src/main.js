const { app, BrowserWindow, ipcMain } = require('electron')
const path = require('path')
const os = require('os')
const crypto = require('crypto')
const pty = require('node-pty')
const WebSocket = require('ws')

let Store
let store
let mainWindow
let ws = null
let reconnectTimer = null
let reportTimer = null
let reconnectDelay = 1000
const sessions = new Map()

async function loadStore () {
  if (!Store) {
    Store = (await import('electron-store')).default
  }
  if (!store) {
    store = new Store({
      name: 'hubterm-desktop',
      defaults: {
        centerUrl: 'ws://127.0.0.1:8097/api/ws/agent',
        nodeName: os.hostname() || 'HubTerm Desktop',
        nodeId: crypto.randomUUID(),
        token: '',
        reportInterval: 3,
      },
    })
  }
  return store
}

function normalizeAgentUrl (configuredUrl) {
  const url = new URL(configuredUrl)
  if (['', '/', '/ws', '/api/ws'].includes(url.pathname)) {
    url.pathname = '/api/ws/agent'
  }
  url.searchParams.set('node_id', store.get('nodeId'))
  return url.toString()
}

function reportUrl () {
  const url = new URL(store.get('centerUrl'))
  url.protocol = url.protocol === 'wss:' ? 'https:' : 'http:'
  url.pathname = '/api/nodes/report'
  url.search = ''
  return url.toString()
}

function buildReport () {
  const totalMemory = os.totalmem()
  const freeMemory = os.freemem()
  const usedMemory = Math.max(totalMemory - freeMemory, 0)
  return {
    node_id: store.get('nodeId'),
    source: 'agent',
    name: store.get('nodeName') || 'HubTerm Desktop',
    hostname: os.hostname(),
    os: process.platform,
    os_version: os.release(),
    arch: os.arch(),
    cpu_percent: 0,
    memory_total: totalMemory,
    memory_used: usedMemory,
    memory_percent: totalMemory > 0 ? usedMemory / totalMemory * 100 : 0,
    disk_total: 0,
    disk_used: 0,
    interfaces: Object.entries(os.networkInterfaces()).flatMap(([name, addrs]) =>
      (addrs || []).filter(addr => !addr.internal).map(addr => ({ name, ip: addr.address }))
    ),
    serial_ports: [],
    sessions: Array.from(sessions.values()).map(session => ({
      session_id: session.id,
      port_name: session.name,
      user: os.userInfo().username || '',
      type: 'master',
      client_ip: '',
      connected_at: Math.floor(session.connectedAt / 1000),
    })),
  }
}

async function registerIfNeeded () {
  if (store.get('token')) {
    return
  }
  const response = await fetch(reportUrl(), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(buildReport()),
  })
  if (!response.ok) {
    throw new Error(`registration failed: ${response.status} ${await response.text()}`)
  }
  const payload = await response.json()
  if (!payload.token) {
    throw new Error('registration did not return a node token')
  }
  store.set('token', payload.token)
}

async function connectAgent () {
  await loadStore()
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
    return
  }
  await registerIfNeeded()
  const protocols = ['hubterm', `hubterm.node.${store.get('token')}`]
  ws = new WebSocket(normalizeAgentUrl(store.get('centerUrl')), protocols)

  ws.on('open', () => {
    reconnectDelay = 1000
    sendReport()
    clearInterval(reportTimer)
    reportTimer = setInterval(sendReport, Math.max(1, Number(store.get('reportInterval')) || 3) * 1000)
    emitStatus()
  })

  ws.on('message', raw => handleAgentMessage(String(raw)))
  ws.on('close', scheduleReconnect)
  ws.on('error', error => {
    mainWindow?.webContents.send('agent:error', error.message)
    emitStatus()
  })
}

function scheduleReconnect () {
  clearInterval(reportTimer)
  reportTimer = null
  ws = null
  emitStatus()
  if (reconnectTimer) {
    return
  }
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    connectAgent().catch(error => {
      mainWindow?.webContents.send('agent:error', error.message)
      scheduleReconnect()
    })
  }, reconnectDelay)
  reconnectDelay = Math.min(reconnectDelay * 2, 30000)
}

function sendReport () {
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    return
  }
  ws.send(JSON.stringify({ type: 'report', data: buildReport() }))
}

function sendTerminalData (sessionId, direction, data) {
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    return
  }
  ws.send(JSON.stringify({
    type: 'terminal_data',
    data: {
      session_id: sessionId,
      direction,
      data: Buffer.from(data).toString('base64'),
    },
  }))
}

function handleAgentMessage (raw) {
  let message
  try {
    message = JSON.parse(raw)
  } catch {
    return
  }
  const payload = message.data?.payload || message.data || {}
  if (message.type === 'write' && payload.session_id && payload.data) {
    const session = sessions.get(payload.session_id)
    if (session) {
      session.pty.write(Buffer.from(payload.data, 'base64').toString())
    }
  }
  if ((message.type === 'disconnect' || message.type === 'kick_session') && payload.session_id) {
    closeSession(payload.session_id)
  }
}

function defaultShell () {
  if (process.platform === 'win32') {
    return process.env.COMSPEC || 'powershell.exe'
  }
  return process.env.SHELL || '/bin/zsh'
}

function createSession (options = {}) {
  const id = crypto.randomUUID()
  const shell = options.shell || defaultShell()
  const name = options.name || path.basename(shell)
  const instance = pty.spawn(shell, [], {
    name: 'xterm-256color',
    cols: options.cols || 100,
    rows: options.rows || 30,
    cwd: os.homedir(),
    env: process.env,
  })
  const session = { id, name, shell, pty: instance, connectedAt: Date.now() }
  sessions.set(id, session)

  instance.onData(data => {
    mainWindow?.webContents.send('terminal:data', { sessionId: id, data })
    sendTerminalData(id, 'output', data)
  })
  instance.onExit(() => {
    sessions.delete(id)
    mainWindow?.webContents.send('terminal:closed', { sessionId: id })
    sendReport()
  })
  sendReport()
  return { sessionId: id, name, shell }
}

function writeSession (sessionId, data) {
  const session = sessions.get(sessionId)
  if (!session) {
    return false
  }
  session.pty.write(data)
  sendTerminalData(sessionId, 'input', data)
  return true
}

function resizeSession (sessionId, cols, rows) {
  const session = sessions.get(sessionId)
  if (!session) {
    return false
  }
  session.pty.resize(cols, rows)
  return true
}

function closeSession (sessionId) {
  const session = sessions.get(sessionId)
  if (!session) {
    return false
  }
  session.pty.kill()
  sessions.delete(sessionId)
  sendReport()
  return true
}

function emitStatus () {
  mainWindow?.webContents.send('agent:status', {
    connected: Boolean(ws && ws.readyState === WebSocket.OPEN),
    nodeId: store?.get('nodeId'),
    nodeName: store?.get('nodeName'),
    centerUrl: store?.get('centerUrl'),
    sessionCount: sessions.size,
  })
}

async function createWindow () {
  await loadStore()
  mainWindow = new BrowserWindow({
    width: 1180,
    height: 760,
    minWidth: 900,
    minHeight: 560,
    title: 'HubTerm Desktop',
    backgroundColor: '#101216',
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
  })
  await mainWindow.loadFile(path.join(__dirname, 'renderer', 'index.html'))
  emitStatus()
}

ipcMain.handle('config:get', async () => {
  await loadStore()
  return {
    centerUrl: store.get('centerUrl'),
    nodeName: store.get('nodeName'),
    nodeId: store.get('nodeId'),
    token: store.get('token'),
    reportInterval: store.get('reportInterval'),
  }
})

ipcMain.handle('config:save', async (_event, config) => {
  await loadStore()
  for (const key of ['centerUrl', 'nodeName', 'token', 'reportInterval']) {
    if (Object.prototype.hasOwnProperty.call(config, key)) {
      store.set(key, config[key])
    }
  }
  if (!config.token) {
    store.set('token', '')
  }
  ws?.close()
  connectAgent().catch(error => mainWindow?.webContents.send('agent:error', error.message))
  emitStatus()
})

ipcMain.handle('agent:connect', async () => connectAgent())
ipcMain.handle('terminal:create', (_event, options) => createSession(options))
ipcMain.handle('terminal:write', (_event, payload) => writeSession(payload.sessionId, payload.data))
ipcMain.handle('terminal:resize', (_event, payload) => resizeSession(payload.sessionId, payload.cols, payload.rows))
ipcMain.handle('terminal:close', (_event, sessionId) => closeSession(sessionId))

app.whenReady().then(async () => {
  await createWindow()
  connectAgent().catch(error => mainWindow?.webContents.send('agent:error', error.message))
})

app.on('window-all-closed', () => {
  for (const sessionId of sessions.keys()) {
    closeSession(sessionId)
  }
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0) {
    createWindow()
  }
})
