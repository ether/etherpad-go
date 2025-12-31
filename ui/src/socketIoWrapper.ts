export function createSocket(path = '/socket.io/'): WebSocket {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}${path}`
    return new WebSocket(url)
}

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'reconnecting' | 'failed'

export interface ConnectionState {
    status: ConnectionStatus
    reconnectAttempts: number
    lastError?: string
}

export class SocketIoWrapper {
    private _socket: WebSocket | undefined
    private static eventCallbacks: { [key: string]: Function[] } = {}

    private readonly connectionState: ConnectionState = {
        status: 'disconnected',
        reconnectAttempts: 0
    }

    private readonly maxReconnectAttempts = 5
    private readonly initialReconnectDelay = 1000
    private readonly maxReconnectDelay = 5000
    private reconnectTimeout: ReturnType<typeof setTimeout> | null = null
    private isManualDisconnect = false
    private hasConnectedOnce = false

    // Public socket property for compatibility with socket.io API
    public get socket(): WebSocket | undefined {
        return this._socket
    }

    constructor() {
        this.ensureSocket()
    }

    private setConnectionState(status: ConnectionStatus, error?: string) {
        const prevStatus = this.connectionState.status
        this.connectionState.status = status
        if (error) {
            this.connectionState.lastError = error
        }
        console.log(`Connection status changed: ${prevStatus} -> ${status}`)
    }

    public getConnectionState(): ConnectionState {
        return { ...this.connectionState }
    }

    private ensureSocket() {
        if (this._socket && this._socket.readyState === WebSocket.OPEN) return
        if (this._socket && this._socket.readyState === WebSocket.CONNECTING) return

        // Clean up old socket if exists
        if (this._socket) {
            this._socket.onopen = null
            this._socket.onclose = null
            this._socket.onerror = null
            this._socket.onmessage = null
            if (this._socket.readyState !== WebSocket.CLOSED) {
                this._socket.close()
            }
        }

        this.setConnectionState('connecting')

        try {
            this._socket = createSocket()
        } catch (e) {
            console.error('WebSocket creation failed:', e)
            this.setConnectionState('failed', String(e))
            this.scheduleReconnect()
            return
        }

        this._socket.onopen = this.onConnect.bind(this)
        this._socket.onclose = this.handleClose.bind(this)
        this._socket.onerror = this.onError.bind(this)
        this._socket.onmessage = this.onMessage.bind(this)
    }

    private onMessage(evt: MessageEvent) {
        const arr = JSON.parse(evt.data)
        console.log(`Received message: ${evt.data}`)
        if (!SocketIoWrapper.eventCallbacks[arr[0]]) return
        SocketIoWrapper.eventCallbacks[arr[0]].forEach(f => {
            f(arr[1])
        })
    }

    private onConnect() {
        console.log('WebSocket connected')
        const wasReconnect = this.hasConnectedOnce
        this.hasConnectedOnce = true
        this.connectionState.reconnectAttempts = 0
        this.setConnectionState('connected')

        // Clear any pending reconnect
        if (this.reconnectTimeout) {
            clearTimeout(this.reconnectTimeout)
            this.reconnectTimeout = null
        }

        if (wasReconnect) {
            // This is a reconnection
            this.emitEvent('reconnect')
        } else {
            // First connection
            this.emitEvent('connect')
        }
    }

    private handleClose(evt?: CloseEvent) {
        console.log('WebSocket closed', evt?.code, evt?.reason)

        // Emit disconnect event
        this.emitEvent('disconnect', evt)

        if (!this.isManualDisconnect) {
            this.setConnectionState('reconnecting')
            this.scheduleReconnect()
        } else {
            this.setConnectionState('disconnected')
        }
    }

    private onError(evt: Event) {
        console.error('WebSocket error', evt)
        this.connectionState.lastError = 'WebSocket error occurred'

        // Emit error event
        this.emitEvent('error', evt)
    }

    private scheduleReconnect() {
        if (this.isManualDisconnect) return
        if (this.reconnectTimeout) return // Already scheduled

        this.connectionState.reconnectAttempts++

        if (this.connectionState.reconnectAttempts > this.maxReconnectAttempts) {
            console.log('Max reconnect attempts reached')
            this.setConnectionState('failed', 'Max reconnect attempts reached')
            // Emit reconnect_failed event
            this.emitEvent('reconnect_failed', new Error('Max reconnect attempts reached'))
            return
        }

        // Emit reconnect_attempt event
        this.emitEvent('reconnect_attempt', this.connectionState.reconnectAttempts)

        // Exponential backoff with jitter
        const delay = Math.min(
            this.initialReconnectDelay * Math.pow(2, this.connectionState.reconnectAttempts - 1),
            this.maxReconnectDelay
        ) + Math.random() * 1000

        console.log(`Scheduling reconnect attempt ${this.connectionState.reconnectAttempts} in ${Math.round(delay)}ms`)

        this.reconnectTimeout = setTimeout(() => {
            this.reconnectTimeout = null
            console.log(`Reconnect attempt ${this.connectionState.reconnectAttempts}`)
            this.ensureSocket()
        }, delay)
    }

    private emitEvent(event: string, data?: any) {
        const callbacks = SocketIoWrapper.eventCallbacks[event]
        if (callbacks) {
            callbacks.forEach(callback => {
                try {
                    callback(data)
                } catch (e) {
                    console.error(`${event} callback error`, e)
                }
            })
        }
    }

    /**
     * Force a reconnection attempt - can be called from UI when user clicks retry
     */
    public forceReconnect() {
        console.log('Force reconnect requested')
        this.isManualDisconnect = false
        this.connectionState.reconnectAttempts = 0

        if (this.reconnectTimeout) {
            clearTimeout(this.reconnectTimeout)
            this.reconnectTimeout = null
        }

        this.ensureSocket()
    }

    /**
     * Manually disconnect the socket
     */
    public disconnect() {
        this.isManualDisconnect = true

        if (this.reconnectTimeout) {
            clearTimeout(this.reconnectTimeout)
            this.reconnectTimeout = null
        }

        if (this._socket) {
            this._socket.close()
        }

        this.setConnectionState('disconnected')
    }

    public connect() {
        this.isManualDisconnect = false
        this.ensureSocket()
    }

    public io = {
        on: (event: string, callback: Function) => {
            this.on(event, callback)
        }
    }

    public once(event: string, callback: Function) {
        const wrappedCallback = (...args: any[]) => {
            this.off(event, wrappedCallback)
            callback(...args)
        }
        this.on(event, wrappedCallback)
    }

    public on(event: string, callback: Function) {
        if (SocketIoWrapper.eventCallbacks[event]) {
            SocketIoWrapper.eventCallbacks[event].push(callback)
        } else {
            SocketIoWrapper.eventCallbacks[event] = [callback]
        }
    }

    public emit(event: string, data: any) {
        if (!this._socket || this._socket.readyState !== WebSocket.OPEN) {
            console.warn('Cannot emit, socket not connected. Current state:', this.connectionState.status)
            return
        }
        // Send as object {event, data} to match the format expected by the backend
        this._socket.send(JSON.stringify({ event, data }))
    }

    public off(event?: string, callback?: Function) {
        if (!event) {
            // Remove all listeners
            SocketIoWrapper.eventCallbacks = {}
            return
        }

        if (!callback) {
            // Remove all listeners for this event
            delete SocketIoWrapper.eventCallbacks[event]
            return
        }

        // Remove specific callback
        const callbacks = SocketIoWrapper.eventCallbacks[event]
        if (callbacks) {
            const index = callbacks.indexOf(callback)
            if (index !== -1) {
                callbacks.splice(index, 1)
            }
        }
    }

    public isConnected(): boolean {
        return this._socket?.readyState === WebSocket.OPEN
    }
}