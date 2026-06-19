import { Injectable, Injector, NgZone } from '@angular/core'
import { ConfigService, HostAppService, Platform } from 'tabby-core'
import { TerminalDecorator } from 'tabby-terminal'
import { BaseTerminalTabComponent } from 'tabby-terminal'

interface NodeReport {
    node_id: string
    node_name: string
    hostname: string
    platform: string
    os_version: string
    arch: string
    sessions: SessionInfo[]
    capabilities: string[]
}

interface SessionInfo {
    id: string
    type: string
    name: string
    host?: string
    port?: number
}

interface CenterCommand {
    type: string
    payload?: any
}

@Injectable()
export class HubTermService {
    private ws: WebSocket | null = null
    private reportTimer: any = null
    private reconnectTimer: any = null
    private reconnectDelay = 1000
    private nodeId: string = ''
    private attachedTabs: Map<BaseTerminalTabComponent, boolean> = new Map()
    private stopping = false

    private get config() { return this.injector.get(ConfigService) }
    private get hostApp() { return this.injector.get(HostAppService) }

    constructor(
        private injector: Injector,
        private zone: NgZone,
    ) {
        this.loadNodeId()
    }

    private loadNodeId() {
        try {
            const saved = localStorage.getItem('hubterm_node_id')
            if (saved) {
                this.nodeId = saved
            } else {
                this.nodeId = this.generateId()
                localStorage.setItem('hubterm_node_id', this.nodeId)
            }
        } catch {
            this.nodeId = this.generateId()
        }
    }

    private generateId(): string {
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
            const r = Math.random() * 16 | 0
            return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16)
        })
    }

    start() {
        const cfg = this.config.store.hubterm
        if (!cfg || !cfg.enabled || !cfg.centerUrl) return

        this.stopping = false
        this.connect(cfg.centerUrl)
    }

    stop() {
        this.stopping = true
        if (this.reportTimer) clearInterval(this.reportTimer)
        if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
        if (this.ws) {
            this.ws.onclose = null
            this.ws.close()
            this.ws = null
        }
    }

    attachTab(tab: BaseTerminalTabComponent) {
        this.attachedTabs.set(tab, true)
    }

    detachTab(tab: BaseTerminalTabComponent) {
        this.attachedTabs.delete(tab)
    }

    sendTerminalData(tab: BaseTerminalTabComponent, data: string) {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return

        // Find session info from tab
        let sessionInfo: SessionInfo | null = null
        for (const [t, _] of this.attachedTabs) {
            if (t === tab) {
                sessionInfo = {
                    id: (tab as any).sessionId || 'unknown',
                    type: (tab as any).sessionType || 'local',
                    name: (tab as any).sessionName || 'terminal',
                }
                break
            }
        }

        this.ws.send(JSON.stringify({
            type: 'terminal_data',
            node_id: this.nodeId,
            session: sessionInfo,
            data: btoa(data),
        }))
    }

    writeToTab(tab: BaseTerminalTabComponent, data: string) {
        if (tab && (tab as any).write) {
            (tab as any).write(atob(data))
        }
    }

    private connect(url: string) {
        if (this.ws) return

        try {
            this.ws = new WebSocket(url)

            this.ws.onopen = () => {
                this.reconnectDelay = 1000
                this.register()
                this.startReporting()
            }

            this.ws.onclose = () => {
                this.ws = null
                if (this.reportTimer) clearInterval(this.reportTimer)
                if (!this.stopping) this.scheduleReconnect(url)
            }

            this.ws.onerror = () => {
                // onclose will fire after this
            }

            this.ws.onmessage = (event) => {
                this.zone.run(() => this.handleCommand(event.data))
            }
        } catch (e) {
            console.error('[HubTerm] connection failed:', e)
            if (!this.stopping) this.scheduleReconnect(url)
        }
    }

    private scheduleReconnect(url: string) {
        if (this.reconnectTimer) return
        this.reconnectTimer = setTimeout(() => {
            this.reconnectTimer = null
            this.connect(url)
        }, this.reconnectDelay)
        this.reconnectDelay = Math.min(this.reconnectDelay * 2, 30000)
    }

    private register() {
        if (!this.ws) return
        const cfg = this.config.store.hubterm
        this.ws.send(JSON.stringify({
            type: 'register',
            node_id: this.nodeId,
            node_name: cfg.nodeName || this.hostApp.platform,
            token: cfg.token || '',
            domain: cfg.domain || '',
        }))
    }

    private startReporting() {
        if (this.reportTimer) clearInterval(this.reportTimer)
        const interval = (this.config.store.hubterm.reportInterval || 3) * 1000
        this.reportTimer = setInterval(() => this.sendReport(), interval)
        this.sendReport()
    }

    private sendReport() {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return

        const report: NodeReport = {
            node_id: this.nodeId,
            node_name: this.config.store.hubterm.nodeName || '',
            hostname: window.location.hostname || '',
            platform: this.hostApp.platform,
            os_version: navigator.userAgent || '',
            arch: '',
            sessions: this.collectSessions(),
            capabilities: ['tabby-terminal', 'serial', 'ssh'],
        }

        this.ws.send(JSON.stringify({
            type: 'report',
            ...report,
        }))
    }

    private collectSessions(): SessionInfo[] {
        const sessions: SessionInfo[] = []
        for (const [tab, _] of this.attachedTabs) {
            sessions.push({
                id: (tab as any).sessionId || 'unknown',
                type: (tab as any).sessionType || 'local',
                name: (tab as any).sessionName || 'terminal',
            })
        }
        return sessions
    }

    private handleCommand(raw: string) {
        try {
            const cmd: CenterCommand = JSON.parse(raw)
            console.log('[HubTerm] command:', cmd.type)

            switch (cmd.type) {
                case 'ping':
                    this.ws?.send(JSON.stringify({ type: 'pong' }))
                    break

                case 'write':
                    // Write data to a specific terminal tab
                    if (cmd.payload?.session_id && cmd.payload?.data) {
                        for (const [tab, _] of this.attachedTabs) {
                            if ((tab as any).sessionId === cmd.payload.session_id) {
                                this.writeToTab(tab, cmd.payload.data)
                                break
                            }
                        }
                    }
                    break

                case 'disconnect':
                    if (cmd.payload?.session_id) {
                        for (const [tab, _] of this.attachedTabs) {
                            if ((tab as any).sessionId === cmd.payload.session_id) {
                                (tab as any).close()
                                break
                            }
                        }
                    }
                    break

                case 'set_permission':
                    console.log('[HubTerm] permission update:', cmd.payload)
                    break

                case 'update_config':
                    if (cmd.payload) {
                        Object.assign(this.config.store.hubterm, cmd.payload)
                        this.config.save()
                    }
                    break

                case 'restart':
                    this.stop()
                    setTimeout(() => this.start(), 1000)
                    break

                default:
                    console.log('[HubTerm] unknown command:', cmd.type)
            }
        } catch (e) {
            console.error('[HubTerm] failed to handle command:', e)
        }
    }
}
