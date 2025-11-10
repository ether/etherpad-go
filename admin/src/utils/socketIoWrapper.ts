export function createSocket(path: string): WebSocket {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}/admin/ws?namespace=${encodeURIComponent(path)}`
    return new WebSocket(url)
}



export class SocketIoWrapper {
    private socket: WebSocket
    private static readonly eventCallbacks: { [key: string]: Function[] } = {}
    private readonly namespace: string;

    constructor(namespace: string) {
        try {
            this.socket = createSocket(namespace)
        } catch (e) {
            console.error('WebSocket creation failed:', e)

            throw e
        }
        this.namespace = namespace
        this.socket.onopen = this.onConnect
        this.socket.onclose = this.onClose
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
        console.error('onopen')
        const iID = window.setInterval(() => {
            console.log('check')
            if (SocketIoWrapper.eventCallbacks['connect']) {
                console.log('Handled connect event')
                SocketIoWrapper.eventCallbacks['connect'].forEach(callback => {
                    callback()
                })
                clearInterval(iID)
            }
        }, 200)
    }

    private onClose() {
        console.log('onclose')
        if (SocketIoWrapper.eventCallbacks['disconnect']) {
            SocketIoWrapper.eventCallbacks['disconnect'].forEach(callback => {
                callback()
            })
        }
        setTimeout(() => {
            console.log('Reconnecting...')
            const socket = createSocket(this.namespace)
            socket.onopen = this.onConnect
            socket.onclose = this.onClose
            socket.onerror = this.onError
            socket.onmessage = this.onMessage
            this.socket = socket
        }, 1000)
    }

    private onError(evt: Event) {
        console.log('onerror', evt)
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
        this.socket.send(JSON.stringify({event, data}))
    }

    public off() {
        console.log("Off")
    }

    disconnect() {
        this.socket.close()
    }
}