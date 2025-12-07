// typescript
export async function createSocket(token: string): Promise<WebSocket> {
    const resp = await fetch(`/admin/validate?token=${token}`)

    if (resp.status === 401) {
        sessionStorage.removeItem('token')
        sessionStorage.removeItem('refresh_token')
        globalThis.location.reload()
        throw new Error('Unauthorized')
    }

    // korrektes Protokoll bestimmen (nicht abhängig vom Token)
    const protocol = globalThis.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${globalThis.location.host}/admin/ws?token=${token}`
    return new WebSocket(url)
}

export class SocketIoWrapper {
    private socket: WebSocket | undefined
    private static readonly eventCallbacks: { [key: string]: Function[] } = {}
    private queueMessages: Array<{ event: string; data?: any }> = []
    private readonly token: string
    private isConnecting = false

    constructor(token: string) {
        this.token = token
        // startet asynchron, aber ensureSocket schützt vor parallelen Versuchen
        void this.ensureSocket()
    }

    private async ensureSocket(): Promise<void> {
        // Wenn bereits OPEN oder CONNECTING -> nichts tun
        if (this.socket && (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING)) {
            return
        }
        if (this.isConnecting) return
        this.isConnecting = true

        try {
            const socket = await createSocket(this.token)
            socket.onopen = this.handleOpen
            socket.onclose = this.handleClose
            socket.onerror = this.handleError
            socket.onmessage = this.handleMessage
            this.socket = socket
        } catch (e) {
            console.error('WebSocket creation failed:', e)
        } finally {
            this.isConnecting = false
        }
    }

    private handleMessage = (evt: MessageEvent) => {
        try {
            const arr = JSON.parse(evt.data)
            const callbacks = SocketIoWrapper.eventCallbacks[arr[0]]
            if (callbacks && callbacks.length) {
                callbacks.forEach(f => {
                    try { f(arr[1]) } catch (e) { console.error('callback error', e) }
                })
            }
        } catch (e) {
            console.error('Failed to parse message', e)
        }
    }

    private handleOpen = () => {
        while (this.queueMessages.length > 0) {
            const msg = this.queueMessages.shift()!
            const payload = msg.data === undefined ? { event: msg.event } : { event: msg.event, data: msg.data }
            try {
                this.socket?.send(JSON.stringify(payload))
            } catch (e) {
                console.error('Failed to send queued message, re-queueing', e)
                this.queueMessages.unshift(msg)
                break
            }
        }

        const connectCallbacks = SocketIoWrapper.eventCallbacks['connect']
        if (connectCallbacks && connectCallbacks.length) {
            connectCallbacks.forEach(cb => {
                try { cb() } catch (e) { console.error('connect callback error', e) }
            })
        }
    }

    private handleClose = (evt?: CloseEvent) => {
        const disconnectCallbacks = SocketIoWrapper.eventCallbacks['disconnect']
        if (disconnectCallbacks && disconnectCallbacks.length) {
            disconnectCallbacks.forEach(cb => {
                try { cb(evt) } catch (e) { console.error('disconnect callback error', e) }
            })
        }

        // Reconnect über ensureSocket (verhindert parallele createSocket-Aufrufe)
        setTimeout(async () => {
            console.log('Reconnecting...')
            try {
                await this.ensureSocket()
            } catch (e) {
                console.error('Reconnect failed', e)
            }
        }, 1000)
    }

    private handleError = (evt: Event) => {
        console.log('onerror', evt)
        const disconnectCallbacks = SocketIoWrapper.eventCallbacks['disconnect']
        if (disconnectCallbacks && disconnectCallbacks.length) {
            disconnectCallbacks.forEach(callback => {
                try { callback(evt) } catch (e) { console.error('error callback error', e) }
            })
        }
    }

    public connect() {
        void this.ensureSocket()
    }

    public io = {
        on: (event: string, callback: (data: any)=>void) => {
            this.on(event, callback)
        }
    }

    public once(event: string, callback: Function) {
        if (SocketIoWrapper.eventCallbacks[event]) {
            SocketIoWrapper.eventCallbacks[event].push(callback)
        } else {
            SocketIoWrapper.eventCallbacks[event] = [callback]
        }
    }

    public on(event: string, callback: (data: any)=>void) {
        if (SocketIoWrapper.eventCallbacks[event]) {
            SocketIoWrapper.eventCallbacks[event].push(callback)
        } else {
            SocketIoWrapper.eventCallbacks[event] = [callback]
        }
    }

    public async emit(event: string, data?: any) {
        const payload = data === undefined ? { event } : { event, data }
        if (this.socket?.readyState !== WebSocket.OPEN) {
            this.queueMessages.push({ event, data })
            // stelle sicher dass eine Verbindung aufgebaut wird
            void this.ensureSocket()
            return
        }

        try {
            this.socket.send(JSON.stringify(payload))
        } catch (e) {
            console.error('Send failed, queueing message', e)
            this.queueMessages.push({ event, data })
        }
    }

    public off() {
        console.log('Off')
    }

    disconnect() {
        this.socket?.close()
        this.socket = undefined
    }
}
