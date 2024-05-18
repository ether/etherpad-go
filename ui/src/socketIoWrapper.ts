export class SocketIoWrapper {
    private socket: WebSocket
    private eventCallbacks: { [key: string]: Function[] } = {}

    constructor() {
        this.socket = new WebSocket('ws://localhost:3000/socket.io/')
        this.socket.onopen = () => {
            console.log('onopen')
            const iID = window.setInterval(() => {
                console.log('check')
                if (this.eventCallbacks['connect'] && this.eventCallbacks['connect'].length == 1) {
                    console.log('Handled connect event')
                    this.eventCallbacks['connect'].forEach(callback => {
                        callback()
                    })
                    clearInterval(iID)
                }
            }, 200)
        }
    }


    public connect() {
        console.log('connect')
    }

    public once(event: string, callback: Function) {
        if (this.eventCallbacks[event]) {
            this.eventCallbacks[event].push(callback)
        } else {
            this.eventCallbacks[event] = [callback]
        }
    }

    public on(event: string, callback: Function) {
        if (this.eventCallbacks[event]) {
            this.eventCallbacks[event].push(callback)
        } else {
            this.eventCallbacks[event] = [callback]
        }
    }

    public emit(event: string, data: any) {
        this.socket.send(JSON.stringify({event, data}))
    }
}