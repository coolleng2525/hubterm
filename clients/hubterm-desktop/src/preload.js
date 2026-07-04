const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('hubterm', {
  getConfig: () => ipcRenderer.invoke('config:get'),
  saveConfig: config => ipcRenderer.invoke('config:save', config),
  connectAgent: () => ipcRenderer.invoke('agent:connect'),
  createTerminal: options => ipcRenderer.invoke('terminal:create', options),
  writeTerminal: payload => ipcRenderer.invoke('terminal:write', payload),
  resizeTerminal: payload => ipcRenderer.invoke('terminal:resize', payload),
  closeTerminal: sessionId => ipcRenderer.invoke('terminal:close', sessionId),
  onTerminalData: callback => ipcRenderer.on('terminal:data', (_event, payload) => callback(payload)),
  onTerminalClosed: callback => ipcRenderer.on('terminal:closed', (_event, payload) => callback(payload)),
  onAgentStatus: callback => ipcRenderer.on('agent:status', (_event, payload) => callback(payload)),
  onAgentError: callback => ipcRenderer.on('agent:error', (_event, payload) => callback(payload)),
})
