export function createSocket(path = '/socket.io/'): WebSocket {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}${path}`
    return new WebSocket(url)
}



export class SocketIoWrapper {
    private socket: WebSocket | undefined
    private static readonly eventCallbacks: { [key: string]: Function[] } = {}

    constructor() {
        this.ensureSocket()
    }

    private ensureSocket() {
        if (this.socket && this.socket.readyState !== WebSocket.CLOSED) return
        try {
            this.socket = createSocket()
        } catch (e) {
            console.error('WebSocket creation failed:', e)

            throw e
        }
        this.socket.onopen = this.onConnect
        this.socket.onclose = this.handleClose
        this.socket.onerror  = this.onError
        this.socket.onmessage = this.onMessage

    }

    private onMessage(evt: MessageEvent) {
        const arr = JSON.parse(evt.data)
        SocketIoWrapper.eventCallbacks[arr[0]].forEach(f=>{
            f(arr[1])
        })
    }

    private onConnect() {
        const iID = window.setInterval(() => {
            console.log('check')
            if (SocketIoWrapper.eventCallbacks['connect'] && SocketIoWrapper.eventCallbacks['connect'].length == 1) {
                console.log('Handled connect event')
                SocketIoWrapper.eventCallbacks['connect'].forEach(callback => {
                    callback()
                })
                clearInterval(iID)
            }
        }, 200)
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

    private onError(evt: Event) {
        if (SocketIoWrapper.eventCallbacks['disconnect']) {
            SocketIoWrapper.eventCallbacks['disconnect'].forEach(callback => {
                callback(evt)
            })
        }

    }


    public connect() {
        console.log('connect')
    }

    public io = {
        on: (event: string, callback: Function)=>{
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

    public on(event: string, callback: Function) {
        if (SocketIoWrapper.eventCallbacks[event]) {
            SocketIoWrapper.eventCallbacks[event].push(callback)
        } else {
            SocketIoWrapper.eventCallbacks[event] = [callback]
        }
    }

    public emit(event: string, data: any) {
        this.ensureSocket()
        this.socket?.send(JSON.stringify({event, data}))
    }

    public off() {
        console.log("Off")
    }
}