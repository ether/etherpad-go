// typescript
export function createSocket(path: string): WebSocket {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}/admin/ws?namespace=${encodeURIComponent(path)}`
    return new WebSocket(url)
}

export class SocketIoWrapper {
    private socket: WebSocket
    private static readonly eventCallbacks: { [key: string]: Function[] } = {}
    private readonly namespace: string
    private queueMessages: Array<{ event: string; data?: any }> = []

    constructor(namespace: string) {
        this.namespace = namespace
        try {
            this.socket = createSocket(namespace)
        } catch (e) {
            console.error('WebSocket creation failed:', e)
            throw e
        }

        // Arrow functions keep `this` bound to the instance
        this.socket.onopen = this.handleOpen
        this.socket.onclose = this.handleClose
        this.socket.onerror = this.handleError
        this.socket.onmessage = this.handleMessage
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
                this.socket.send(JSON.stringify(payload))
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

        setTimeout(() => {
            console.log('Reconnecting...')
            try {
                const socket = createSocket(this.namespace)
                socket.onopen = this.handleOpen
                socket.onclose = this.handleClose
                socket.onerror = this.handleError
                socket.onmessage = this.handleMessage
                this.socket = socket
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
        console.log('connect')
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

    public emit(event: string, data?: any) {
        const payload = data === undefined ? { event } : { event, data }

        if (this.socket.readyState !== WebSocket.OPEN) {
            // Queue when not open (CONNECTING, CLOSING, CLOSED)
            this.queueMessages.push({ event, data })
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
        this.socket.close()
    }
}
