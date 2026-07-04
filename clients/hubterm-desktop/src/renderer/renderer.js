const state = {
  config: null,
  connected: false,
  activeSessionId: null,
  sessions: new Map(),
  buffers: new Map(),
}

const elements = {
  nodeId: document.getElementById('node-id'),
  connectionPill: document.getElementById('connection-pill'),
  configForm: document.getElementById('config-form'),
  centerUrl: document.getElementById('center-url'),
  nodeName: document.getElementById('node-name'),
  token: document.getElementById('token'),
  reportInterval: document.getElementById('report-interval'),
  connectAgent: document.getElementById('connect-agent'),
  newTerminal: document.getElementById('new-terminal'),
  closeTerminal: document.getElementById('close-terminal'),
  sessionCount: document.getElementById('session-count'),
  sessionList: document.getElementById('session-list'),
  terminalTitle: document.getElementById('terminal-title'),
  terminalSubtitle: document.getElementById('terminal-subtitle'),
  terminalOutput: document.getElementById('terminal-output'),
  terminalInputForm: document.getElementById('terminal-input-form'),
  terminalInput: document.getElementById('terminal-input'),
  sendInput: document.getElementById('send-input'),
}

function appendBuffer (sessionId, data) {
  const next = `${state.buffers.get(sessionId) || ''}${data}`
  state.buffers.set(sessionId, next.slice(-80000))
  if (sessionId === state.activeSessionId) {
    renderTerminal()
  }
}

function renderConfig () {
  if (!state.config) {
    return
  }
  elements.centerUrl.value = state.config.centerUrl || ''
  elements.nodeName.value = state.config.nodeName || ''
  elements.token.value = state.config.token || ''
  elements.reportInterval.value = state.config.reportInterval || 3
  elements.nodeId.textContent = state.config.nodeId ? `Node ${state.config.nodeId}` : 'Node pending'
}

function renderStatus () {
  elements.connectionPill.textContent = state.connected ? 'Online' : 'Offline'
  elements.connectionPill.classList.toggle('online', state.connected)
}

function renderSessions () {
  elements.sessionCount.textContent = String(state.sessions.size)
  elements.sessionList.replaceChildren()

  for (const session of state.sessions.values()) {
    const button = document.createElement('button')
    button.type = 'button'
    button.className = `session-button${session.sessionId === state.activeSessionId ? ' active' : ''}`
    button.innerHTML = `<span>${session.name}</span><span>${session.shell}</span>`
    button.addEventListener('click', () => {
      state.activeSessionId = session.sessionId
      renderSessions()
      renderTerminal()
    })
    elements.sessionList.appendChild(button)
  }
}

function renderTerminal () {
  const session = state.sessions.get(state.activeSessionId)
  const hasSession = Boolean(session)
  elements.closeTerminal.disabled = !hasSession
  elements.terminalInput.disabled = !hasSession
  elements.sendInput.disabled = !hasSession

  if (!session) {
    elements.terminalTitle.textContent = 'No terminal'
    elements.terminalSubtitle.textContent = 'Create a local shell to start streaming through HubTerm.'
    elements.terminalOutput.textContent = ''
    return
  }

  elements.terminalTitle.textContent = session.name
  elements.terminalSubtitle.textContent = `${session.shell} · ${session.sessionId}`
  elements.terminalOutput.textContent = state.buffers.get(session.sessionId) || ''
  elements.terminalOutput.scrollTop = elements.terminalOutput.scrollHeight
}

async function loadConfig () {
  state.config = await window.hubterm.getConfig()
  renderConfig()
}

elements.configForm.addEventListener('submit', async event => {
  event.preventDefault()
  await window.hubterm.saveConfig({
    centerUrl: elements.centerUrl.value.trim(),
    nodeName: elements.nodeName.value.trim(),
    token: elements.token.value.trim(),
    reportInterval: Number(elements.reportInterval.value) || 3,
  })
  await loadConfig()
})

elements.connectAgent.addEventListener('click', async () => {
  await window.hubterm.connectAgent()
})

elements.newTerminal.addEventListener('click', async () => {
  const session = await window.hubterm.createTerminal({ cols: 120, rows: 34 })
  state.sessions.set(session.sessionId, session)
  state.buffers.set(session.sessionId, '')
  state.activeSessionId = session.sessionId
  renderSessions()
  renderTerminal()
  elements.terminalInput.focus()
})

elements.closeTerminal.addEventListener('click', async () => {
  if (!state.activeSessionId) {
    return
  }
  await window.hubterm.closeTerminal(state.activeSessionId)
})

elements.terminalInputForm.addEventListener('submit', async event => {
  event.preventDefault()
  const command = elements.terminalInput.value
  if (!state.activeSessionId || command.length === 0) {
    return
  }
  await window.hubterm.writeTerminal({ sessionId: state.activeSessionId, data: `${command}\r` })
  elements.terminalInput.value = ''
})

window.addEventListener('resize', () => {
  if (state.activeSessionId) {
    window.hubterm.resizeTerminal({ sessionId: state.activeSessionId, cols: 120, rows: 34 })
  }
})

window.hubterm.onAgentStatus(status => {
  state.connected = Boolean(status.connected)
  state.config = { ...state.config, ...status }
  renderStatus()
  renderConfig()
})

window.hubterm.onAgentError(message => {
  if (state.activeSessionId) {
    appendBuffer(state.activeSessionId, `\r\n[hubterm] ${message}\r\n`)
  }
})

window.hubterm.onTerminalData(({ sessionId, data }) => {
  appendBuffer(sessionId, data)
})

window.hubterm.onTerminalClosed(({ sessionId }) => {
  state.sessions.delete(sessionId)
  state.buffers.delete(sessionId)
  if (state.activeSessionId === sessionId) {
    state.activeSessionId = state.sessions.keys().next().value || null
  }
  renderSessions()
  renderTerminal()
})

loadConfig()
  .then(() => {
    renderStatus()
    renderSessions()
    renderTerminal()
  })
  .catch(error => {
    elements.terminalOutput.textContent = `[hubterm] failed to load config: ${error.message}`
  })
